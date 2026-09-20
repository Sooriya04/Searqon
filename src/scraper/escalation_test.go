package scraper

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDetectBotChallenge(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		body        string
		contentType string
		wantBlocked bool
		wantCategory string
	}{
		{
			name:        "Cloudflare Just a moment challenge",
			statusCode:  403,
			body:        `<html><head><title>Just a moment...</title></head><body><div id="cf-browser-verification">Verifying...</div></body></html>`,
			contentType: "text/html",
			wantBlocked: true,
			wantCategory: "cloudflare",
		},
		{
			name:        "Cloudflare 503 Under Attack",
			statusCode:  503,
			body:        `<html><body>Checking your browser before accessing cloudflare.com</body></html>`,
			contentType: "text/html",
			wantBlocked: true,
			wantCategory: "cloudflare",
		},
		{
			name:        "DataDome protection",
			statusCode:  403,
			body:        `<html><body><script src="https://geo.captcha-delivery.com/captcha/datadome.js"></script></body></html>`,
			contentType: "text/html",
			wantBlocked: true,
			wantCategory: "datadome",
		},
		{
			name:        "PerimeterX challenge",
			statusCode:  403,
			body:        `<html><body><div id="px-captcha"></div>Access to this page has been denied because we believe you are using automation tools</body></html>`,
			contentType: "text/html",
			wantBlocked: true,
			wantCategory: "perimeterx",
		},
		{
			name:        "Rate limit 429",
			statusCode:  429,
			body:        `Too Many Requests`,
			contentType: "text/plain",
			wantBlocked: true,
			wantCategory: "rate_limit",
		},
		{
			name:        "Empty JS gate requiring JavaScript",
			statusCode:  200,
			body:        `<html><body>Please enable JavaScript to view this page.</body></html>`,
			contentType: "text/html",
			wantBlocked: true,
			wantCategory: "js_gate",
		},
		{
			name:        "Normal content page",
			statusCode:  200,
			body:        `<html><head><title>Golang Concurrency</title></head><body><h1>Guide to Concurrency</h1><p>Goroutines are lightweight threads of execution managed by the Go runtime. They allow developers to write concurrent programs easily and efficiently with minimal overhead compared to standard operating system threads.</p></body></html>`,
			contentType: "text/html",
			wantBlocked: false,
			wantCategory: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := DetectBotChallenge(tt.statusCode, tt.body, tt.contentType)
			if res.IsBlocked != tt.wantBlocked {
				t.Errorf("DetectBotChallenge() isBlocked = %v, want %v (category: %s)", res.IsBlocked, tt.wantBlocked, res.Category)
			}
			if tt.wantCategory != "" && res.Category != tt.wantCategory {
				t.Errorf("DetectBotChallenge() category = %v, want %v", res.Category, tt.wantCategory)
			}
		})
	}
}

func TestBrowserPersonaConsistency(t *testing.T) {
	for _, p := range modernPersonas {
		if p.IsChromium {
			if p.SecChUa == "" {
				t.Errorf("Chromium persona %s missing SecChUa", p.Name)
			}
			if p.SecChUaPlatform == "" {
				t.Errorf("Chromium persona %s missing SecChUaPlatform", p.Name)
			}
		} else {
			// Non-Chromium (Safari/Firefox) must NOT send SecChUa
			if p.SecChUa != "" {
				t.Errorf("Non-Chromium persona %s should not have SecChUa", p.Name)
			}
		}

		headers := GetStealthSpoofedHeaders("https://example.com/test", &p)
		if headers["User-Agent"] != p.UserAgent {
			t.Errorf("Mismatch in User-Agent header for %s", p.Name)
		}
		if headers["Referer"] == "" {
			t.Errorf("Missing Referer in stealth headers for %s", p.Name)
		}
	}
}

func TestProxyPoolResolution(t *testing.T) {
	os.Setenv("ROTATING_PROXIES", "127.0.0.1:8001,http://127.0.0.1:8002")
	os.Setenv("RESIDENTIAL_PROXY_URL", "http://user:pass@127.0.0.1:9001")
	defer func() {
		os.Unsetenv("ROTATING_PROXIES")
		os.Unsetenv("RESIDENTIAL_PROXY_URL")
		InitProxyPool()
	}()

	InitProxyPool()

	if !HasProxies() {
		t.Fatal("Expected HasProxies() to be true")
	}
	if !HasResidentialProxies() {
		t.Fatal("Expected HasResidentialProxies() to be true")
	}

	resProxy, err := GetNextResidentialProxy()
	if err != nil || resProxy == nil {
		t.Fatalf("Failed to retrieve residential proxy: %v", err)
	}
	if !strings.Contains(resProxy.String(), "9001") {
		t.Errorf("Expected residential proxy port 9001, got %s", resProxy.String())
	}
}

func TestEscalationChainTier1ToTier2(t *testing.T) {
	// Mock server that blocks standard user agent with 403 Cloudflare challenge,
	// but serves valid content when stealth spoofed browser headers are presented.
	var requestCount int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)

		// If it's standard fast HTTP without stealth Sec-Fetch-Dest or Sec-Ch-Ua
		if r.Header.Get("Sec-Fetch-Dest") == "" || r.Header.Get("Sec-Fetch-Site") == "" {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`<html><head><title>Just a moment...</title></head><body><div id="cf-browser-verification">Checking your browser...</div></body></html>`))
			return
		}

		// Tier 2: Valid modern browser headers detected!
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<!DOCTYPE html><html><head><title>Stealth Passed</title></head><body><article><h1>Deep Intelligence Extraction</h1><p>The anti-bot escalation pipeline successfully bypassed the challenge page using realistic browser persona fingerprinting and headers. This ensures high reliability web scraping for modern search engines and AI agents across protected sites.</p></article></body></html>`))
	}))
	defer ts.Close()

	opts := ScrapeOptions{
		Format:       "markdown",
		BypassCache:  true,
		StealthLevel: "auto",
	}

	startTime := time.Now()
	result, raw, err := ExecuteSmartEscalationScrape(ts.URL, "", opts, startTime)
	if err != nil {
		t.Fatalf("Expected escalation scrape to succeed, got error: %v", err)
	}

	if result.EscalationTier != string(TierSpoofedHeaders) {
		t.Errorf("Expected EscalationTier = %s, got %s", TierSpoofedHeaders, result.EscalationTier)
	}
	if !result.BotDetected {
		t.Errorf("Expected BotDetected = true, got %v", result.BotDetected)
	}
	if !strings.Contains(result.Content, "anti-bot escalation pipeline") {
		t.Errorf("Expected extracted content to contain target text, got: %s", result.Content)
	}
	if len(raw) == 0 {
		t.Errorf("Expected non-empty raw HTML")
	}
}
