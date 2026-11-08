package db

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"src/models"
)

// GetSearchCache retrieves cached search results checking Redis, SQLite, PostgreSQL, or memory.
func GetSearchCache(query string) ([]models.SearchResult, string, bool) {
	// 1. Check Redis L1 cache first
	if results, provider, found := RedisGetSearchCache(query); found {
		return results, provider, true
	}

	dbMu.RLock()
	backend := dbBackend
	sDB := sqliteDB
	pPool := pgPool
	dbMu.RUnlock()

	// 2. Check SQLite persistent cache
	if backend == "sqlite" && sDB != nil {
		var resultsJSON string
		var provider string
		var createdAt time.Time

		err := sDB.QueryRow(`
			SELECT results, provider, created_at
			FROM search_cache
			WHERE query = ?
		`, query).Scan(&resultsJSON, &provider, &createdAt)

		if err == nil {
			if time.Since(createdAt) <= searchCacheTTL {
				var results []models.SearchResult
				if err := json.Unmarshal([]byte(resultsJSON), &results); err == nil {
					// Populate Redis L1 cache
					RedisSaveSearchCache(query, results, provider, searchCacheTTL-time.Since(createdAt))
					return results, provider, true
				}
			}
		}
		return nil, "", false
	}

	// 3. Check PostgreSQL persistent cache
	if backend == "postgres" && pPool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var resultsJSON []byte
		var provider string
		var createdAt time.Time

		err := pPool.QueryRow(ctx, `
			SELECT results, provider, created_at
			FROM search_cache
			WHERE query = $1
		`, query).Scan(&resultsJSON, &provider, &createdAt)

		if err == nil {
			if time.Since(createdAt) <= searchCacheTTL {
				var results []models.SearchResult
				if err := json.Unmarshal(resultsJSON, &results); err == nil {
					RedisSaveSearchCache(query, results, provider, searchCacheTTL-time.Since(createdAt))
					return results, provider, true
				}
			}
		}
		return nil, "", false
	}

	// 4. In-memory cache fallback
	if val, found := searchMemoryCache.Load(query); found {
		if entry, ok := val.(inMemorySearchEntry); ok && time.Since(entry.createdAt) <= searchCacheTTL {
			return entry.results, entry.provider, true
		}
	}

	return nil, "", false
}

// SaveSearchCache caches search results into Redis, SQLite, PostgreSQL, and memory.
func SaveSearchCache(query string, results []models.SearchResult, provider string) {
	SaveSearchCacheWithEmbedding(query, results, provider, nil)
}

// SaveSearchCacheWithEmbedding caches search results along with an optional vector embedding.
func SaveSearchCacheWithEmbedding(query string, results []models.SearchResult, provider string, embedding []float32) {
	// 1. Save to in-memory cache
	searchMemoryCache.Store(query, inMemorySearchEntry{
		results:   results,
		provider:  provider,
		embedding: embedding,
		createdAt: time.Now().UTC(),
	})

	// 2. Save to Redis L1 cache
	RedisSaveSearchCache(query, results, provider, searchCacheTTL)

	dbMu.RLock()
	backend := dbBackend
	sDB := sqliteDB
	pPool := pgPool
	dbMu.RUnlock()

	resultsJSON, err := json.Marshal(results)
	if err != nil {
		log.Printf("[Database] Failed to marshal search cache: %v", err)
		return
	}

	var embJSON *string
	if len(embedding) > 0 {
		if eb, err := json.Marshal(embedding); err == nil {
			s := string(eb)
			embJSON = &s
		}
	}

	// 3. Save to SQLite
	if backend == "sqlite" && sDB != nil {
		_, err := sDB.Exec(`
			INSERT INTO search_cache (query, results, provider, embedding, created_at)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(query) DO UPDATE SET
				results = excluded.results,
				provider = excluded.provider,
				embedding = COALESCE(excluded.embedding, search_cache.embedding),
				created_at = CURRENT_TIMESTAMP
		`, query, string(resultsJSON), provider, embJSON)

		if err != nil {
			log.Printf("[Database] SQLite failed to save search cache: %v", err)
		}
		return
	}

	// 4. Save to PostgreSQL
	if backend == "postgres" && pPool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, err := pPool.Exec(ctx, `
			INSERT INTO search_cache (query, results, provider, embedding, created_at)
			VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
			ON CONFLICT (query) DO UPDATE
			SET results = EXCLUDED.results,
			    provider = EXCLUDED.provider,
			    embedding = COALESCE(EXCLUDED.embedding, search_cache.embedding),
			    created_at = CURRENT_TIMESTAMP
		`, query, resultsJSON, provider, embJSON)

		if err != nil {
			log.Printf("[Database] PostgreSQL failed to save search cache: %v", err)
		}
	}
}

// GetSemanticSearchCache looks for cached search results whose query embedding has cosine similarity >= threshold.
func GetSemanticSearchCache(query string, queryEmbedding []float32, threshold float64) ([]models.SearchResult, string, float64, bool) {
	if len(queryEmbedding) == 0 {
		return nil, "", 0, false
	}
	if threshold <= 0 {
		threshold = 0.92
	}

	var bestResults []models.SearchResult
	var bestProvider string
	var bestSim float64

	// 1. Check in-memory search cache
	searchMemoryCache.Range(func(key, val interface{}) bool {
		entry, ok := val.(inMemorySearchEntry)
		if !ok || time.Since(entry.createdAt) > searchCacheTTL {
			return true
		}
		if len(entry.embedding) > 0 {
			sim := models.CosineSimilarity(queryEmbedding, entry.embedding)
			if sim > bestSim && sim >= threshold {
				bestSim = sim
				bestResults = entry.results
				bestProvider = entry.provider
			}
		}
		return true
	})

	if bestSim >= threshold && len(bestResults) > 0 {
		return bestResults, bestProvider, bestSim, true
	}

	dbMu.RLock()
	backend := dbBackend
	sDB := sqliteDB
	pPool := pgPool
	dbMu.RUnlock()

	// 2. Check SQLite persistent cache
	if backend == "sqlite" && sDB != nil {
		rows, err := sDB.Query(`
			SELECT results, provider, embedding, created_at
			FROM search_cache
			WHERE embedding IS NOT NULL AND embedding != ''
		`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var resultsJSON, provider, embJSON string
				var createdAt time.Time
				if err := rows.Scan(&resultsJSON, &provider, &embJSON, &createdAt); err == nil {
					if time.Since(createdAt) > searchCacheTTL {
						continue
					}
					var emb []float32
					if err := json.Unmarshal([]byte(embJSON), &emb); err == nil && len(emb) > 0 {
						sim := models.CosineSimilarity(queryEmbedding, emb)
						if sim > bestSim && sim >= threshold {
							var res []models.SearchResult
							if err := json.Unmarshal([]byte(resultsJSON), &res); err == nil {
								bestSim = sim
								bestResults = res
								bestProvider = provider
							}
						}
					}
				}
			}
		}
		if bestSim >= threshold && len(bestResults) > 0 {
			return bestResults, bestProvider, bestSim, true
		}
	}

	// 3. Check PostgreSQL persistent cache
	if backend == "postgres" && pPool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		rows, err := pPool.Query(ctx, `
			SELECT results, provider, embedding, created_at
			FROM search_cache
			WHERE embedding IS NOT NULL AND embedding != ''
		`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var resultsJSON []byte
				var provider string
				var embJSON *string
				var createdAt time.Time
				if err := rows.Scan(&resultsJSON, &provider, &embJSON, &createdAt); err == nil && embJSON != nil {
					if time.Since(createdAt) > searchCacheTTL {
						continue
					}
					var emb []float32
					if err := json.Unmarshal([]byte(*embJSON), &emb); err == nil && len(emb) > 0 {
						sim := models.CosineSimilarity(queryEmbedding, emb)
						if sim > bestSim && sim >= threshold {
							var res []models.SearchResult
							if err := json.Unmarshal(resultsJSON, &res); err == nil {
								bestSim = sim
								bestResults = res
								bestProvider = provider
							}
						}
					}
				}
			}
		}
		if bestSim >= threshold && len(bestResults) > 0 {
			return bestResults, bestProvider, bestSim, true
		}
	}

	return nil, "", 0, false
}
