package scraper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"src/utils"
)

func TestLightpandaScraper_RenderDynamicJS(t *testing.T) {
	enabled, lpPath := utils.LoadLightpandaConfig()
	if !enabled || lpPath == "" {
		t.Skip("Lightpanda binary not detected in default paths or disabled; skipping live Lightpanda execution test")
	}

	// Dynamic test server serving HTML with client-side JavaScript execution
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>Lightpanda Headless Test</title>
</head>
<body>
    <header>
        <h1>Searqon Intelligence</h1>
    </header>
    <main>
        <div id="static-content">Initial SSR payload.</div>
        <div id="dynamic-target">Loading...</div>
    </main>
    <script>
        document.getElementById('dynamic-target').innerHTML = '<article><p>Dynamically evaluated with Lightpanda JavaScript Engine!</p></article>';
    </script>
</body>
</html>`))
	}))
	defer ts.Close()

	startTime := time.Now()
	result, rawHTML, err := ScrapeWithLightpanda(ts.URL, "SearqonBot/1.0", lpPath, "markdown", startTime, "")
	if err != nil {
		t.Fatalf("ScrapeWithLightpanda failed: %v", err)
	}

	// Verify status & extraction metadata
	if result.StatusCode != 200 {
		t.Errorf("Expected StatusCode 200, got %d", result.StatusCode)
	}
	if result.ExtractionMethod != "lightpanda" {
		t.Errorf("Expected ExtractionMethod 'lightpanda', got %s", result.ExtractionMethod)
	}
	if result.RenderMethod != "lightpanda" {
		t.Errorf("Expected RenderMethod 'lightpanda', got %s", result.RenderMethod)
	}

	// Verify JS execution rendered dynamic DOM nodes
	if !strings.Contains(result.Content, "Dynamically evaluated with Lightpanda JavaScript Engine!") {
		t.Errorf("Expected extracted content to contain JS-rendered text, got:\n%s", result.Content)
	}

	if !strings.Contains(rawHTML, "Dynamically evaluated with Lightpanda JavaScript Engine!") {
		t.Errorf("Expected raw HTML to contain JS-rendered DOM elements")
	}

	if result.Duration <= 0 {
		t.Errorf("Expected positive execution duration, got %dms", result.Duration)
	}
}

func TestLightpandaScraper_InvalidBinaryPath(t *testing.T) {
	startTime := time.Now()
	_, _, err := ScrapeWithLightpanda("https://example.com", "", "/non/existent/path/to/lightpanda", "markdown", startTime, "")
	if err == nil {
		t.Fatalf("Expected error when using invalid lightpanda binary path, got nil")
	}
}
