package db

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"src/models"
)

// GetScrapeCache retrieves cached page extraction content.
func GetScrapeCache(targetURL string) (models.ScrapeResult, bool) {
	dbMu.RLock()
	enabled := dbEnabled
	pool := dbPool
	dbMu.RUnlock()

	if !enabled || pool == nil {
		memoryMu.Lock()
		defer memoryMu.Unlock()
		entry, found := scrapeMemoryCache[targetURL]
		if found && time.Now().Before(entry.expiresAt) {
			// Mark it as cached
			entry.result.Cached = true
			return entry.result, true
		}
		return models.ScrapeResult{}, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var r models.ScrapeResult
	var outboundLinksJSON []byte
	var metadataJSON []byte
	var structuredDataJSON []byte
	var errorMsg *string

	err := pool.QueryRow(ctx, `
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

	return r, true
}

// SaveScrapeCache stores page extraction content in the database.
func SaveScrapeCache(r models.ScrapeResult) {
	dbMu.RLock()
	enabled := dbEnabled
	pool := dbPool
	dbMu.RUnlock()

	if !enabled || pool == nil {
		memoryMu.Lock()
		scrapeMemoryCache[r.URL] = inMemoryScrapeEntry{
			result:    r,
			expiresAt: time.Now().Add(scrapeCacheTTL),
		}
		memoryMu.Unlock()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

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

	_, err = pool.Exec(ctx, `
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
		log.Printf("[Database] Failed to save scrape cache: %v", err)
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
	enabled := dbEnabled
	pool := dbPool
	dbMu.RUnlock()

	if !enabled || pool == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Delete expired search cache (TTL is 24 hours by default)
	_, err := pool.Exec(ctx, `
		DELETE FROM search_cache 
		WHERE created_at < CURRENT_TIMESTAMP - $1::interval
	`, searchCacheTTL.String())
	if err != nil {
		log.Printf("[Database] Search cache cleanup failed: %v", err)
	}

	// 2. Delete expired scrape cache
	_, err = pool.Exec(ctx, `
		DELETE FROM scrape_cache 
		WHERE expires_at < CURRENT_TIMESTAMP
	`)
	if err != nil {
		log.Printf("[Database] Scrape cache cleanup failed: %v", err)
	}
}
