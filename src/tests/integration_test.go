package tests

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"src/config"
	"src/db"
	"src/models"
	"src/scraper"
)

func TestFullDatabaseLifecycle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "searqon_db_lifecycle_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "lifecycle.db")

	cfg := config.DefaultConfig()
	cfg.Database.Enabled = true
	cfg.Database.Type = "sqlite"
	cfg.Database.SQLite.Path = dbPath
	cfg.Redis.Enabled = false

	db.InitDB()
	defer db.CloseDB()

	// 1. Verify Search Cache
	query := "machine learning models"
	results := []models.SearchResult{
		{
			Title:   "ML Architecture",
			URL:     "https://example.org/ml",
			Snippet: "Deep neural networks overview",
			Source:  "test_source",
		},
	}
	db.SaveSearchCache(query, results, "duckduckgo")

	cachedResults, provider, found := db.GetSearchCache(query)
	if !found {
		t.Fatalf("expected to find search cache for query '%s'", query)
	}
	if provider != "duckduckgo" {
		t.Fatalf("expected provider 'duckduckgo', got '%s'", provider)
	}
	if len(cachedResults) != 1 || cachedResults[0].Title != "ML Architecture" {
		t.Fatalf("unexpected cached search results: %+v", cachedResults)
	}

	// 2. Verify Scrape Cache
	scrapeRes := models.ScrapeResult{
		URL:         "https://example.org/ml",
		Title:       "ML Architecture",
		Content:     "Substantive machine learning content for testing full retrieval lifecycle.",
		Markdown:    "# ML Architecture\n\nSubstantive content.",
		WordCount:   8,
		Scraped:     true,
		StatusCode:  200,
		ContentType: "text/html",
	}
	db.SaveScrapeCache(scrapeRes)

	cachedScrape, foundScrape := db.GetScrapeCache("https://example.org/ml")
	if !foundScrape {
		t.Fatalf("expected to find cached scrape result")
	}
	if cachedScrape.Title != "ML Architecture" || cachedScrape.WordCount != 8 {
		t.Fatalf("unexpected cached scrape result: %+v", cachedScrape)
	}

	// 3. Verify Sessions
	sessionCookie := `[{"name":"token","value":"secret-session-abc"}]`
	if err := db.SaveSessionCookies("example.org", sessionCookie); err != nil {
		t.Fatalf("failed to save session cookies: %v", err)
	}
	loadedCookie, err := db.GetSessionCookies("example.org")
	if err != nil || loadedCookie != sessionCookie {
		t.Fatalf("session cookie mismatch: got %s, err: %v", loadedCookie, err)
	}

	// 4. Verify SQLite FTS5 Full-Text Search
	db.IndexScrapeInFTS("https://example.org/ml", "ML Architecture", "A description of ML", "Substantive machine learning content for testing full retrieval lifecycle.")
	time.Sleep(50 * time.Millisecond)

	ftsHits, err := db.SearchLocalIndex("machine learning", 5)
	if err != nil {
		t.Fatalf("failed FTS search: %v", err)
	}
	if len(ftsHits) == 0 {
		t.Fatalf("expected at least 1 FTS hit")
	}
}

func TestEscalationChainBypass(t *testing.T) {
	// Mock server that challenges Tier 1 (Cloudflare signature) and permits Tier 2 (Spoofed Headers)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		secCHUA := r.Header.Get("Sec-CH-UA")

		if secCHUA == "" || ua == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`<!DOCTYPE html><html><head><title>Just a moment...</title></head><body><div id="challenge-running">Checking your browser before accessing the site.</div></body></html>`))
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<!DOCTYPE html><html><head><title>Access Granted</title></head><body><article><h1>Unrestricted Intelligence</h1><p>The anti-bot escalation pipeline successfully cleared the Cloudflare challenge using modern browser persona spoofing.</p></article></body></html>`))
	}))
	defer ts.Close()

	opts := scraper.ScrapeOptions{
		Format:       "markdown",
		BypassCache:  true,
		StealthLevel: "auto",
	}

	result, raw, err := scraper.ExecuteSmartEscalationScrape(ts.URL, "Searqon/2.0", opts, time.Now())
	if err != nil {
		t.Fatalf("expected escalation scrape to succeed, got error: %v", err)
	}

	if result.StatusCode != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", result.StatusCode)
	}
	if result.EscalationTier != string(scraper.TierSpoofedHeaders) {
		t.Fatalf("expected Tier 2 (spoofed_headers), got: %s", result.EscalationTier)
	}
	if !result.BotDetected {
		t.Fatalf("expected BotDetected to be true")
	}
	if len(raw) == 0 {
		t.Fatalf("expected non-empty raw HTML payload")
	}
}
