package scraper

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
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

func TestLightpandaScraper_CDP_InvalidEndpoint(t *testing.T) {
	startTime := time.Now()
	_, _, err := ScrapeWithLightpandaCDP("ws://127.0.0.1:59999", "https://example.com", "markdown", startTime, "")
	if err == nil {
		t.Fatalf("Expected error when connecting to unavailable CDP endpoint, got nil")
	}
}

func TestLightpandaScraper_CDP_RenderDynamicJS(t *testing.T) {
	enabled, lpPath := utils.LoadLightpandaConfig()
	if !enabled || lpPath == "" {
		t.Skip("Lightpanda binary not detected; skipping live CDP server test")
	}

	// Spin up local Lightpanda CDP server on test port 9231
	cmd := exec.Command(lpPath, "serve", "--host", "127.0.0.1", "--port", "9231")
	if err := cmd.Start(); err != nil {
		t.Skipf("Failed to start lightpanda serve subprocess: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// Wait for CDP server to open port
	serverReady := false
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		conn, err := net.DialTimeout("tcp", "127.0.0.1:9231", 100*time.Millisecond)
		if err == nil {
			conn.Close()
			serverReady = true
			break
		}
	}
	if !serverReady {
		t.Fatalf("Lightpanda CDP server failed to listen on 127.0.0.1:9231")
	}

	// Dynamic test server serving client-side JS DOM
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>CDP Dynamic Test</title></head>
<body>
    <div id="target">Waiting for client JS...</div>
    <script>
        document.getElementById('target').innerHTML = '<h2>Rendered via Lightpanda CDP WebSocket!</h2>';
    </script>
</body>
</html>`))
	}))
	defer ts.Close()

	startTime := time.Now()
	result, rawHTML, err := ScrapeWithLightpandaCDP("ws://127.0.0.1:9231", ts.URL, "markdown", startTime, "")
	if err != nil {
		t.Fatalf("ScrapeWithLightpandaCDP failed: %v", err)
	}

	if result.StatusCode != 200 {
		t.Errorf("Expected StatusCode 200, got %d", result.StatusCode)
	}
	if result.ExtractionMethod != "lightpanda_cdp" {
		t.Errorf("Expected ExtractionMethod 'lightpanda_cdp', got %s", result.ExtractionMethod)
	}
	if !strings.Contains(result.Content, "Rendered via Lightpanda CDP WebSocket!") {
		t.Errorf("Expected extracted markdown to contain JS-rendered text, got:\n%s", result.Content)
	}
	if !strings.Contains(rawHTML, "Rendered via Lightpanda CDP WebSocket!") {
		t.Errorf("Expected raw HTML to contain dynamic DOM elements")
	}
}
