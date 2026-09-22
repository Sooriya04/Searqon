package db

import (
	"context"
	"log"
	"strings"
	"time"

	"src/models"
)

// InitFullTextSearch configures full-text search indexes for PostgreSQL (tsvector) or SQLite (FTS5).
func InitFullTextSearch() {
	dbMu.RLock()
	enabled := dbEnabled
	backend := dbBackend
	sDB := sqliteDB
	pPool := pgPool
	dbMu.RUnlock()

	if !enabled {
		return
	}

	if backend == "sqlite" && sDB != nil {
		schema := `
		CREATE VIRTUAL TABLE IF NOT EXISTS scrape_cache_fts USING fts5(
			url UNINDEXED,
			title,
			description,
			content
		);
		`
		_, err := sDB.Exec(schema)
		if err != nil {
			log.Printf("[Database] SQLite FTS5 table initialization notice: %v", err)
		} else {
			log.Println("[Database] SQLite full-text search (FTS5) index verified successfully.")
		}
		return
	}

	if backend == "postgres" && pPool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		schema := `
		ALTER TABLE scrape_cache ADD COLUMN IF NOT EXISTS tsv tsvector GENERATED ALWAYS AS (
			to_tsvector('english', coalesce(title, '') || ' ' || coalesce(description, '') || ' ' || coalesce(content, ''))
		) STORED;

		CREATE INDEX IF NOT EXISTS idx_scrape_cache_tsv ON scrape_cache USING gin(tsv);
		`

		_, err := pPool.Exec(ctx, schema)
		if err != nil {
			log.Printf("[Database] Failed to configure PostgreSQL full-text search: %v", err)
		} else {
			log.Println("[Database] PostgreSQL full-text search index (tsvector) verified successfully.")
		}
	}
}

// IndexScrapeInFTS indexes scraped content into SQLite FTS5 index.
func IndexScrapeInFTS(url, title, desc, content string) {
	dbMu.RLock()
	backend := dbBackend
	sDB := sqliteDB
	dbMu.RUnlock()

	if backend != "sqlite" || sDB == nil {
		return
	}

	_, _ = sDB.Exec(`
		INSERT INTO scrape_cache_fts (url, title, description, content)
		VALUES (?, ?, ?, ?)
	`, url, title, desc, content)
}

// SearchLocalIndex performs a full-text search over the scraped corpus in SQLite or PostgreSQL.
func SearchLocalIndex(query string, limit int) ([]models.SearchResult, error) {
	dbMu.RLock()
	enabled := dbEnabled
	backend := dbBackend
	sDB := sqliteDB
	pPool := pgPool
	dbMu.RUnlock()

	if !enabled {
		return nil, nil
	}

	if limit <= 0 {
		limit = 10
	}

	// 1. SQLite FTS5 Search
	if backend == "sqlite" && sDB != nil {
		// Clean query for FTS5
		cleanQuery := strings.ReplaceAll(query, "\"", "")
		cleanQuery = strings.ReplaceAll(cleanQuery, "'", "")

		rows, err := sDB.Query(`
			SELECT url, title, description, content, -1.0 * bm25(scrape_cache_fts) AS rank
			FROM scrape_cache_fts
			WHERE scrape_cache_fts MATCH ?
			ORDER BY rank DESC
			LIMIT ?
		`, cleanQuery, limit)

		if err != nil {
			// Fallback to LIKE if FTS query syntax error
			likeTerm := "%" + query + "%"
			rows, err = sDB.Query(`
				SELECT url, title, description, content, 1.0 AS rank
				FROM scrape_cache
				WHERE title LIKE ? OR content LIKE ?
				LIMIT ?
			`, likeTerm, likeTerm, limit)
			if err != nil {
				return nil, err
			}
		}
		defer rows.Close()

		var results []models.SearchResult
		for rows.Next() {
			var r models.SearchResult
			var desc string
			var rank float64
			if err := rows.Scan(&r.URL, &r.Title, &desc, &r.Content, &rank); err != nil {
				continue
			}
			r.Snippet = desc
			r.Scraped = true
			r.Source = "local_index"
			r.Score = rank
			results = append(results, r)
		}
		return results, nil
	}

	// 2. PostgreSQL tsvector Search
	if backend == "postgres" && pPool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		rows, err := pPool.Query(ctx, `
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

	return nil, nil
}
