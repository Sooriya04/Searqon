package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	dbPool         *pgxpool.Pool
	dbEnabled      bool
	dbMu           sync.RWMutex
	searchCacheTTL time.Duration = 24 * time.Hour
	scrapeCacheTTL time.Duration = 7 * 24 * time.Hour
	cleanupCancel  context.CancelFunc
)

// InitDB initializes the PostgreSQL connection pool.
// If DATABASE_URL is not set, it gracefully disables the cache and continues running.
func InitDB() {
	dbMu.Lock()
	defer dbMu.Unlock()

	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Println("[Database] DATABASE_URL environment variable is empty. Cache database is DISABLED (no-cache mode).")
		dbEnabled = false
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		log.Printf("[Database] Failed to parse DATABASE_URL: %v. Running with database DISABLED.", err)
		dbEnabled = false
		return
	}

	// Adjust connection pool configurations for performance
	config.MaxConns = 15
	config.MinConns = 2
	config.MaxConnIdleTime = 15 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Printf("[Database] Failed to initialize connection pool: %v. Running with database DISABLED.", err)
		dbEnabled = false
		return
	}

	// Ping check
	if err := pool.Ping(ctx); err != nil {
		log.Printf("[Database] Connection test failed: %v. Running with database DISABLED.", err)
		pool.Close()
		dbEnabled = false
		return
	}

	dbPool = pool
	dbEnabled = true
	log.Println("[Database] Connection pool initialized successfully. Cache database is ENABLED.")

	// Load TTL configs
	if searchTTLHours := os.Getenv("SEARCH_CACHE_TTL_HOURS"); searchTTLHours != "" {
		if hours, err := strconv.Atoi(searchTTLHours); err == nil && hours > 0 {
			searchCacheTTL = time.Duration(hours) * time.Hour
			log.Printf("[Database] Configured search cache TTL to %d hours", hours)
		}
	}
	if scrapeTTLDays := os.Getenv("SCRAPE_CACHE_TTL_DAYS"); scrapeTTLDays != "" {
		if days, err := strconv.Atoi(scrapeTTLDays); err == nil && days > 0 {
			scrapeCacheTTL = time.Duration(days) * 24 * time.Hour
			log.Printf("[Database] Configured scrape cache TTL to %d days", days)
		}
	}

	// Auto-initialize tables if they do not exist
	initTables()

	// Start cache cleanup background worker
	cleanupCtx, cleanupCancelFunc := context.WithCancel(context.Background())
	cleanupCancel = cleanupCancelFunc
	StartCacheCleanupWorker(cleanupCtx)
}

// CloseDB closes the database connection pool safely.
func CloseDB() {
	dbMu.Lock()
	defer dbMu.Unlock()
	if cleanupCancel != nil {
		cleanupCancel()
	}
	if dbEnabled && dbPool != nil {
		dbPool.Close()
		log.Println("[Database] Connection pool closed.")
	}
}

func initTables() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	schema := `
	CREATE TABLE IF NOT EXISTS search_cache (
		query TEXT PRIMARY KEY,
		results JSONB NOT NULL,
		provider TEXT NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS scrape_cache (
		url TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		markdown TEXT NOT NULL,
		word_count INTEGER NOT NULL,
		scraped BOOLEAN DEFAULT TRUE,
		error_msg TEXT,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_search_cache_created_at ON search_cache(created_at);
	CREATE INDEX IF NOT EXISTS idx_scrape_cache_created_at ON scrape_cache(created_at);
	`

	_, err := dbPool.Exec(ctx, schema)
	if err != nil {
		log.Printf("[Database] Error checking/creating tables: %v", err)
	} else {
		log.Println("[Database] Tables verified and initialized.")
	}
}

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

// ─── Scrape Cache Helpers ───────────────────────────────────────────────────

func getScrapeCache(targetURL string) (ScrapeResult, bool) {
	dbMu.RLock()
	enabled := dbEnabled
	pool := dbPool
	ttl := scrapeCacheTTL
	dbMu.RUnlock()

	if !enabled || pool == nil {
		return ScrapeResult{}, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var res ScrapeResult
	var errorMsg *string
	var isScraped bool

	minCreatedAt := time.Now().Add(-ttl)
	err := pool.QueryRow(ctx, `
		SELECT title, content, markdown, word_count, scraped, error_msg, created_at
		FROM scrape_cache WHERE url = $1 AND created_at > $2 LIMIT 1
	`, targetURL, minCreatedAt).Scan(&res.Title, &res.Content, &res.Markdown, &res.WordCount, &isScraped, &errorMsg, &res.StartTime)

	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			log.Printf("[Database] Error querying scrape_cache for %q: %v", targetURL, err)
		}
		return ScrapeResult{}, false
	}

	res.URL = targetURL
	res.EndTime = res.StartTime
	if errorMsg != nil {
		res.Error = *errorMsg
	}

	return res, true
}

func saveScrapeCache(res ScrapeResult) {
	dbMu.RLock()
	enabled := dbEnabled
	pool := dbPool
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
	isScraped := res.Error == ""

	_, err := pool.Exec(ctx, `
		INSERT INTO scrape_cache (url, title, content, markdown, word_count, scraped, error_msg, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP)
		ON CONFLICT (url) DO UPDATE
		SET title = EXCLUDED.title, content = EXCLUDED.content, markdown = EXCLUDED.markdown,
		    word_count = EXCLUDED.word_count, scraped = EXCLUDED.scraped, error_msg = EXCLUDED.error_msg,
		    created_at = CURRENT_TIMESTAMP
	`, res.URL, res.Title, res.Content, res.Markdown, res.WordCount, isScraped, errorMsg)

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
	scrapeTTL := scrapeCacheTTL
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
	scrapeLimit := time.Now().Add(-scrapeTTL)
	scrapeTag, err := pool.Exec(ctx, "DELETE FROM scrape_cache WHERE created_at < $1", scrapeLimit)
	if err != nil {
		log.Printf("[Database] Error cleaning expired scrape_cache entries: %v", err)
	} else if affected := scrapeTag.RowsAffected(); affected > 0 {
		log.Printf("[Database] Evicted %d expired scrape cache entries.", affected)
	}
}
