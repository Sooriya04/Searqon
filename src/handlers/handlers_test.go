package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"src/db"
	"src/models"
)

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	HealthHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rr.Code)
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Status    string   `json:"status"`
			Endpoints []string `json:"endpoints"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if !resp.Success || resp.Data.Status != "ok" {
		t.Errorf("Unexpected health response: %+v", resp)
	}
	if len(resp.Data.Endpoints) == 0 {
		t.Errorf("Expected non-empty endpoints list")
	}
}

func TestOpenAPIHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	rr := httptest.NewRecorder()

	OpenAPIHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected application/json, got %s", contentType)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "openapi") {
		t.Errorf("Expected response to contain openapi definition")
	}
}

func TestMetricsHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()

	MetricsHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "searqon_") && !strings.Contains(body, "http_requests_total") {
		t.Errorf("Expected Prometheus metrics in response, got:\n%s", body)
	}
}

func TestMapHandler_Validation(t *testing.T) {
	// 1. GET should be rejected (only POST allowed)
	req := httptest.NewRequest(http.MethodGet, "/map", nil)
	rr := httptest.NewRecorder()
	MapHandler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 Method Not Allowed, got %d", rr.Code)
	}

	// 2. Empty body or missing URL should return 400 Bad Request
	req2 := httptest.NewRequest(http.MethodPost, "/map", bytes.NewBufferString(`{"url":""}`))
	rr2 := httptest.NewRecorder()
	MapHandler(rr2, req2)
	if rr2.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for empty URL, got %d", rr2.Code)
	}
}

func TestExtractHandler(t *testing.T) {
	// 1. Method validation
	req := httptest.NewRequest(http.MethodGet, "/extract", nil)
	rr := httptest.NewRecorder()
	ExtractHandler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 Method Not Allowed, got %d", rr.Code)
	}

	// 2. Missing params validation
	req2 := httptest.NewRequest(http.MethodPost, "/extract", bytes.NewBufferString(`{"url":""}`))
	rr2 := httptest.NewRecorder()
	ExtractHandler(rr2, req2)
	if rr2.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", rr2.Code)
	}

	// 3. Live extraction against test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>Extraction Test Page</title></head><body>
			<article>
				<h1 class="headline">Searqon Engine v2</h1>
				<span class="price">$0.00 Open Source</span>
				<p>This is a complete comprehensive test article containing enough descriptive words to satisfy the content verification threshold required by the web intelligence engine for valid scraping extraction.</p>
			</article>
		</body></html>`)
	}))
	defer ts.Close()

	payload, _ := json.Marshal(ExtractRequest{
		URL: ts.URL,
		Selectors: map[string]string{
			"title": ".headline",
			"cost":  ".price",
		},
		BypassCache: true,
	})

	req3 := httptest.NewRequest(http.MethodPost, "/extract", bytes.NewBuffer(payload))
	rr3 := httptest.NewRecorder()
	ExtractHandler(rr3, req3)

	if rr3.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", rr3.Code, rr3.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			URL  string            `json:"url"`
			Data map[string]string `json:"data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rr3.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Data.Data["title"] != "Searqon Engine v2" {
		t.Errorf("Expected title 'Searqon Engine v2', got '%s'", resp.Data.Data["title"])
	}
	if resp.Data.Data["cost"] != "$0.00 Open Source" {
		t.Errorf("Expected cost '$0.00 Open Source', got '%s'", resp.Data.Data["cost"])
	}

	// 4. Test cached extraction (Finding 5: CSS Selectors on Cached Content)
	cachedPayload, _ := json.Marshal(ExtractRequest{
		URL: ts.URL,
		Selectors: map[string]string{
			"title": ".headline",
			"cost":  ".price",
		},
		BypassCache: false,
	})
	req4 := httptest.NewRequest(http.MethodPost, "/extract", bytes.NewBuffer(cachedPayload))
	rr4 := httptest.NewRecorder()
	ExtractHandler(rr4, req4)

	if rr4.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for cached extraction, got %d: %s", rr4.Code, rr4.Body.String())
	}

	var cachedResp struct {
		Success bool `json:"success"`
		Data    struct {
			URL  string            `json:"url"`
			Data map[string]string `json:"data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rr4.Body).Decode(&cachedResp); err != nil {
		t.Fatalf("Failed to decode cached response: %v", err)
	}

	if cachedResp.Data.Data["title"] != "Searqon Engine v2" {
		t.Errorf("Expected cached title 'Searqon Engine v2', got '%s'", cachedResp.Data.Data["title"])
	}
	if cachedResp.Data.Data["cost"] != "$0.00 Open Source" {
		t.Errorf("Expected cached cost '$0.00 Open Source', got '%s'", cachedResp.Data.Data["cost"])
	}

	// 5. Test extraction with CSS selectors when URL was cached without raw HTML in memory
	cachedWithoutHTML := ts.URL + "/cached-only-markdown"
	db.SaveScrapeCache(models.ScrapeResult{
		URL:      cachedWithoutHTML,
		Title:    "Old Title",
		Markdown: "# Old Title\n\nOnly markdown content without HTML tags.",
		Content:  "Only markdown content without HTML tags.",
		Scraped:  true,
	})

	cachedWithoutHTMLPayload, _ := json.Marshal(ExtractRequest{
		URL: cachedWithoutHTML,
		Selectors: map[string]string{
			"title": ".headline",
		},
		BypassCache: false,
	})
	req5 := httptest.NewRequest(http.MethodPost, "/extract", bytes.NewBuffer(cachedWithoutHTMLPayload))
	rr5 := httptest.NewRecorder()
	ExtractHandler(rr5, req5)

	if rr5.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for cached extraction without raw HTML, got %d: %s", rr5.Code, rr5.Body.String())
	}

	var cachedWithoutHTMLResp struct {
		Success bool `json:"success"`
		Data    struct {
			Data map[string]string `json:"data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rr5.Body).Decode(&cachedWithoutHTMLResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if cachedWithoutHTMLResp.Data.Data["title"] != "Searqon Engine v2" {
		t.Errorf("Expected title 'Searqon Engine v2' after cache bypass for missing raw HTML, got '%s'", cachedWithoutHTMLResp.Data.Data["title"])
	}
}

func TestScrapeHandler_Validation(t *testing.T) {
	// GET not allowed
	req := httptest.NewRequest(http.MethodGet, "/scrape", nil)
	rr := httptest.NewRecorder()
	ScrapeHandler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 Method Not Allowed, got %d", rr.Code)
	}

	// Missing URL
	req2 := httptest.NewRequest(http.MethodPost, "/scrape", bytes.NewBufferString(`{"url":""}`))
	rr2 := httptest.NewRecorder()
	ScrapeHandler(rr2, req2)
	if rr2.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", rr2.Code)
	}
}

func TestSearchHandler_Validation(t *testing.T) {
	// PUT method should be rejected (only GET or POST allowed)
	req := httptest.NewRequest(http.MethodPut, "/search", nil)
	rr := httptest.NewRecorder()
	SearchHandler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 Method Not Allowed, got %d", rr.Code)
	}

	// Missing Query on POST
	req2 := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBufferString(`{"query":""}`))
	rr2 := httptest.NewRecorder()
	SearchHandler(rr2, req2)
	if rr2.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", rr2.Code)
	}

	// Missing Query on GET
	req3 := httptest.NewRequest(http.MethodGet, "/search", nil)
	rr3 := httptest.NewRecorder()
	SearchHandler(rr3, req3)
	if rr3.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for GET without ?q=, got %d", rr3.Code)
	}
}

func TestPipelineHandler_Validation(t *testing.T) {
	// GET not allowed
	req := httptest.NewRequest(http.MethodGet, "/pipeline", nil)
	rr := httptest.NewRecorder()
	PipelineHandler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 Method Not Allowed, got %d", rr.Code)
	}

	// Missing Query
	req2 := httptest.NewRequest(http.MethodPost, "/pipeline", bytes.NewBufferString(`{"query":""}`))
	rr2 := httptest.NewRecorder()
	PipelineHandler(rr2, req2)
	if rr2.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", rr2.Code)
	}
}

func TestScrapeHandler_LiveExecution(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>Handler Scrape Test</title></head><body><article><h1>Scraped Content</h1><p>Testing Searqon HTTP handler end-to-end markdown extraction pipeline with full paragraphs and detailed explanations to ensure content word counts exceed minimum safety thresholds.</p></article></body></html>`)
	}))
	defer ts.Close()

	payload, _ := json.Marshal(models.ScrapeRequest{
		URL:          ts.URL,
		Format:       "markdown",
		BypassCache:  true,
		StealthLevel: "auto",
	})

	req := httptest.NewRequest(http.MethodPost, "/scrape", bytes.NewBuffer(payload))
	rr := httptest.NewRecorder()
	ScrapeHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Success bool                `json:"success"`
		Data    models.ScrapeResult `json:"data"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("Expected success=true")
	}
	if !strings.Contains(resp.Data.Markdown, "Testing Searqon HTTP handler") && !strings.Contains(resp.Data.Content, "Testing Searqon HTTP handler") {
		t.Errorf("Extracted content missing expected text: %s", resp.Data.Markdown)
	}
}

func TestCrawlHandler_ParameterParity(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<html><body><h1>Root</h1><a href="%s/sub">Sub</a></body></html>`, ts.URL)
	}))
	defer ts.Close()

	// 1. Test using 'depth'
	body1 := fmt.Sprintf(`{"url": "%s", "depth": 1, "limit": 2}`, ts.URL)
	req1 := httptest.NewRequest(http.MethodPost, "/crawl", bytes.NewBufferString(body1))
	rr1 := httptest.NewRecorder()
	CrawlHandler(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for crawl with depth, got %d: %s", rr1.Code, rr1.Body.String())
	}

	// 2. Test using 'max_depth' alias
	body2 := fmt.Sprintf(`{"url": "%s", "max_depth": 1, "limit": 2}`, ts.URL)
	req2 := httptest.NewRequest(http.MethodPost, "/crawl", bytes.NewBufferString(body2))
	rr2 := httptest.NewRecorder()
	CrawlHandler(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for crawl with max_depth, got %d: %s", rr2.Code, rr2.Body.String())
	}
}

func TestBatchScrapeHandler_MaxURLLimit(t *testing.T) {
	urls := make([]string, MaxBatchURLs+1)
	for i := 0; i < len(urls); i++ {
		urls[i] = fmt.Sprintf("https://example.com/%d", i)
	}

	payload, err := json.Marshal(models.BatchScrapeRequest{
		URLs: urls,
	})
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/scrape/batch", bytes.NewBuffer(payload))
	rr := httptest.NewRecorder()
	BatchScrapeHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 Bad Request for batch scrape exceeding max URL limit, got %d: %s", rr.Code, rr.Body.String())
	}

	if !strings.Contains(rr.Body.String(), "exceeds maximum limit of 50 URLs") {
		t.Errorf("Expected error message to mention maximum limit, got: %s", rr.Body.String())
	}
}

func TestSearchHandler_TokenLimitForwarding(t *testing.T) {
	scrapeFalse := false
	payload, err := json.Marshal(models.SearchRequest{
		Query:      "test token forwarding",
		Limit:      3,
		Scrape:     &scrapeFalse,
		MaxTokens:  100,
		TokenLimit: 100,
	})
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBuffer(payload))
	rr := httptest.NewRecorder()
	SearchHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSearchHandler_SearXNGURLForwarding(t *testing.T) {
	scrapeFalse := false
	payload, err := json.Marshal(models.SearchRequest{
		Query:       "test searxng forwarding",
		Limit:       2,
		Scrape:      &scrapeFalse,
		SearXNGURL:  "http://custom-searxng.internal:8080",
	})
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBuffer(payload))
	req.Header.Set("X-Searxng-URL", "http://custom-searxng.internal:8080")
	rr := httptest.NewRecorder()
	SearchHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestPipelineHandler_SearXNGURLForwarding(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>Handler Pipeline Test</title></head><body><article><h1>Scraped Content</h1><p>Testing Searqon HTTP handler end-to-end markdown extraction pipeline with full paragraphs and detailed explanations.</p></article></body></html>`)
	}))
	defer ts.Close()

	query := "test pipeline cached query"
	db.SaveSearchCache(query, []models.SearchResult{
		{URL: ts.URL, Title: "Handler Pipeline Test"},
	}, "searxng")

	payload, err := json.Marshal(models.PipelineRequest{
		Query:       query,
		MaxSources:  1,
		SearXNGURL:  "http://custom-searxng.internal:8080",
	})
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/pipeline", bytes.NewBuffer(payload))
	req.Header.Set("X-Searxng-URL", "http://custom-searxng.internal:8080")
	rr := httptest.NewRecorder()
	PipelineHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}
}
