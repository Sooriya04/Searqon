package db

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"src/models"
)

// GetSearchCache retrieves cached search results.
func GetSearchCache(query string) ([]models.SearchResult, string, bool) {
	dbMu.RLock()
	enabled := dbEnabled
	pool := dbPool
	dbMu.RUnlock()

	if !enabled || pool == nil {
		return nil, "", false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var resultsJSON []byte
	var provider string
	var createdAt time.Time

	err := pool.QueryRow(ctx, `
		SELECT results, provider, created_at
		FROM search_cache
		WHERE query = $1
	`, query).Scan(&resultsJSON, &provider, &createdAt)

	if err != nil {
		return nil, "", false
	}

	if time.Since(createdAt) > searchCacheTTL {
		return nil, "", false
	}

	var results []models.SearchResult
	if err := json.Unmarshal(resultsJSON, &results); err != nil {
		return nil, "", false
	}

	return results, provider, true
}

// SaveSearchCache caches search results in the database.
func SaveSearchCache(query string, results []models.SearchResult, provider string) {
	dbMu.RLock()
	enabled := dbEnabled
	pool := dbPool
	dbMu.RUnlock()

	if !enabled || pool == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resultsJSON, err := json.Marshal(results)
	if err != nil {
		log.Printf("[Database] Failed to marshal search cache: %v", err)
		return
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO search_cache (query, results, provider, created_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		ON CONFLICT (query) DO UPDATE
		SET results = EXCLUDED.results,
		    provider = EXCLUDED.provider,
		    created_at = CURRENT_TIMESTAMP
	`, query, resultsJSON, provider)

	if err != nil {
		log.Printf("[Database] Failed to save search cache: %v", err)
	}
}
