package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

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
		url                 TEXT PRIMARY KEY,
		canonical_url       TEXT,
		domain              TEXT NOT NULL,

		-- Core content
		title               TEXT,
		content             TEXT,
		markdown            TEXT,
		word_count          INTEGER DEFAULT 0,

		-- Metadata
		description         TEXT,
		author              TEXT,
		published_at        TIMESTAMP WITH TIME ZONE,
		language            TEXT,

		-- Outbound links (for crawler frontier)
		outbound_links      JSONB DEFAULT '[]',

		-- HTTP / technical
		status_code         INTEGER,
		content_type        TEXT,

		-- Scrape status
		scraped             BOOLEAN DEFAULT TRUE,
		extraction_method   TEXT,              -- 'readability' | 'goquery' | 'failed'
		error_msg           TEXT,
		fetch_duration_ms   INTEGER,

		-- Cache lifecycle
		created_at          TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		expires_at          TIMESTAMP WITH TIME ZONE DEFAULT (CURRENT_TIMESTAMP + INTERVAL '24 hours')
	);

	CREATE INDEX IF NOT EXISTS idx_search_cache_created_at ON search_cache(created_at);
	CREATE INDEX IF NOT EXISTS idx_scrape_cache_created_at ON scrape_cache(created_at);
	CREATE INDEX IF NOT EXISTS idx_scrape_cache_domain ON scrape_cache(domain);
	CREATE INDEX IF NOT EXISTS idx_scrape_cache_expires_at ON scrape_cache(expires_at);
	`

	_, err := dbPool.Exec(ctx, schema)
	if err != nil {
		log.Printf("[Database] Error checking/creating tables: %v", err)
	} else {
		log.Println("[Database] Tables verified and initialized.")
	}
}
