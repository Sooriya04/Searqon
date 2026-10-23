package crawler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"src/models"
)

func TestMapSiteURLs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Sitemap Test Page</title></head>
<body>
    <nav>
        <a href="/about">About Us</a>
        <a href="/pricing">Pricing Plans</a>
        <a href="/docs/api">API Reference</a>
        <a href="https://external-competitor.com/link">External Link</a>
        <a href="#jump-target">Anchor Link</a>
        <a href="">Empty Link</a>
    </nav>
    <main>
        <h1>Welcome</h1>
    </main>
</body>
</html>`))
	}))
	defer ts.Close()

	// Test mapping internal URLs with a limit
	result := MapSiteURLs(ts.URL, 10)
	if result.Error != "" {
		t.Fatalf("MapSiteURLs returned unexpected error: %s", result.Error)
	}

	if result.SourceURL != ts.URL {
		t.Errorf("Expected SourceURL %s, got %s", ts.URL, result.SourceURL)
	}

	if len(result.Links) < 3 {
		t.Errorf("Expected at least 3 internal links, got %d", len(result.Links))
	}

	for _, link := range result.Links {
		if strings.Contains(link.URL, "external-competitor.com") {
			t.Errorf("External link should have been filtered out: %s", link.URL)
		}
		if strings.Contains(link.URL, "#jump-target") {
			t.Errorf("Fragment should have been stripped: %s", link.URL)
		}
	}

	// Verify limit behavior
	limitedResult := MapSiteURLs(ts.URL, 1)
	if len(limitedResult.Links) > 1 {
		t.Errorf("Expected at most 1 link when limit=1, got %d", len(limitedResult.Links))
	}
}

func TestMapSiteURLs_InvalidURL(t *testing.T) {
	result := MapSiteURLs("http://invalid-non-existent-domain-1234567.xyz", 5)
	if result.Error == "" {
		t.Errorf("Expected error for unreachable domain, got none")
	}
}

func TestCrawlSite(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<html><body><h1>Home</h1><a href="/page1">Page 1</a></body></html>`)
		case "/page1":
			fmt.Fprint(w, `<html><body><h1>Page 1</h1><a href="/page2">Page 2</a></body></html>`)
		case "/page2":
			fmt.Fprint(w, `<html><body><h1>Page 2</h1>Content here</body></html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	var streamedPages int
	res := CrawlSite(ts.URL, 5, 2, "markdown", func(page models.ScrapeResult) {
		streamedPages++
	})

	if res.Error != "" {
		t.Fatalf("CrawlSite failed: %s", res.Error)
	}

	if res.Total == 0 {
		t.Errorf("Expected at least 1 crawled page, got 0")
	}

	if streamedPages == 0 {
		t.Errorf("Expected callback invocation for streamed pages")
	}
}
