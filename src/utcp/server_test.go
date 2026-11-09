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

func TestResolveBaseURL_PortStripping(t *testing.T) {
	server := NewServer()

	// 1. https with default port :443 should strip :443
	req1 := httptest.NewRequest(http.MethodGet, "https://api.searqon.com:443/utcp", nil)
	req1.Header.Set("X-Forwarded-Proto", "https")
	req1.Header.Set("X-Forwarded-Host", "api.searqon.com:443")
	if base := server.ResolveBaseURL(req1); base != "https://api.searqon.com" {
		t.Errorf("expected 'https://api.searqon.com', got '%s'", base)
	}

	// 2. http with default port :80 should strip :80
	req2 := httptest.NewRequest(http.MethodGet, "http://api.searqon.com:80/utcp", nil)
	req2.Header.Set("X-Forwarded-Proto", "http")
	req2.Header.Set("X-Forwarded-Host", "api.searqon.com:80")
	if base := server.ResolveBaseURL(req2); base != "http://api.searqon.com" {
		t.Errorf("expected 'http://api.searqon.com', got '%s'", base)
	}

	// 3. https with offloaded internal port :80 in forwarded host should strip :80
	req3 := httptest.NewRequest(http.MethodGet, "http://api.searqon.com/utcp", nil)
	req3.Header.Set("X-Forwarded-Proto", "https")
	req3.Header.Set("X-Forwarded-Host", "api.searqon.com:80")
	if base := server.ResolveBaseURL(req3); base != "https://api.searqon.com" {
		t.Errorf("expected 'https://api.searqon.com', got '%s'", base)
	}

	// 4. Custom port :8443 or :7493 should NOT be stripped
	req4 := httptest.NewRequest(http.MethodGet, "https://api.searqon.com:8443/utcp", nil)
	req4.Header.Set("X-Forwarded-Proto", "https")
	req4.Header.Set("X-Forwarded-Host", "api.searqon.com:8443")
	if base := server.ResolveBaseURL(req4); base != "https://api.searqon.com:8443" {
		t.Errorf("expected 'https://api.searqon.com:8443', got '%s'", base)
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

func TestServerCacheCapAndEviction(t *testing.T) {
	server := NewServer()

	// Send requests for 30 distinct hosts (cap is 16)
	for i := 0; i < 30; i++ {
		req := httptest.NewRequest(http.MethodGet, "/utcp", nil)
		req.Header.Set("X-Forwarded-Host", strings.Repeat("a", 1)+"-host-"+string(rune('a'+i))+".example.com")
		rec := httptest.NewRecorder()
		server.ManualHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("request %d failed with code %d", i, rec.Code)
		}
	}

	server.mu.RLock()
	cacheLen := len(server.cachedManuals)
	orderLen := len(server.cacheOrder)
	server.mu.RUnlock()

	if cacheLen > maxCachedManuals {
		t.Errorf("cache size exceeded cap: got %d, max %d", cacheLen, maxCachedManuals)
	}
	if orderLen > maxCachedManuals {
		t.Errorf("cacheOrder size exceeded cap: got %d, max %d", orderLen, maxCachedManuals)
	}
}

func TestCrawlParameterParityInManual(t *testing.T) {
	manual := BuildManual("http://localhost:7493")

	var crawlTool *Tool
	for _, tool := range manual.Tools {
		if tool.Name == "searqon_crawl" {
			tCopy := tool
			crawlTool = &tCopy
			break
		}
	}

	if crawlTool == nil {
		t.Fatalf("searqon_crawl not found in manual")
	}

	if _, ok := crawlTool.Inputs.Properties["depth"]; !ok {
		t.Errorf("expected 'depth' property in searqon_crawl inputs, got properties: %+v", crawlTool.Inputs.Properties)
	}
	if _, ok := crawlTool.Inputs.Properties["max_depth"]; ok {
		t.Errorf("unexpected 'max_depth' property; should be standardized to 'depth'")
	}
}

func TestSearchCategoryPrunedFromManual(t *testing.T) {
	manual := BuildManual("http://localhost:7493")

	var searchTool *Tool
	for _, tool := range manual.Tools {
		if tool.Name == "searqon_search" {
			tCopy := tool
			searchTool = &tCopy
			break
		}
	}

	if searchTool == nil {
		t.Fatalf("searqon_search not found in manual")
	}

	if _, ok := searchTool.Inputs.Properties["category"]; ok {
		t.Errorf("unexpected 'category' parameter in searqon_search; unsupported fields should not be exposed")
	}
}

func TestToolEndpointContracts(t *testing.T) {
	manual := BuildManual("http://localhost:7493")

	expectedPaths := map[string]string{
		"searqon_search":        "/search",
		"searqon_scrape":        "/scrape",
		"searqon_research":      "/pipeline",
		"searqon_map":           "/map",
		"searqon_extract":       "/extract",
		"searqon_crawl":         "/crawl",
		"searqon_search_stream": "/search/stream",
	}

	for _, tool := range manual.Tools {
		expectedPath, ok := expectedPaths[tool.Name]
		if !ok {
			t.Errorf("unrecognized tool in manual: %s", tool.Name)
			continue
		}

		if !strings.Contains(tool.ToolCallTemplate.URL, expectedPath) {
			t.Errorf("tool %s URL %q does not match expected endpoint %q", tool.Name, tool.ToolCallTemplate.URL, expectedPath)
		}

		if tool.ToolCallTemplate.CallTemplateType == "http" {
			if tool.ToolCallTemplate.HTTPMethod != http.MethodPost {
				t.Errorf("expected POST method for HTTP tool %s, got %s", tool.Name, tool.ToolCallTemplate.HTTPMethod)
			}
			if tool.ToolCallTemplate.ContentType != "application/json" {
				t.Errorf("expected application/json content-type for tool %s, got %s", tool.Name, tool.ToolCallTemplate.ContentType)
			}
		} else if tool.ToolCallTemplate.CallTemplateType == "sse" {
			if !strings.Contains(tool.ToolCallTemplate.URL, "query={query}") {
				t.Errorf("expected sse query parameter template in %s URL, got %s", tool.Name, tool.ToolCallTemplate.URL)
			}
		}
	}
}
