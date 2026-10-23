package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
