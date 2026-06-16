package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

// ─── Search Cache Helpers ───────────────────────────────────────────────────

func getSearchCache(query string) ([]SearchResult, string, bool) {
	dbMu.RLock()
	enabled := dbEnabled
	pool := dbPool
	ttl := searchCacheTTL
	dbMu.RUnlock()

	if !enabled || pool == nil {
		return nil, "", false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var resultsJSON []byte
	var provider string

	// Look up in db - only return if created within TTL
	minCreatedAt := time.Now().Add(-ttl)
	err := pool.QueryRow(ctx, "SELECT results, provider FROM search_cache WHERE query = $1 AND created_at > $2 LIMIT 1", query, minCreatedAt).Scan(&resultsJSON, &provider)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			log.Printf("[Database] Error querying search_cache for %q: %v", query, err)
		}
		return nil, "", false
	}

	var results []SearchResult
	if err := json.Unmarshal(resultsJSON, &results); err != nil {
		log.Printf("[Database] Error parsing cached search JSON for %q: %v", query, err)
		return nil, "", false
	}

	return results, provider, true
}

func saveSearchCache(query string, results []SearchResult, provider string) {
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
		log.Printf("[Database] Error marshaling results for cache query %q: %v", query, err)
		return
	}

	// Upsert query results
	_, err = pool.Exec(ctx, `
		INSERT INTO search_cache (query, results, provider, created_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		ON CONFLICT (query) DO UPDATE
		SET results = EXCLUDED.results, provider = EXCLUDED.provider, created_at = CURRENT_TIMESTAMP
	`, query, resultsJSON, provider)

	if err != nil {
		log.Printf("[Database] Error saving query cache for %q: %v", query, err)
	}
}
