package main

import (
	"compress/flate"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
)

// ─── Stealth HTTP Client ─────────────────────────────────────────────────────

var httpClient = &http.Client{
	Timeout: 3 * time.Second, // fast-fail: blocked/slow pages fall back to snippets
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

var defaultHeaders = map[string]string{
	"User-Agent":                "SearqonBot/1.0 (+https://sooriya04.github.io/Searqon/)",
	"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
	"Accept-Language":           "en-US,en;q=0.9",
	"Cache-Control":             "no-cache",
	"Sec-Fetch-Dest":            "document",
	"Sec-Fetch-Mode":            "navigate",
	"Sec-Fetch-Site":            "none",
	"Sec-Fetch-User":            "?1",
	"Upgrade-Insecure-Requests": "1",
}

// ─── HTML Fetch Helper ───────────────────────────────────────────────────────

func fetchHTML(targetURL string, userAgent string) (string, *url.URL, int, string, error) {
	parsedURL, err := url.Parse(targetURL)
	if err != nil || parsedURL.Scheme == "" {
		return "", nil, 0, "", fmt.Errorf("invalid URL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return "", nil, 0, "", fmt.Errorf("request creation failed: %v", err)
	}
	for key, value := range defaultHeaders {
		req.Header.Set(key, value)
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", nil, 0, "", fmt.Errorf("fetch failed: %v", err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	contentTypeLower := strings.ToLower(contentType)
	if resp.StatusCode >= 400 {
		return "", nil, resp.StatusCode, contentType, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	if contentTypeLower != "" && !strings.Contains(contentTypeLower, "text/") &&
		!strings.Contains(contentTypeLower, "xml") && !strings.Contains(contentTypeLower, "json") {
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
	if err != nil {
		return "", nil, 0, contentType, fmt.Errorf("read failed: %v", err)
	}

	return string(body), parsedURL, resp.StatusCode, contentType, nil
}
