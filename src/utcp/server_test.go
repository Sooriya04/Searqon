package utcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestUtcpManualStructure(t *testing.T) {
	baseURL := "https://api.searqon.ai"
	manual := BuildManual(baseURL)

	if manual.UtcpVersion != "1.0.1" {
		t.Fatalf("expected UtcpVersion 1.0.1, got %s", manual.UtcpVersion)
	}
	if manual.ManualVersion != "1.0.0" {
		t.Fatalf("expected ManualVersion 1.0.0, got %s", manual.ManualVersion)
	}

	expectedTools := map[string]string{
		"searqon_search":        "http",
		"searqon_scrape":        "http",
		"searqon_research":      "http",
		"searqon_map":           "http",
		"searqon_extract":       "http",
		"searqon_crawl":         "http",
		"searqon_search_stream": "sse",
	}

	if len(manual.Tools) != len(expectedTools) {
		t.Fatalf("expected %d tools, got %d", len(expectedTools), len(manual.Tools))
	}

	for _, tool := range manual.Tools {
		expectedType, ok := expectedTools[tool.Name]
		if !ok {
			t.Errorf("unexpected tool in manual: %s", tool.Name)
			continue
		}
		if tool.ToolCallTemplate.CallTemplateType != expectedType {
			t.Errorf("tool %s expected call_template_type %s, got %s", tool.Name, expectedType, tool.ToolCallTemplate.CallTemplateType)
		}
		if !strings.HasPrefix(tool.ToolCallTemplate.URL, baseURL) {
			t.Errorf("tool %s URL %s does not start with baseURL %s", tool.Name, tool.ToolCallTemplate.URL, baseURL)
		}
		if tool.Inputs.Type != "object" {
			t.Errorf("tool %s inputs type must be 'object', got %s", tool.Name, tool.Inputs.Type)
		}
		if len(tool.Inputs.Properties) == 0 {
			t.Errorf("tool %s inputs properties must not be empty", tool.Name)
		}
		if len(tool.Inputs.Required) == 0 {
			t.Errorf("tool %s required properties must not be empty", tool.Name)
		}
	}

	// Verify valid JSON marshaling
	data, err := json.Marshal(manual)
	if err != nil {
		t.Fatalf("failed to marshal UtcpManual to JSON: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal JSON into generic map: %v", err)
	}
	if parsed["utcp_version"] != "1.0.1" {
		t.Errorf("expected utcp_version 1.0.1 in parsed JSON, got %v", parsed["utcp_version"])
	}
}

func TestResolveBaseURL(t *testing.T) {
	server := NewServer()

	// 1. Default fallback from Host header
	req := httptest.NewRequest(http.MethodGet, "http://localhost:7493/utcp", nil)
	baseURL := server.ResolveBaseURL(req)
	if baseURL != "http://localhost:7493" {
		t.Errorf("expected 'http://localhost:7493', got '%s'", baseURL)
	}

	// 2. Reverse proxy headers (X-Forwarded-Proto & X-Forwarded-Host)
	req = httptest.NewRequest(http.MethodGet, "http://internal-service:8080/utcp", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "search.company.com")
	baseURL = server.ResolveBaseURL(req)
	if baseURL != "https://search.company.com" {
		t.Errorf("expected 'https://search.company.com', got '%s'", baseURL)
	}

	// 3. Multi-host X-Forwarded-Host header
	req.Header.Set("X-Forwarded-Host", "search.company.com, edge-proxy.internal")
	baseURL = server.ResolveBaseURL(req)
	if baseURL != "https://search.company.com" {
		t.Errorf("expected 'https://search.company.com', got '%s'", baseURL)
	}

	// 4. Explicit SEARQON_PUBLIC_URL environment variable override
	os.Setenv("SEARQON_PUBLIC_URL", "https://public.searqon.org/")
	defer os.Unsetenv("SEARQON_PUBLIC_URL")

	baseURL = server.ResolveBaseURL(req)
	if baseURL != "https://public.searqon.org" {
		t.Errorf("expected 'https://public.searqon.org', got '%s'", baseURL)
	}
}

func TestServerManualHandler(t *testing.T) {
	server := NewServer()

	// 1. Test OPTIONS Pre-flight request
	req := httptest.NewRequest(http.MethodOptions, "/utcp", nil)
	rec := httptest.NewRecorder()
	server.ManualHandler(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status %d for OPTIONS, got %d", http.StatusNoContent, rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("missing or invalid Access-Control-Allow-Origin")
	}
	if !strings.Contains(rec.Header().Get("Access-Control-Allow-Methods"), "GET") {
		t.Errorf("missing GET in Access-Control-Allow-Methods")
	}

	// 2. Test GET request
	req = httptest.NewRequest(http.MethodGet, "http://localhost:7493/utcp", nil)
	rec = httptest.NewRecorder()
	server.ManualHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d for GET, got %d", http.StatusOK, rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", rec.Header().Get("Content-Type"))
	}

	var manual UtcpManual
	if err := json.Unmarshal(rec.Body.Bytes(), &manual); err != nil {
		t.Fatalf("failed to decode manual response: %v", err)
	}
	if manual.UtcpVersion != "1.0.1" {
		t.Errorf("expected utcp_version 1.0.1, got %s", manual.UtcpVersion)
	}

	// 3. Test Unsupported Method
	req = httptest.NewRequest(http.MethodPost, "/utcp", nil)
	rec = httptest.NewRecorder()
	server.ManualHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d for POST, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestServerConcurrencyAndCaching(t *testing.T) {
	server := NewServer()
	var wg sync.WaitGroup
	concurrency := 20

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			proto := "http"
			if idx%2 == 0 {
				proto = "https"
			}
			host := "searqon.cluster.local"
			req := httptest.NewRequest(http.MethodGet, "/utcp", nil)
			req.Header.Set("X-Forwarded-Proto", proto)
			req.Header.Set("X-Forwarded-Host", host)
			rec := httptest.NewRecorder()
			server.ManualHandler(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("concurrent request %d failed with code %d", idx, rec.Code)
			}
		}(i)
	}

	wg.Wait()
}
