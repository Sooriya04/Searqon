package db

import (
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
	SaveSearchCache("golang intelligence", testResults, "searxng")

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

	// 2. Test Scrape Cache
	scrapeRes := models.ScrapeResult{
		URL:         "https://example.com/test",
		Title:       "Test Page",
		Content:     "This is full text content for extraction verification.",
		Markdown:    "# Test Page\n\nThis is full text content.",
		WordCount:   8,
		Scraped:     true,
		StatusCode:  200,
		ContentType: "text/html",
	}
	SaveScrapeCache(scrapeRes)

	cachedScrape, foundScrape := GetScrapeCache("https://example.com/test")
	if !foundScrape {
		t.Fatalf("expected to find cached scrape result")
	}
	if cachedScrape.Title != "Test Page" || cachedScrape.WordCount != 8 {
		t.Fatalf("unexpected scrape cache result: %+v", cachedScrape)
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
