package scraper

import (
	"compress/flate"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
	"src/utils"
)

// PreResolveDNS performs parallel DNS lookups for target hosts before batch scraping.
func PreResolveDNS(rawURLs []string) {
	var wg sync.WaitGroup
	seen := make(map[string]bool)
	for _, raw := range rawURLs {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Hostname() == "" || seen[parsed.Hostname()] {
			continue
		}
		seen[parsed.Hostname()] = true
		wg.Add(1)
		go func(host string) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
			defer cancel()
			_, _ = net.DefaultResolver.LookupIPAddr(ctx, host)
		}(parsed.Hostname())
	}
	wg.Wait()
}

var httpClient *http.Client

func init() {
	jar, _ := utils.NewPersistentCookieJar()
	httpClient = &http.Client{
		Timeout: 3 * time.Second,
		Jar:     jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	initBrowserTransport()
}

var defaultHeaders = map[string]string{
	"User-Agent":                "SearqonBot/1.0 (+https://sooriya04.github.io/Searqon/)",
	"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
	"Accept-Language":           "en-US,en;q=0.9",
	"Cache-Control":             "no-cache",
	"Pragma":                    "no-cache",
	"Sec-Fetch-Dest":            "document",
	"Sec-Fetch-Mode":            "navigate",
	"Sec-Fetch-Site":            "none",
	"Sec-Fetch-User":            "?1",
	"Upgrade-Insecure-Requests": "1",
}

// FetchHTML retrieves raw HTML from a target URL with a stealth user-agent.
func FetchHTML(targetURL string, userAgent string) (string, *url.URL, int, string, error) {
	parsedURL, err := url.Parse(targetURL)
	if err != nil || parsedURL.Scheme == "" {
		return "", nil, 0, "", fmt.Errorf("invalid URL")
	}

	maxAttempts := 3
	var lastErr error
	var contentType string

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(200*attempt) * time.Millisecond)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = WaitDomainRateLimit(ctx, parsedURL.Hostname())

		req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
		if err != nil {
			cancel()
			return "", nil, 0, "", fmt.Errorf("request creation failed: %v", err)
		}
		for key, value := range pickBrowserHeaders(userAgent) {
			req.Header.Set(key, value)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			cancel()
			lastErr = fmt.Errorf("fetch failed: %v", err)
			continue
		}

		contentType = resp.Header.Get("Content-Type")
		if resp.StatusCode >= 500 || resp.StatusCode == 429 {
			resp.Body.Close()
			cancel()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}

		if resp.StatusCode >= 400 {
			resp.Body.Close()
			cancel()
			return "", nil, resp.StatusCode, contentType, fmt.Errorf("HTTP %d", resp.StatusCode)
		}

		contentTypeLower := strings.ToLower(contentType)
		if contentTypeLower != "" && !strings.Contains(contentTypeLower, "text/") &&
			!strings.Contains(contentTypeLower, "xml") && !strings.Contains(contentTypeLower, "json") {
			resp.Body.Close()
			cancel()
			return "", nil, resp.StatusCode, contentType, fmt.Errorf("binary content-type: %s", contentType)
		}

		var reader io.Reader = resp.Body
		switch strings.ToLower(resp.Header.Get("Content-Encoding")) {
		case "gzip":
			gzReader, err := gzip.NewReader(resp.Body)
			if err == nil {
				defer gzReader.Close()
				reader = gzReader
			}
		case "br":
			reader = brotli.NewReader(resp.Body)
		case "deflate":
			reader = flate.NewReader(resp.Body)
		}

		body, err := io.ReadAll(io.LimitReader(reader, 5*1024*1024))
		resp.Body.Close()
		cancel()

		if err != nil {
			lastErr = fmt.Errorf("read failed: %v", err)
			continue
		}

		return string(body), parsedURL, resp.StatusCode, contentType, nil
	}

	return "", nil, 0, contentType, lastErr
}
