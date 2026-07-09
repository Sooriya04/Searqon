package db

import (
	"context"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"src/models"
)

type inMemorySearchEntry struct {
	results   []models.SearchResult
	provider  string
	createdAt time.Time
}

type inMemoryScrapeEntry struct {
	result    models.ScrapeResult
	expiresAt time.Time
}

var (
	dbPool            *pgxpool.Pool
	dbEnabled         bool
	dbMu              sync.RWMutex
	searchCacheTTL    time.Duration = 24 * time.Hour
	scrapeCacheTTL    time.Duration = 7 * 24 * time.Hour
	cleanupCancel     context.CancelFunc
	searchMemoryCache = make(map[string]inMemorySearchEntry)
	scrapeMemoryCache = make(map[string]inMemoryScrapeEntry)
	memoryMu          sync.Mutex
)

// InitDB initializes the database connection pool and schema.
func InitDB() {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Println("[Database] DATABASE_URL is empty. Cache database is DISABLED.")
		dbMu.Lock()
		dbEnabled = false
		dbMu.Unlock()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		log.Printf("[Database] Failed to parse DATABASE_URL: %v. Database is DISABLED.", err)
		dbMu.Lock()
		dbEnabled = false
		dbMu.Unlock()
		return
	}

	config.MaxConns = 15
	config.MinConns = 2
	config.MaxConnIdleTime = 15 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Printf("[Database] Failed to init pool: %v. Database is DISABLED.", err)
		dbMu.Lock()
		dbEnabled = false
		dbMu.Unlock()
		return
	}

	if err := pool.Ping(ctx); err != nil {
		log.Printf("[Database] Ping test failed: %v. Database is DISABLED.", err)
		pool.Close()
		dbMu.Lock()
		dbEnabled = false
		dbMu.Unlock()
		return
	}

	dbMu.Lock()
	dbPool = pool
	dbEnabled = true
	dbMu.Unlock()

	log.Println("[Database] PostgreSQL connection pool initialized successfully.")

	if searchTTLHours := os.Getenv("SEARCH_CACHE_TTL_HOURS"); searchTTLHours != "" {
		if hours, err := strconv.Atoi(searchTTLHours); err == nil && hours > 0 {
			searchCacheTTL = time.Duration(hours) * time.Hour
		}
	}
	if scrapeTTLDays := os.Getenv("SCRAPE_CACHE_TTL_DAYS"); scrapeTTLDays != "" {
		if days, err := strconv.Atoi(scrapeTTLDays); err == nil && days > 0 {
			scrapeCacheTTL = time.Duration(days) * 24 * time.Hour
		}
	}

	initTables()
	InitVectorDB()
	InitFullTextSearch()

	cleanupCtx, cleanupCancelFunc := context.WithCancel(context.Background())
	cleanupCancel = cleanupCancelFunc
	StartCacheCleanupWorker(cleanupCtx)
}

// CloseDB closes the connection pool.
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

// DbEnabled returns database enabled status.
func DbEnabled() bool {
	dbMu.RLock()
	defer dbMu.RUnlock()
	return dbEnabled
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
		title               TEXT,
		content             TEXT,
		markdown            TEXT,
		word_count          INTEGER DEFAULT 0,
		description         TEXT,
		author              TEXT,
		published_at        TIMESTAMP WITH TIME ZONE,
		language            TEXT,
		outbound_links      JSONB DEFAULT '[]',
		status_code         INTEGER,
		content_type        TEXT,
		scraped             BOOLEAN DEFAULT TRUE,
		extraction_method   TEXT,
		error_msg           TEXT,
		fetch_duration_ms   INTEGER,
		created_at          TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		expires_at          TIMESTAMP WITH TIME ZONE DEFAULT (CURRENT_TIMESTAMP + INTERVAL '24 hours')
	);

	CREATE INDEX IF NOT EXISTS idx_search_cache_created_at ON search_cache(created_at);
	CREATE INDEX IF NOT EXISTS idx_scrape_cache_created_at ON scrape_cache(created_at);

	ALTER TABLE scrape_cache ADD COLUMN IF NOT EXISTS canonical_url TEXT;
	ALTER TABLE scrape_cache ADD COLUMN IF NOT EXISTS domain TEXT NOT NULL DEFAULT 'unknown';
	ALTER TABLE scrape_cache ADD COLUMN IF NOT EXISTS description TEXT;
	ALTER TABLE scrape_cache ADD COLUMN IF NOT EXISTS author TEXT;
	ALTER TABLE scrape_cache ADD COLUMN IF NOT EXISTS published_at TIMESTAMP WITH TIME ZONE;
	ALTER TABLE scrape_cache ADD COLUMN IF NOT EXISTS language TEXT;
	ALTER TABLE scrape_cache ADD COLUMN IF NOT EXISTS outbound_links JSONB DEFAULT '[]';
	ALTER TABLE scrape_cache ADD COLUMN IF NOT EXISTS status_code INTEGER;
	ALTER TABLE scrape_cache ADD COLUMN IF NOT EXISTS content_type TEXT;
	ALTER TABLE scrape_cache ADD COLUMN IF NOT EXISTS scraped BOOLEAN DEFAULT TRUE;
	ALTER TABLE scrape_cache ADD COLUMN IF NOT EXISTS extraction_method TEXT;
	ALTER TABLE scrape_cache ADD COLUMN IF NOT EXISTS error_msg TEXT;
	ALTER TABLE scrape_cache ADD COLUMN IF NOT EXISTS fetch_duration_ms INTEGER;
	ALTER TABLE scrape_cache ADD COLUMN IF NOT EXISTS expires_at TIMESTAMP WITH TIME ZONE DEFAULT (CURRENT_TIMESTAMP + INTERVAL '24 hours');

	CREATE INDEX IF NOT EXISTS idx_scrape_cache_domain ON scrape_cache(domain);
	CREATE INDEX IF NOT EXISTS idx_scrape_cache_expires_at ON scrape_cache(expires_at);
	`

	_, err := dbPool.Exec(ctx, schema)
	if err != nil {
		log.Printf("[Database] Error creating tables: %v", err)
	}
}
