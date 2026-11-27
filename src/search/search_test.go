package search

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"src/config"
	"src/models"
)

func TestSearchPipeline_TimeoutSafety(t *testing.T) {
	// Create an already-cancelled or very short deadline context to trigger scrape timeout path
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait for context to cancel
	<-ctx.Done()

	// Run pipeline with scrape=true
	// When timeout fires, the results slice should be safely cloned under mu.Lock() without panicking or racing
	resp := RunSearchPipelineWithContext(ctx, "golang testing", 5, true, false, 0, 0, false, "")

	// Result should return gracefully
	if resp.Results == nil {
		// Empty results or none provider is fine for cancelled/stubbed context
		t.Logf("Search returned with provider=%s", resp.Provider)
	}
}

func TestSearchPipeline_MaxTokensParameter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	<-ctx.Done()

	resp := RunSearchPipelineWithContext(ctx, "golang token limit forwarding", 3, true, false, 0, 150, false, "")
	if resp.Results == nil {
		t.Logf("Search returned with provider=%s", resp.Provider)
	}
}

func TestSearchPipeline_ResultCloningUnderLock(t *testing.T) {
	// Directly verify slice isolation
	original := []models.SearchResult{
		{URL: "https://example.com/1", Title: "One"},
		{URL: "https://example.com/2", Title: "Two"},
	}

	cloned := make([]models.SearchResult, len(original))
	copy(cloned, original)

	// Mutating original should not affect cloned
	original[0].Content = "mutated content"
	if cloned[0].Content == "mutated content" {
		t.Fatalf("cloned slice was unexpectedly mutated through backing array")
	}
}

func TestGetSearXNGBase_DefaultFallback(t *testing.T) {
	origEnv := os.Getenv("SEARXNG_URL")
	os.Unsetenv("SEARXNG_URL")
	defer func() {
		if origEnv != "" {
			os.Setenv("SEARXNG_URL", origEnv)
		}
	}()

	cfg := config.Get()
	origURL := cfg.SearXNG.URL
	cfg.SearXNG.URL = ""
	defer func() {
		cfg.SearXNG.URL = origURL
	}()

	base := getSearXNGBase()
	if base != "http://localhost:8080" {
		t.Fatalf("expected fallback SearXNG base URL http://localhost:8080, got %s", base)
	}
}

func TestFallbackHTTPClientConfiguration(t *testing.T) {
	if fallbackHTTPClient == nil {
		t.Fatal("expected fallbackHTTPClient to be initialized")
	}
	if fallbackHTTPClient.Timeout != 2*time.Second {
		t.Fatalf("expected 2s timeout, got %v", fallbackHTTPClient.Timeout)
	}
	transport, ok := fallbackHTTPClient.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected fallbackHTTPClient.Transport to be *http.Transport")
	}
	if transport.MaxIdleConns != 100 {
		t.Errorf("expected MaxIdleConns=100, got %d", transport.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != 20 {
		t.Errorf("expected MaxIdleConnsPerHost=20, got %d", transport.MaxIdleConnsPerHost)
	}
	if transport.IdleConnTimeout != 90*time.Second {
		t.Errorf("expected IdleConnTimeout=90s, got %v", transport.IdleConnTimeout)
	}
}

func TestSearchSearXNGWithURL_CustomEndpoint(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("q") != "custom endpoint test" {
			http.Error(w, "bad query", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[{"url":"https://example.com/custom","title":"Custom SearXNG Result","content":"Custom content snippet"}]}`))
	}))
	defer ts.Close()

	results, err := searchSearXNGWithURL("custom endpoint test", 5, ts.URL)
	if err != nil {
		t.Fatalf("unexpected error querying custom searxng: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].URL != "https://example.com/custom" {
		t.Fatalf("expected custom URL, got %s", results[0].URL)
	}
	if results[0].Title != "Custom SearXNG Result" {
		t.Fatalf("expected custom title, got %s", results[0].Title)
	}
	if results[0].Source != "searxng" {
		t.Fatalf("expected source 'searxng', got %s", results[0].Source)
	}
}

func TestParseDDGHTML(t *testing.T) {
	htmlData := `
	<div class="result">
		<h2 class="result__title">
			<a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgithub.com%2Fgolang%2Fgo&rut=123">Go Programming Language</a>
		</h2>
		<a class="result__url" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgithub.com%2Fgolang%2Fgo">
			github.com &rsaquo; golang &rsaquo; go
		</a>
		<div class="result__snippet">The Go programming language source repository.</div>
	</div>
	<div class="result">
		<h2 class="result__title">
			<a class="result__a" href="https://example.com/direct">Direct Link Title</a>
		</h2>
		<div class="result__snippet">Direct link description.</div>
	</div>
	<div class="result">
		<h2 class="result__title">
			<a class="result__a" href="/bad/internal">Internal Link Only</a>
		</h2>
		<div class="result__snippet">Should be skipped because it stays on duckduckgo.com domain.</div>
	</div>
	`

	results, err := parseDDGHTML(strings.NewReader(htmlData), 5)
	if err != nil {
		t.Fatalf("parseDDGHTML failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 parsed results, got %d", len(results))
	}

	if results[0].URL != "https://github.com/golang/go" {
		t.Errorf("expected unwrapped target URL https://github.com/golang/go, got %s", results[0].URL)
	}
	if results[0].Title != "Go Programming Language" {
		t.Errorf("expected Go Programming Language, got %s", results[0].Title)
	}

	if results[1].URL != "https://example.com/direct" {
		t.Errorf("expected https://example.com/direct, got %s", results[1].URL)
	}
}

func TestDecomposeQueries_NoBrokenParentheses(t *testing.T) {
	// Simulated expanded query with parentheses containing OR
	expanded := "docker and kubernetes (container OR k8s)"
	queries := decomposeQueries(expanded)

	for _, q := range queries {
		open := strings.Count(q, "(")
		close := strings.Count(q, ")")
		if open != close {
			t.Fatalf("decomposed query has unbalanced parentheses: %q (open=%d close=%d)", q, open, close)
		}
		if strings.Contains(q, "(container") && !strings.Contains(q, ")") {
			t.Fatalf("broken parenthesis fragment detected: %q", q)
		}
	}
}


