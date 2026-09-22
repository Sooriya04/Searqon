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
	// 1. Save to in-memory cache
	searchMemoryCache.Store(query, inMemorySearchEntry{
		results:   results,
		provider:  provider,
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

	// 3. Save to SQLite
	if backend == "sqlite" && sDB != nil {
		_, err := sDB.Exec(`
			INSERT INTO search_cache (query, results, provider, created_at)
			VALUES (?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(query) DO UPDATE SET
				results = excluded.results,
				provider = excluded.provider,
				created_at = CURRENT_TIMESTAMP
		`, query, string(resultsJSON), provider)

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
			INSERT INTO search_cache (query, results, provider, created_at)
			VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
			ON CONFLICT (query) DO UPDATE
			SET results = EXCLUDED.results,
			    provider = EXCLUDED.provider,
			    created_at = CURRENT_TIMESTAMP
		`, query, resultsJSON, provider)

		if err != nil {
			log.Printf("[Database] PostgreSQL failed to save search cache: %v", err)
		}
	}
}
