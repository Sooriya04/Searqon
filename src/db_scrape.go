package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"
)

// ─── Scrape Cache Helpers ───────────────────────────────────────────────────

func getScrapeCache(targetURL string) (ScrapeResult, bool) {
	dbMu.RLock()
	enabled := dbEnabled
	pool := dbPool
	dbMu.RUnlock()

	if !enabled || pool == nil {
		return ScrapeResult{}, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var res ScrapeResult
	var canonicalURL, title, content, markdown, description, author, language, contentType, extractionMethod, errorMsg *string
	var wordCount, statusCode, fetchDurationMS *int
	var outboundLinks []byte
	var publishedAt *time.Time
	var isScraped bool
	var createdAt time.Time

	err := pool.QueryRow(ctx, `
		SELECT canonical_url, domain, title, content, markdown, word_count, description, author, published_at, language, outbound_links, status_code, content_type, scraped, extraction_method, error_msg, fetch_duration_ms, created_at
		FROM scrape_cache WHERE url = $1 AND expires_at > CURRENT_TIMESTAMP LIMIT 1
	`, targetURL).Scan(
		&canonicalURL, &res.Domain, &title, &content, &markdown, &wordCount, &description, &author, &publishedAt, &language, &outboundLinks, &statusCode, &contentType, &isScraped, &extractionMethod, &errorMsg, &fetchDurationMS, &createdAt,
	)

	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			log.Printf("[Database] Error querying scrape_cache for %q: %v", targetURL, err)
		}
		return ScrapeResult{}, false
	}

	res.URL = targetURL
	if canonicalURL != nil { res.CanonicalURL = *canonicalURL }
	if title != nil { res.Title = *title }
	if content != nil { res.Content = *content }
	if markdown != nil { res.Markdown = *markdown }
	if wordCount != nil { res.WordCount = *wordCount }
	if description != nil { res.Description = *description }
	if author != nil { res.Author = *author }
	res.PublishedAt = publishedAt
	if language != nil { res.Language = *language }
	if statusCode != nil { res.StatusCode = *statusCode }
	if contentType != nil { res.ContentType = *contentType }
	res.Scraped = isScraped
	if extractionMethod != nil { res.ExtractionMethod = *extractionMethod }
	if errorMsg != nil { res.Error = *errorMsg }
	if fetchDurationMS != nil { res.FetchDurationMS = *fetchDurationMS }

	res.StartTime = createdAt.UTC().Format(time.RFC3339)
	res.EndTime = res.StartTime
	res.Duration = int64(res.FetchDurationMS)

	if len(outboundLinks) > 0 {
		var links []string
		if err := json.Unmarshal(outboundLinks, &links); err == nil {
			res.OutboundLinks = links
		}
	}

	return res, true
}

func saveScrapeCache(res ScrapeResult) {
	dbMu.RLock()
	enabled := dbEnabled
	pool := dbPool
	ttl := scrapeCacheTTL
	dbMu.RUnlock()

	if !enabled || pool == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var errorMsg *string
	if res.Error != "" {
		errorMsg = &res.Error
	}

	domain := res.Domain
	if domain == "" {
		if u, err := url.Parse(res.URL); err == nil {
			domain = u.Hostname()
		}
	}
	if domain == "" {
		domain = "unknown"
	}

	linksJSON, _ := json.Marshal(res.OutboundLinks)
	if len(res.OutboundLinks) == 0 {
		linksJSON = []byte("[]")
	}

	expiresAt := time.Now().Add(ttl)

	_, err := pool.Exec(ctx, `
		INSERT INTO scrape_cache (
			url, canonical_url, domain, title, content, markdown, word_count, 
			description, author, published_at, language, outbound_links, 
			status_code, content_type, scraped, extraction_method, error_msg, 
			fetch_duration_ms, created_at, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, CURRENT_TIMESTAMP, $19)
		ON CONFLICT (url) DO UPDATE
		SET canonical_url = EXCLUDED.canonical_url,
		    domain = EXCLUDED.domain,
		    title = EXCLUDED.title,
		    content = EXCLUDED.content,
		    markdown = EXCLUDED.markdown,
		    word_count = EXCLUDED.word_count,
		    description = EXCLUDED.description,
		    author = EXCLUDED.author,
		    published_at = EXCLUDED.published_at,
		    language = EXCLUDED.language,
		    outbound_links = EXCLUDED.outbound_links,
		    status_code = EXCLUDED.status_code,
		    content_type = EXCLUDED.content_type,
		    scraped = EXCLUDED.scraped,
		    extraction_method = EXCLUDED.extraction_method,
		    error_msg = EXCLUDED.error_msg,
		    fetch_duration_ms = EXCLUDED.fetch_duration_ms,
		    created_at = CURRENT_TIMESTAMP,
		    expires_at = EXCLUDED.expires_at
	`, res.URL, res.CanonicalURL, domain, res.Title, res.Content, res.Markdown, res.WordCount,
		res.Description, res.Author, res.PublishedAt, res.Language, linksJSON,
		res.StatusCode, res.ContentType, res.Scraped, res.ExtractionMethod, errorMsg,
		res.FetchDurationMS, expiresAt)

	if err != nil {
		log.Printf("[Database] Error saving scrape cache for %q: %v", res.URL, err)
	}
}

// ─── Cache Cleanup Background Worker ──────────────────────────────────────────

func StartCacheCleanupWorker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		log.Println("[Database] Started cache cleanup background worker (interval: 1 hour).")
		// Run initial cleanup on startup
		cleanupExpiredCache()

		for {
			select {
			case <-ticker.C:
				cleanupExpiredCache()
			case <-ctx.Done():
				ticker.Stop()
				log.Println("[Database] Stopped cache cleanup background worker.")
				return
			}
		}
	}()
}

func cleanupExpiredCache() {
	dbMu.RLock()
	enabled := dbEnabled
	pool := dbPool
	searchTTL := searchCacheTTL
	dbMu.RUnlock()

	if !enabled || pool == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Delete expired search cache entries
	searchLimit := time.Now().Add(-searchTTL)
	searchTag, err := pool.Exec(ctx, "DELETE FROM search_cache WHERE created_at < $1", searchLimit)
	if err != nil {
		log.Printf("[Database] Error cleaning expired search_cache entries: %v", err)
	} else if affected := searchTag.RowsAffected(); affected > 0 {
		log.Printf("[Database] Evicted %d expired search cache entries.", affected)
	}

	// Delete expired scrape cache entries
	scrapeTag, err := pool.Exec(ctx, "DELETE FROM scrape_cache WHERE expires_at < CURRENT_TIMESTAMP")
	if err != nil {
		log.Printf("[Database] Error cleaning expired scrape_cache entries: %v", err)
	} else if affected := scrapeTag.RowsAffected(); affected > 0 {
		log.Printf("[Database] Evicted %d expired scrape cache entries.", affected)
	}
}
