package utils

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"src/db"
)

func TestPersistentCookieJar_DecoupledWriteAndImmediateMemory(t *testing.T) {
	jar, err := NewPersistentCookieJar()
	if err != nil {
		t.Fatalf("failed to create persistent cookie jar: %v", err)
	}

	u, _ := url.Parse("https://example.com/test")
	testCookie := &http.Cookie{
		Name:    "session_id",
		Value:   "cookie_val_123",
		Path:    "/",
		Domain:  "example.com",
		Expires: time.Now().Add(24 * time.Hour),
	}

	// Calling SetCookies should return immediately without blocking
	start := time.Now()
	jar.SetCookies(u, []*http.Cookie{testCookie})
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Errorf("SetCookies took unexpectedly long (%v), synchronous DB write blocking detected", elapsed)
	}

	// Cookies should be immediately retrievable from memory without waiting for DB flush
	cookies := jar.Cookies(u)
	if len(cookies) == 0 {
		t.Fatalf("expected cookies to be available in-memory immediately, got 0")
	}
	if cookies[0].Name != "session_id" || cookies[0].Value != "cookie_val_123" {
		t.Errorf("cookie mismatch: %+v", cookies[0])
	}

	// Flush the background persistence worker
	FlushCookieJarPersistWorker()

	// Verify it reached database
	savedJSON, err := db.GetSessionCookies("example.com")
	if err == nil && savedJSON != "" {
		if savedJSON == "[]" {
			t.Errorf("saved cookies in DB is empty array")
		}
	}
}

func TestPersistentCookieJar_ConcurrentSetCookies(t *testing.T) {
	jar, err := NewPersistentCookieJar()
	if err != nil {
		t.Fatalf("failed to create persistent cookie jar: %v", err)
	}

	var wg sync.WaitGroup
	// Simulate 20 concurrent scraper goroutines setting cookies across different domains
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			domain := fmt.Sprintf("sub%d.example.org", idx%5)
			u, _ := url.Parse(fmt.Sprintf("https://%s/page", domain))
			c := &http.Cookie{
				Name:  fmt.Sprintf("cookie_%d", idx),
				Value: fmt.Sprintf("val_%d", idx),
			}
			jar.SetCookies(u, []*http.Cookie{c})
			_ = jar.Cookies(u)
		}(i)
	}

	wg.Wait()
	FlushCookieJarPersistWorker()
}
