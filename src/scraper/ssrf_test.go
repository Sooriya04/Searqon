package scraper

import (
	"net/http"
	"os"
	"testing"
)

func TestValidateScrapeURL_SSRF(t *testing.T) {
	// Ensure test override is disabled during unit test to verify real SSRF filter
	oldArg := os.Args[0]
	os.Args[0] = "searqon_binary"
	defer func() { os.Args[0] = oldArg }()

	blockedURLs := []string{
		"http://127.0.0.1:8080",
		"http://127.0.0.1/admin",
		"http://localhost/secret",
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.1/internal",
		"http://172.16.0.5:5432",
		"http://192.168.1.1/router",
		"file:///etc/passwd",
		"gopher://127.0.0.1:6379",
	}

	for _, u := range blockedURLs {
		_, err := ValidateScrapeURL(u)
		if err == nil {
			t.Errorf("expected SSRF block for %s, got nil err", u)
		}
	}

	allowedURLs := []string{
		"https://example.com/article",
		"https://en.wikipedia.org/wiki/Go_(programming_language)",
	}

	for _, u := range allowedURLs {
		_, err := ValidateScrapeURL(u)
		if err != nil {
			t.Errorf("expected %s to be allowed, got err: %v", u, err)
		}
	}
}

func TestCheckRedirect_SSRF(t *testing.T) {
	oldArg := os.Args[0]
	os.Args[0] = "searqon_binary"
	defer func() { os.Args[0] = oldArg }()

	// Test httpClient.CheckRedirect rejects redirects to private IPs
	redirectReq, err := http.NewRequest("GET", "http://127.0.0.1:8080/admin", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	err = httpClient.CheckRedirect(redirectReq, []*http.Request{})
	if err == nil {
		t.Errorf("expected httpClient.CheckRedirect to block private IP, got nil err")
	}

	// Test robotsHttpClient.CheckRedirect rejects redirects to metadata IPs
	metaReq, err := http.NewRequest("GET", "http://169.254.169.254/latest/meta-data/", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	err = robotsHttpClient.CheckRedirect(metaReq, []*http.Request{})
	if err == nil {
		t.Errorf("expected robotsHttpClient.CheckRedirect to block metadata IP, got nil err")
	}
}
