package scraper

import (
	"fmt"
	"sync"
	"testing"
)

func TestBoundedRawHTMLCache_BasicAndEviction(t *testing.T) {
	// Create cache with capacity 3
	c := newBoundedRawHTMLCache(3)

	c.Store("https://example.com/1", "<html>1</html>")
	c.Store("https://example.com/2", "<html>2</html>")
	c.Store("https://example.com/3", "<html>3</html>")

	if c.Len() != 3 {
		t.Fatalf("expected len 3, got %d", c.Len())
	}

	// Verify all 3 are present
	if val, ok := c.Load("https://example.com/1"); !ok || val != "<html>1</html>" {
		t.Fatalf("expected <html>1</html>, got %q (found: %v)", val, ok)
	}

	// Adding a 4th entry should evict the least recently used
	// Since we just loaded /1, /2 is now the oldest
	c.Store("https://example.com/4", "<html>4</html>")

	if c.Len() != 3 {
		t.Fatalf("expected len 3, got %d", c.Len())
	}

	// /2 should have been evicted
	if _, ok := c.Load("https://example.com/2"); ok {
		t.Fatalf("expected https://example.com/2 to be evicted")
	}

	// /1, /3, /4 should be present
	for _, expectedURL := range []string{"https://example.com/1", "https://example.com/3", "https://example.com/4"} {
		if _, ok := c.Load(expectedURL); !ok {
			t.Fatalf("expected %s to be present", expectedURL)
		}
	}

	// Updating an existing key should update value without increasing len
	c.Store("https://example.com/1", "<html>1-updated</html>")
	if c.Len() != 3 {
		t.Fatalf("expected len 3, got %d", c.Len())
	}
	if val, ok := c.Load("https://example.com/1"); !ok || val != "<html>1-updated</html>" {
		t.Fatalf("expected updated value, got %q", val)
	}
}

func TestBoundedRawHTMLCache_Concurrency(t *testing.T) {
	c := newBoundedRawHTMLCache(16)
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(2)
		urlStr := fmt.Sprintf("https://example.com/%d", i%10)

		go func(u string) {
			defer wg.Done()
			c.Store(u, fmt.Sprintf("<html>%s</html>", u))
		}(urlStr)

		go func(u string) {
			defer wg.Done()
			c.Load(u)
		}(urlStr)
	}

	wg.Wait()
	if c.Len() > 16 {
		t.Fatalf("cache exceeded capacity 16: got %d", c.Len())
	}
}
