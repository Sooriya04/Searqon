package scraper

import (
	"testing"
	"time"
)

func TestScrapeProvenanceFields(t *testing.T) {
	html := "<html><head><title>Provenance Test</title></head><body><h1>Hello Provenance</h1><p>This is test content for cryptographic citation provenance and hash validation in Searqon engine.</p></body></html>"
	res := ScrapeHTMLContent(html, "https://example.com/prov", "https://example.com/prov", "markdown", time.Now())

	if !res.Scraped {
		t.Fatalf("Expected successful scrape, got error: %s", res.Error)
	}

	if res.ContentHash == "" {
		t.Errorf("Expected non-empty ContentHash (SHA-256), got empty")
	}

	if res.ScrapedAt == "" {
		t.Errorf("Expected non-empty ScrapedAt timestamp, got empty")
	} else {
		if _, err := time.Parse(time.RFC3339, res.ScrapedAt); err != nil {
			t.Errorf("Expected ScrapedAt to be RFC3339 format, got %s (err: %v)", res.ScrapedAt, err)
		}
	}
}
