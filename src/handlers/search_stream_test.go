package handlers

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"src/db"
	"src/models"
)

func TestSearchStreamHandler(t *testing.T) {
	// Seed search cache for deterministic, instant test execution without live network flakes
	cachedResults := []models.SearchResult{
		{
			Title:   "Go Testing Guide",
			URL:     "https://golang.org/doc/testing",
			Snippet: "Testing in Go is simple and fast.",
			Content: "Go has a built-in testing framework with go test.",
			Scraped: true,
		},
	}
	db.SaveSearchCache("golang-test-stream", cachedResults, "multi")

	req := httptest.NewRequest(http.MethodGet, "/search/stream?q=golang-test-stream", nil)
	w := httptest.NewRecorder()

	SearchStreamHandler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		t.Fatalf("Expected text/event-stream, got %s", contentType)
	}

	scanner := bufio.NewScanner(w.Body)
	eventsFound := make(map[string]bool)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event:") {
			evt := strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			eventsFound[evt] = true
		}
	}

	if !eventsFound["discovery"] {
		t.Errorf("Expected 'discovery' event in SSE stream, found: %+v", eventsFound)
	}
	if !eventsFound["result"] {
		t.Errorf("Expected 'result' event in SSE stream, found: %+v", eventsFound)
	}
	if !eventsFound["done"] {
		t.Errorf("Expected 'done' event in SSE stream, found: %+v", eventsFound)
	}
}
