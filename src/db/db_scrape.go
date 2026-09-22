package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"time"

	"src/models"
)

// GetScrapeCache retrieves cached page extraction content checking Redis, SQLite, PostgreSQL, or memory.
func GetScrapeCache(targetURL string) (models.ScrapeResult, bool) {
	// 1. Check Redis L1 cache first
	if res, found := RedisGetScrapeCache(targetURL); found {
		return res, true
	}

	// 2. Check in-memory cache
	if val, found := scrapeMemoryCache.Load(targetURL); found {
		if entry, ok := val.(inMemoryScrapeEntry); ok && time.Now().Before(entry.expiresAt) {
			res := entry.result
			res.Cached = true
			return res, true
		}
	}

	dbMu.RLock()
	backend := dbBackend
	sDB := sqliteDB
	pPool := pgPool
	dbMu.RUnlock()

	// 3. Check SQLite persistent cache
	if backend == "sqlite" && sDB != nil {
		var r models.ScrapeResult
		var outboundLinksJSON string
		var metadataJSON string
		var structuredDataJSON string
		var errorMsg sql.NullString
		var publishedAt sql.NullTime
		var scrapedInt int

		err := sDB.QueryRow(`
			SELECT url, canonical_url, domain, title, content, markdown, metadata, structured_data, word_count,
			       description, author, published_at, language, outbound_links,
			       status_code, content_type, scraped, extraction_method, error_msg,
			       fetch_duration_ms
			FROM scrape_cache
			WHERE url = ? AND expires_at > CURRENT_TIMESTAMP
		`, targetURL).Scan(
			&r.URL, &r.CanonicalURL, &r.Domain, &r.Title, &r.Content, &r.Markdown, &metadataJSON, &structuredDataJSON, &r.WordCount,
			&r.Description, &r.Author, &publishedAt, &r.Language, &outboundLinksJSON,
			&r.StatusCode, &r.ContentType, &scrapedInt, &r.ExtractionMethod, &errorMsg,
			&r.FetchDurationMS,
		)

		if err != nil {
			return models.ScrapeResult{}, false
		}

		r.Scraped = (scrapedInt != 0)
		if errorMsg.Valid {
			r.Error = errorMsg.String
		}
		if publishedAt.Valid {
			r.PublishedAt = &publishedAt.Time
		}
		if len(outboundLinksJSON) > 0 {
			_ = json.Unmarshal([]byte(outboundLinksJSON), &r.OutboundLinks)
		}
		if len(metadataJSON) > 0 {
			_ = json.Unmarshal([]byte(metadataJSON), &r.Metadata)
		}
		if len(structuredDataJSON) > 0 && structuredDataJSON != "null" {
			r.StructuredData = json.RawMessage(structuredDataJSON)
		}

		r.Cached = true
		// Populate Redis
		RedisSaveScrapeCache(r, scrapeCacheTTL)
		return r, true
	}

	// 4. Check PostgreSQL persistent cache
	if backend == "postgres" && pPool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var r models.ScrapeResult
		var outboundLinksJSON []byte
		var metadataJSON []byte
		var structuredDataJSON []byte
		var errorMsg *string

		err := pPool.QueryRow(ctx, `
			SELECT url, canonical_url, domain, title, content, markdown, metadata, structured_data, word_count,
			       description, author, published_at, language, outbound_links,
			       status_code, content_type, scraped, extraction_method, error_msg,
			       fetch_duration_ms
			FROM scrape_cache
			WHERE url = $1 AND expires_at > CURRENT_TIMESTAMP
		`, targetURL).Scan(
			&r.URL, &r.CanonicalURL, &r.Domain, &r.Title, &r.Content, &r.Markdown, &metadataJSON, &structuredDataJSON, &r.WordCount,
			&r.Description, &r.Author, &r.PublishedAt, &r.Language, &outboundLinksJSON,
			&r.StatusCode, &r.ContentType, &r.Scraped, &r.ExtractionMethod, &errorMsg,
			&r.FetchDurationMS,
		)

		if err != nil {
			return models.ScrapeResult{}, false
		}

		if errorMsg != nil {
			r.Error = *errorMsg
		}
		if len(outboundLinksJSON) > 0 {
			_ = json.Unmarshal(outboundLinksJSON, &r.OutboundLinks)
		}
		if len(metadataJSON) > 0 {
			_ = json.Unmarshal(metadataJSON, &r.Metadata)
		}
		if len(structuredDataJSON) > 0 {
			r.StructuredData = append(json.RawMessage(nil), structuredDataJSON...)
		}

		r.Cached = true
		RedisSaveScrapeCache(r, scrapeCacheTTL)
		return r, true
	}

	return models.ScrapeResult{}, false
}

// SaveScrapeCache stores page extraction content in Redis, SQLite, PostgreSQL, and memory.
func SaveScrapeCache(r models.ScrapeResult) {
	// 1. Save in memory
	scrapeMemoryCache.Store(r.URL, inMemoryScrapeEntry{
		result:    r,
		expiresAt: time.Now().Add(scrapeCacheTTL),
	})

	// 2. Save in Redis
	RedisSaveScrapeCache(r, scrapeCacheTTL)

	dbMu.RLock()
	backend := dbBackend
	sDB := sqliteDB
	pPool := pgPool
	dbMu.RUnlock()

	outboundLinksJSON, err := json.Marshal(r.OutboundLinks)
	if err != nil {
		outboundLinksJSON = []byte("[]")
	}
	metadataJSON, err := json.Marshal(r.Metadata)
	if err != nil {
		metadataJSON = []byte("{}")
	}
	structuredDataJSON := []byte("null")
	if len(r.StructuredData) > 0 {
		structuredDataJSON = append([]byte(nil), r.StructuredData...)
	}

	var errorMsg *string
	if r.Error != "" {
		errorMsg = &r.Error
	}

	expiresAt := time.Now().Add(scrapeCacheTTL)

	// 3. Save to SQLite
	if backend == "sqlite" && sDB != nil {
		scrapedInt := 0
		if r.Scraped {
			scrapedInt = 1
		}

		_, err = sDB.Exec(`
			INSERT INTO scrape_cache (
				url, canonical_url, domain, title, content, markdown, metadata, structured_data, word_count,
				description, author, published_at, language, outbound_links,
				status_code, content_type, scraped, extraction_method, error_msg,
				fetch_duration_ms, created_at, expires_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?)
			ON CONFLICT(url) DO UPDATE SET
				canonical_url       = excluded.canonical_url,
				domain              = excluded.domain,
				title               = excluded.title,
				content             = excluded.content,
				markdown            = excluded.markdown,
				metadata            = excluded.metadata,
				structured_data     = excluded.structured_data,
				word_count          = excluded.word_count,
				description         = excluded.description,
				author              = excluded.author,
				published_at        = excluded.published_at,
				language            = excluded.language,
				outbound_links      = excluded.outbound_links,
				status_code         = excluded.status_code,
				content_type        = excluded.content_type,
				scraped             = excluded.scraped,
				extraction_method   = excluded.extraction_method,
				error_msg           = excluded.error_msg,
				fetch_duration_ms   = excluded.fetch_duration_ms,
				created_at          = CURRENT_TIMESTAMP,
				expires_at          = excluded.expires_at
		`,
			r.URL, r.CanonicalURL, r.Domain, r.Title, r.Content, r.Markdown, string(metadataJSON), string(structuredDataJSON), r.WordCount,
			r.Description, r.Author, r.PublishedAt, r.Language, string(outboundLinksJSON),
			r.StatusCode, r.ContentType, scrapedInt, r.ExtractionMethod, errorMsg,
			r.FetchDurationMS, expiresAt,
		)

		if err != nil {
			log.Printf("[Database] SQLite failed to save scrape cache: %v", err)
		}
		return
	}

	// 4. Save to PostgreSQL
	if backend == "postgres" && pPool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, err = pPool.Exec(ctx, `
			INSERT INTO scrape_cache (
				url, canonical_url, domain, title, content, markdown, metadata, structured_data, word_count,
				description, author, published_at, language, outbound_links,
				status_code, content_type, scraped, extraction_method, error_msg,
				fetch_duration_ms, created_at, expires_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, CURRENT_TIMESTAMP, $21)
			ON CONFLICT (url) DO UPDATE SET
				canonical_url       = EXCLUDED.canonical_url,
				domain              = EXCLUDED.domain,
				title               = EXCLUDED.title,
				content             = EXCLUDED.content,
				markdown            = EXCLUDED.markdown,
				metadata            = EXCLUDED.metadata,
				structured_data     = EXCLUDED.structured_data,
				word_count          = EXCLUDED.word_count,
				description         = EXCLUDED.description,
				author              = EXCLUDED.author,
				published_at        = EXCLUDED.published_at,
				language            = EXCLUDED.language,
				outbound_links      = EXCLUDED.outbound_links,
				status_code         = EXCLUDED.status_code,
				content_type        = EXCLUDED.content_type,
				scraped             = EXCLUDED.scraped,
				extraction_method   = EXCLUDED.extraction_method,
				error_msg           = EXCLUDED.error_msg,
				fetch_duration_ms   = EXCLUDED.fetch_duration_ms,
				created_at          = CURRENT_TIMESTAMP,
				expires_at          = EXCLUDED.expires_at
		`,
			r.URL, r.CanonicalURL, r.Domain, r.Title, r.Content, r.Markdown, metadataJSON, structuredDataJSON, r.WordCount,
			r.Description, r.Author, r.PublishedAt, r.Language, outboundLinksJSON,
			r.StatusCode, r.ContentType, r.Scraped, r.ExtractionMethod, errorMsg,
			r.FetchDurationMS, expiresAt,
		)

		if err != nil {
			log.Printf("[Database] PostgreSQL failed to save scrape cache: %v", err)
		}
	}
}

// StartCacheCleanupWorker initiates a background routine that regularly removes expired cache rows.
func StartCacheCleanupWorker(ctx context.Context) {
	ticker := time.NewTicker(12 * time.Hour)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runCleanup()
			}
		}
	}()
}

func runCleanup() {
	dbMu.RLock()
	backend := dbBackend
	sDB := sqliteDB
	pPool := pgPool
	dbMu.RUnlock()

	if backend == "sqlite" && sDB != nil {
		_, _ = sDB.Exec("DELETE FROM search_cache WHERE created_at < DATETIME('now', '-' || ? || ' seconds')", int(searchCacheTTL.Seconds()))
		_, _ = sDB.Exec("DELETE FROM scrape_cache WHERE expires_at < CURRENT_TIMESTAMP")
		return
	}

	if backend == "postgres" && pPool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, _ = pPool.Exec(ctx, `
			DELETE FROM search_cache 
			WHERE created_at < CURRENT_TIMESTAMP - $1::interval
		`, searchCacheTTL.String())

		_, _ = pPool.Exec(ctx, `
			DELETE FROM scrape_cache 
			WHERE expires_at < CURRENT_TIMESTAMP
		`)
	}
}
