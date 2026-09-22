package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "modernc.org/sqlite"

	"src/config"
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
	dbBackend         string = "none" // "sqlite", "postgres", "none"
	sqliteDB          *sql.DB
	pgPool            *pgxpool.Pool
	dbEnabled         bool
	dbMu              sync.RWMutex
	searchCacheTTL    time.Duration = 24 * time.Hour
	scrapeCacheTTL    time.Duration = 7 * 24 * time.Hour
	cleanupCancel     context.CancelFunc
	searchMemoryCache sync.Map
	scrapeMemoryCache sync.Map
)

// InitDB initializes the chosen database backend (SQLite or PostgreSQL) and Redis.
func InitDB() {
	cfg := config.Get()

	searchCacheTTL = cfg.SearchCacheTTL()
	scrapeCacheTTL = cfg.ScrapeCacheTTL()

	// 1. Initialize Redis cache layer
	InitRedis(cfg.Redis)

	// 2. Check if primary database is enabled
	if !cfg.Database.Enabled {
		log.Println("[Database] Database is DISABLED in settings. Using high-speed in-memory cache.")
		dbMu.Lock()
		dbEnabled = false
		dbBackend = "none"
		dbMu.Unlock()
		return
	}

	dbType := strings.ToLower(strings.TrimSpace(cfg.Database.Type))
	if dbType == "" || dbType == "sqlite" {
		initSQLite(cfg.Database.SQLite.Path)
	} else if dbType == "postgres" || dbType == "pg" {
		initPostgres(cfg)
	} else {
		log.Printf("[Database] Unknown database type '%s', falling back to SQLite", cfg.Database.Type)
		initSQLite(cfg.Database.SQLite.Path)
	}

	// 3. Start periodic background cleanup worker
	cleanupCtx, cleanupCancelFunc := context.WithCancel(context.Background())
	cleanupCancel = cleanupCancelFunc
	StartCacheCleanupWorker(cleanupCtx)
}

func initSQLite(dbPath string) {
	if dbPath == "" {
		dbPath = "./data/searqon.db"
	}

	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0755)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Printf("[Database] Failed to open SQLite database %s: %v. Using in-memory fallback.", dbPath, err)
		dbMu.Lock()
		dbEnabled = false
		dbBackend = "none"
		dbMu.Unlock()
		return
	}

	// Enable WAL journal mode & concurrent reader performance
	_, _ = db.Exec("PRAGMA journal_mode=WAL;")
	_, _ = db.Exec("PRAGMA synchronous=NORMAL;")
	_, _ = db.Exec("PRAGMA busy_timeout=5000;")

	dbMu.Lock()
	sqliteDB = db
	dbBackend = "sqlite"
	dbEnabled = true
	dbMu.Unlock()

	log.Printf("[Database] SQLite database initialized successfully at %s", dbPath)

	initSQLiteTables()
	InitFullTextSearch()
	InitVectorDB()
}

func initPostgres(cfg *config.Config) {
	connStr := cfg.Database.Postgres.URL
	if connStr == "" {
		p := cfg.Database.Postgres
		if p.Host == "" {
			p.Host = "localhost"
		}
		if p.Port <= 0 {
			p.Port = 5432
		}
		if p.User == "" {
			p.User = "postgres"
		}
		if p.DBName == "" {
			p.DBName = "searqon"
		}
		if p.SSLMode == "" {
			p.SSLMode = "disable"
		}
		connStr = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
			p.User, p.Password, p.Host, p.Port, p.DBName, p.SSLMode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pgConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		log.Printf("[Database] Failed to parse PostgreSQL config: %v. Using in-memory cache.", err)
		dbMu.Lock()
		dbEnabled = false
		dbBackend = "none"
		dbMu.Unlock()
		return
	}

	pgConfig.MaxConns = 15
	pgConfig.MinConns = 2
	pgConfig.MaxConnIdleTime = 15 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, pgConfig)
	if err != nil {
		log.Printf("[Database] Failed to init PostgreSQL pool: %v. Using in-memory cache.", err)
		dbMu.Lock()
		dbEnabled = false
		dbBackend = "none"
		dbMu.Unlock()
		return
	}

	if err := pool.Ping(ctx); err != nil {
		log.Printf("[Database] PostgreSQL ping failed: %v. Using in-memory cache.", err)
		pool.Close()
		dbMu.Lock()
		dbEnabled = false
		dbBackend = "none"
		dbMu.Unlock()
		return
	}

	dbMu.Lock()
	pgPool = pool
	dbBackend = "postgres"
	dbEnabled = true
	dbMu.Unlock()

	log.Println("[Database] PostgreSQL connection pool initialized successfully.")

	initPostgresTables()
	InitFullTextSearch()
	InitVectorDB()
}

func initSQLiteTables() {
	schema := `
	CREATE TABLE IF NOT EXISTS search_cache (
		query TEXT PRIMARY KEY,
		results TEXT NOT NULL,
		provider TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_search_cache_created_at ON search_cache(created_at);

	CREATE TABLE IF NOT EXISTS scrape_cache (
		url                 TEXT PRIMARY KEY,
		canonical_url       TEXT,
		domain              TEXT NOT NULL DEFAULT 'unknown',
		title               TEXT,
		content             TEXT,
		markdown            TEXT,
		metadata            TEXT DEFAULT '{}',
		structured_data     TEXT DEFAULT 'null',
		word_count          INTEGER DEFAULT 0,
		description         TEXT,
		author              TEXT,
		published_at        DATETIME,
		language            TEXT,
		outbound_links      TEXT DEFAULT '[]',
		status_code         INTEGER,
		content_type        TEXT,
		scraped             INTEGER DEFAULT 1,
		extraction_method   TEXT,
		error_msg           TEXT,
		fetch_duration_ms   INTEGER,
		created_at          DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at          DATETIME
	);
	CREATE INDEX IF NOT EXISTS idx_scrape_cache_domain ON scrape_cache(domain);
	CREATE INDEX IF NOT EXISTS idx_scrape_cache_expires_at ON scrape_cache(expires_at);

	CREATE TABLE IF NOT EXISTS sessions (
		domain TEXT PRIMARY KEY,
		cookies TEXT NOT NULL DEFAULT '[]',
		user_agent TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := sqliteDB.Exec(schema)
	if err != nil {
		log.Printf("[Database] Error creating SQLite tables: %v", err)
	}
}

func initPostgresTables() {
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
		domain              TEXT NOT NULL DEFAULT 'unknown',
		title               TEXT,
		content             TEXT,
		markdown            TEXT,
		metadata            JSONB,
		structured_data     JSONB,
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
	CREATE INDEX IF NOT EXISTS idx_scrape_cache_domain ON scrape_cache(domain);
	CREATE INDEX IF NOT EXISTS idx_scrape_cache_expires_at ON scrape_cache(expires_at);

	CREATE TABLE IF NOT EXISTS sessions (
		domain TEXT PRIMARY KEY,
		cookies JSONB NOT NULL DEFAULT '[]',
		user_agent TEXT,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := pgPool.Exec(ctx, schema)
	if err != nil {
		log.Printf("[Database] Error creating PostgreSQL tables: %v", err)
	}
}

// CloseDB closes database connections and workers.
func CloseDB() {
	dbMu.Lock()
	defer dbMu.Unlock()

	if cleanupCancel != nil {
		cleanupCancel()
	}

	CloseRedis()

	if sqliteDB != nil {
		_ = sqliteDB.Close()
		log.Println("[Database] SQLite connection closed.")
	}

	if pgPool != nil {
		pgPool.Close()
		log.Println("[Database] PostgreSQL connection pool closed.")
	}

	dbEnabled = false
	dbBackend = "none"
}

// DbEnabled returns whether persistent database is enabled.
func DbEnabled() bool {
	dbMu.RLock()
	defer dbMu.RUnlock()
	return dbEnabled
}

// GetBackend returns active backend name ("sqlite", "postgres", "none").
func GetBackend() string {
	dbMu.RLock()
	defer dbMu.RUnlock()
	return dbBackend
}
