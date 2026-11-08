package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"src/config"
	"src/models"
)

func TestSQLiteDatabaseOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "searqon_db_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test_searqon.db")

	cfg := config.DefaultConfig()
	cfg.Database.Enabled = true
	cfg.Database.Type = "sqlite"
	cfg.Database.SQLite.Path = dbPath
	cfg.Redis.Enabled = false

	initSQLite(dbPath)
	defer CloseDB()

	if !DbEnabled() {
		t.Fatalf("expected DB to be enabled")
	}
	if GetBackend() != "sqlite" {
		t.Fatalf("expected backend 'sqlite', got '%s'", GetBackend())
	}

	// 1. Test Search Cache
	testResults := []models.SearchResult{
		{
			Title:   "Searqon Intelligence",
			URL:     "https://searqon.local",
			Snippet: "Deep search and scraping engine",
			Source:  "test",
		},
	}
	embedding := []float32{0.1, 0.2, 0.3, 0.9}
	SaveSearchCacheWithEmbedding("golang intelligence", testResults, "searxng", embedding)

	cached, provider, found := GetSearchCache("golang intelligence")
	if !found {
		t.Fatalf("expected to find cached search results")
	}
	if provider != "searxng" {
		t.Fatalf("expected provider 'searxng', got '%s'", provider)
	}
	if len(cached) != 1 || cached[0].Title != "Searqon Intelligence" {
		t.Fatalf("unexpected cached results: %+v", cached)
	}

	// 1b. Test Semantic Cache Near-Match
	nearEmbedding := []float32{0.1, 0.2, 0.31, 0.89}
	semResults, semProvider, sim, semFound := GetSemanticSearchCache("golang ai agent intelligence", nearEmbedding, 0.92)
	if !semFound {
		t.Fatalf("expected semantic cache near match to be found")
	}
	if semProvider != "searxng" {
		t.Fatalf("expected provider 'searxng', got '%s'", semProvider)
	}
	if sim < 0.92 {
		t.Fatalf("expected similarity >= 0.92, got %f", sim)
	}
	if len(semResults) != 1 || semResults[0].Title != "Searqon Intelligence" {
		t.Fatalf("unexpected semantic cached results: %+v", semResults)
	}

	// Dissimilar query should NOT match
	dissimilarEmbedding := []float32{0.9, -0.2, 0.0, 0.1}
	_, _, _, dissFound := GetSemanticSearchCache("completely different topic", dissimilarEmbedding, 0.92)
	if dissFound {
		t.Fatalf("expected dissimilar query not to match semantic cache")
	}

	// 2. Test Scrape Cache
	nowISO := time.Now().UTC().Format(time.RFC3339)
	scrapeRes := models.ScrapeResult{
		URL:         "https://example.com/test",
		Title:       "Test Page",
		Content:     "This is full text content for extraction verification.",
		Markdown:    "# Test Page\n\nThis is full text content.",
		WordCount:   8,
		Scraped:     true,
		StatusCode:  200,
		ContentType: "text/html",
		ContentHash: "a1b2c3d4e5f6",
		ScrapedAt:   nowISO,
	}
	SaveScrapeCache(scrapeRes)

	cachedScrape, foundScrape := GetScrapeCache("https://example.com/test")
	if !foundScrape {
		t.Fatalf("expected to find cached scrape result")
	}
	if cachedScrape.Title != "Test Page" || cachedScrape.WordCount != 8 {
		t.Fatalf("unexpected scrape cache result: %+v", cachedScrape)
	}
	if cachedScrape.ContentHash != "a1b2c3d4e5f6" {
		t.Fatalf("expected content_hash 'a1b2c3d4e5f6', got '%s'", cachedScrape.ContentHash)
	}
	if cachedScrape.ScrapedAt == "" {
		t.Fatalf("expected scraped_at to be populated")
	}

	// 3. Test Sessions
	err = SaveSessionCookies("example.com", `[{"name":"session_id","value":"xyz123"}]`)
	if err != nil {
		t.Fatalf("failed to save session cookies: %v", err)
	}
	cookies, err := GetSessionCookies("example.com")
	if err != nil {
		t.Fatalf("failed to get session cookies: %v", err)
	}
	if cookies != `[{"name":"session_id","value":"xyz123"}]` {
		t.Fatalf("unexpected cookies: %s", cookies)
	}

	// 4. Test Local Index FTS
	IndexScrapeInFTS("https://example.com/test", "Test Page", "A short description", "This is full text content for extraction verification.")
	time.Sleep(50 * time.Millisecond)

	ftsResults, err := SearchLocalIndex("extraction verification", 5)
	if err != nil {
		t.Fatalf("failed to search local index: %v", err)
	}
	if len(ftsResults) == 0 {
		t.Fatalf("expected at least 1 result from FTS search")
	}
	if ftsResults[0].URL != "https://example.com/test" {
		t.Fatalf("unexpected FTS result URL: %s", ftsResults[0].URL)
	}
}

func TestSQLiteSchemaMigrationPreExistingDB(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "searqon_migration_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "legacy_searqon.db")

	// Create legacy database lacking content_hash and scraped_at
	legacyDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to create legacy db: %v", err)
	}
	legacySchema := `
	CREATE TABLE scrape_cache (
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
	`
	if _, err := legacyDB.Exec(legacySchema); err != nil {
		legacyDB.Close()
		t.Fatalf("failed to create legacy scrape_cache table: %v", err)
	}
	legacyDB.Close()

	// Initialize with Searqon - this triggers initSQLite and migrateSQLiteTables
	initSQLite(dbPath)
	defer CloseDB()

	// Verify columns exist now
	rows, err := sqliteDB.Query("PRAGMA table_info(scrape_cache);")
	if err != nil {
		t.Fatalf("PRAGMA table_info failed: %v", err)
	}
	defer rows.Close()

	cols := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err == nil {
			cols[name] = true
		}
	}

	if !cols["content_hash"] {
		t.Errorf("migration failed: content_hash column missing")
	}
	if !cols["scraped_at"] {
		t.Errorf("migration failed: scraped_at column missing")
	}

	// Verify SaveScrapeCache works on this migrated DB without error
	nowISO := time.Now().UTC().Format(time.RFC3339)
	res := models.ScrapeResult{
		URL:         "https://migrated.local/article",
		Title:       "Migrated Page",
		Content:     "Content on migrated sqlite table",
		Markdown:    "Content on migrated sqlite table",
		Scraped:     true,
		ContentHash: "hash-legacy-12345",
		ScrapedAt:   nowISO,
	}
	SaveScrapeCache(res)

	got, found := GetScrapeCache("https://migrated.local/article")
	if !found {
		t.Fatalf("expected to find saved scrape cache in migrated table")
	}
	if got.ContentHash != "hash-legacy-12345" {
		t.Errorf("expected ContentHash 'hash-legacy-12345', got '%s'", got.ContentHash)
	}
}
