package db

import (
	"context"
	"log"
	"time"

	"src/models"
)

// InitFullTextSearch configures the generated tsvector column and GIN index for full-text search.
func InitFullTextSearch() {
	dbMu.RLock()
	enabled := dbEnabled
	pool := dbPool
	dbMu.RUnlock()

	if !enabled || pool == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	schema := `
	-- Add tsvector generated column
	ALTER TABLE scrape_cache ADD COLUMN IF NOT EXISTS tsv tsvector GENERATED ALWAYS AS (
		to_tsvector('english', coalesce(title, '') || ' ' || coalesce(description, '') || ' ' || coalesce(content, ''))
	) STORED;

	-- Create GIN index for fast text searches
	CREATE INDEX IF NOT EXISTS idx_scrape_cache_tsv ON scrape_cache USING gin(tsv);
	`

	_, err := pool.Exec(ctx, schema)
	if err != nil {
		log.Printf("[Database] Failed to configure full-text search: %v", err)
	} else {
		log.Println("[Database] Full-text search index (tsvector) verified successfully.")
	}
}

// SearchLocalIndex performs a full-text search over the scraped corpus in the database.
func SearchLocalIndex(query string, limit int) ([]models.SearchResult, error) {
	dbMu.RLock()
	enabled := dbEnabled
	pool := dbPool
	dbMu.RUnlock()

	if !enabled || pool == nil {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := pool.Query(ctx, `
		SELECT url, title, description, content, ts_rank_cd(tsv, plainto_tsquery('english', $1)) AS rank
		FROM scrape_cache
		WHERE tsv @@ plainto_tsquery('english', $1)
		ORDER BY rank DESC
		LIMIT $2
	`, query, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.SearchResult
	for rows.Next() {
		var r models.SearchResult
		var desc string
		var rank float64

		err := rows.Scan(&r.URL, &r.Title, &desc, &r.Content, &rank)
		if err != nil {
			return nil, err
		}

		r.Snippet = desc
		r.Scraped = true
		r.Source = "local_index"
		r.Score = rank
		results = append(results, r)
	}

	return results, nil
}
