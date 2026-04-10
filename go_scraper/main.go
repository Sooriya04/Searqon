package main

import (
	"compress/flate"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/brotli"
	readability "github.com/go-shiori/go-readability"
)

// ─── Request / Response Models ───────────────────────────────────────────────

type ScrapeRequest struct {
	URL    string `json:"url"`
	Format string `json:"format"` // "markdown" (default) | "text"
}

type BatchScrapeRequest struct {
	URLs   []string `json:"urls"`
	Format string   `json:"format"`
}

type MapRequest struct {
	URL   string `json:"url"`
	Limit int    `json:"limit"`
}

type CrawlRequest struct {
	URL    string `json:"url"`
	Limit  int    `json:"limit"` // max pages, default 30
	Depth  int    `json:"depth"` // link depth, default 2
	Format string `json:"format"`
}

type ScrapeResult struct {
	Title     string `json:"title"`
	Content   string `json:"content"`            // plain text (always present)
	Markdown  string `json:"markdown,omitempty"` // only when format=markdown
	URL       string `json:"url"`
	WordCount int    `json:"wordCount"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Duration  int64  `json:"duration"`
	Error     string `json:"error,omitempty"`
}

type MapLink struct {
	URL         string `json:"url"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

type MapResult struct {
	SourceURL string    `json:"sourceUrl"`
	Links     []MapLink `json:"links"`
	Count     int       `json:"count"`
	Duration  int64     `json:"duration"`
}

type CrawlResult struct {
	SourceURL string         `json:"sourceUrl"`
	Pages     []ScrapeResult `json:"pages"`
	Total     int            `json:"total"`
	Duration  int64          `json:"duration"`
	Error     string         `json:"error,omitempty"`
}

// ─── Noise Removal Selectors ─────────────────────────────────────────────────

var noiseSelectors = []string{
	"script", "style", "noscript", "iframe", "svg",
	"nav", "header", "footer", "aside",
	"form", "button", "input", "select", "textarea",
	".ad", ".ads", ".advert", ".advertisement",
	".sidebar", ".side-bar",
	".nav", ".navbar", ".navigation", ".menu",
	".footer", ".header",
	".cookie", ".cookie-banner", ".cookie-notice",
	".popup", ".modal", ".overlay",
	".social", ".share", ".sharing",
	".comment", ".comments",
	".related", ".recommended",
	".breadcrumb", ".breadcrumbs",
	"[role='navigation']", "[role='banner']", "[role='contentinfo']",
	"[role='complementary']", "[aria-hidden='true']",
}

// ─── Stealth HTTP Client ─────────────────────────────────────────────────────

var httpClient = &http.Client{
	Timeout: 8 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

var defaultHeaders = map[string]string{
	"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
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

func fetchHTML(targetURL string) (string, *url.URL, int, error) {
	parsedURL, err := url.Parse(targetURL)
	if err != nil || parsedURL.Scheme == "" {
		return "", nil, 0, fmt.Errorf("invalid URL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return "", nil, 0, fmt.Errorf("request creation failed: %v", err)
	}
	for key, value := range defaultHeaders {
		req.Header.Set(key, value)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", nil, 0, fmt.Errorf("fetch failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", nil, resp.StatusCode, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if contentType != "" && !strings.Contains(contentType, "text/") &&
		!strings.Contains(contentType, "xml") && !strings.Contains(contentType, "json") {
		return "", nil, resp.StatusCode, fmt.Errorf("binary content-type: %s", contentType)
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
		return "", nil, 0, fmt.Errorf("read failed: %v", err)
	}

	return string(body), parsedURL, resp.StatusCode, nil
}

// ─── Core Scraping Function ──────────────────────────────────────────────────

func scrapeSingleURL(targetURL string, format string) ScrapeResult {
	startTime := time.Now()
	startISO := startTime.UTC().Format(time.RFC3339)

	result := ScrapeResult{URL: targetURL, StartTime: startISO}

	if format == "" {
		format = "markdown"
	}

	htmlContent, parsedURL, _, err := fetchHTML(targetURL)
	if err != nil {
		result.Error = err.Error()
		result.EndTime = time.Now().UTC().Format(time.RFC3339)
		result.Duration = time.Since(startTime).Milliseconds()
		return result
	}

	// ── Strategy 1: go-readability ────────────────────────────────────────────
	article, err := readability.FromReader(strings.NewReader(htmlContent), parsedURL)
	if err == nil && len(strings.TrimSpace(article.TextContent)) > 100 {
		plainText := cleanText(article.TextContent)
		title := article.Title
		if title == "" {
			title = extractTitleFromHTML(htmlContent)
		}

		result.Title = title
		result.Content = plainText
		result.WordCount = countWords(plainText)

		if format == "markdown" {
			if markdownStr, merr := htmlToMarkdown(article.Content, parsedURL.String()); merr == nil {
				result.Markdown = markdownStr
			} else {
				result.Markdown = plainText
			}
		}

		result.EndTime = time.Now().UTC().Format(time.RFC3339)
		result.Duration = time.Since(startTime).Milliseconds()
		log.Printf("[Go Scraper] Readability+MD: %s (%d words, %dms)", targetURL, result.WordCount, result.Duration)
		return result
	}

	// ── Strategy 2: goquery fallback ──────────────────────────────────────────
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		result.Error = fmt.Sprintf("parse failed: %v", err)
		result.EndTime = time.Now().UTC().Format(time.RFC3339)
		result.Duration = time.Since(startTime).Milliseconds()
		return result
	}

	for _, selector := range noiseSelectors {
		doc.Find(selector).Remove()
	}

	title := strings.TrimSpace(doc.Find("title").First().Text())
	if title == "" {
		title = strings.TrimSpace(doc.Find("h1").First().Text())
	}

	var textParts []string
	doc.Find("body").Find("h1, h2, h3, h4, h5, h6, p, li, blockquote, pre, td, th").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if len(text) < 3 {
			return
		}
		textParts = append(textParts, text)
	})

	content := cleanText(strings.Join(textParts, " "))
	if countWords(content) < 20 {
		content = cleanText(doc.Find("body").Text())
	}

	result.Title = title
	result.Content = content
	result.WordCount = countWords(content)

	if format == "markdown" {
		cleanedHTML, _ := doc.Find("body").Html()
		if markdownStr, merr := htmlToMarkdown(cleanedHTML, parsedURL.String()); merr == nil {
			result.Markdown = markdownStr
		} else {
			result.Markdown = content
		}
	}

	result.EndTime = time.Now().UTC().Format(time.RFC3339)
	result.Duration = time.Since(startTime).Milliseconds()
	log.Printf("[Go Scraper] GoQuery+MD: %s (%d words, %dms)", targetURL, result.WordCount, result.Duration)
	return result
}

// ─── HTML → Markdown ─────────────────────────────────────────────────────────

func htmlToMarkdown(htmlContent string, baseURL string) (string, error) {
	return md.ConvertString(htmlContent)
}

// ─── Site Mapper ─────────────────────────────────────────────────────────────

func mapSiteURLs(targetURL string, limit int) MapResult {
	startTime := time.Now()
	result := MapResult{SourceURL: targetURL}

	if limit <= 0 {
		limit = 100
	}

	parsedBase, err := url.Parse(targetURL)
	if err != nil {
		return result
	}
	baseDomain := parsedBase.Hostname()

	htmlContent, _, _, err := fetchHTML(targetURL)
	if err != nil {
		return result
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return result
	}

	seen := map[string]bool{targetURL: true}
	var links []MapLink

	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		if len(links) >= limit {
			return
		}
		href, exists := s.Attr("href")
		if !exists || href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") {
			return
		}

		resolved, err := parsedBase.Parse(href)
		if err != nil {
			return
		}

		// Only same-domain links
		if resolved.Hostname() != baseDomain {
			return
		}

		// Normalize: remove fragment and query
		resolved.Fragment = ""
		cleanURL := resolved.String()

		if seen[cleanURL] {
			return
		}
		seen[cleanURL] = true

		title := strings.TrimSpace(s.Text())
		if title == "" {
			title, _ = s.Attr("title")
		}

		links = append(links, MapLink{URL: cleanURL, Title: title})
	})

	result.Links = links
	result.Count = len(links)
	result.Duration = time.Since(startTime).Milliseconds()
	log.Printf("[Go Scraper] Map: %s → %d links (%dms)", targetURL, result.Count, result.Duration)
	return result
}

// ─── Recursive Crawler ───────────────────────────────────────────────────────

func crawlSite(targetURL string, limit, depth int, format string) CrawlResult {
	startTime := time.Now()

	if limit <= 0 {
		limit = 30
	}
	if depth <= 0 {
		depth = 2
	}

	result := CrawlResult{SourceURL: targetURL}

	parsedBase, err := url.Parse(targetURL)
	if err != nil {
		result.Error = "invalid URL"
		return result
	}
	baseDomain := parsedBase.Hostname()

	// BFS queue
	type queueItem struct {
		url   string
		depth int
	}
	queue := []queueItem{{url: targetURL, depth: 0}}
	visited := map[string]bool{targetURL: true}

	var mu sync.Mutex
	var pages []ScrapeResult
	sem := make(chan struct{}, 5) // 5 concurrent scrapers max

	var wg sync.WaitGroup

	for len(queue) > 0 && len(visited) <= limit {
		item := queue[0]
		queue = queue[1:]

		wg.Add(1)
		go func(u string, d int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			scraped := scrapeSingleURL(u, format)

			mu.Lock()
			pages = append(pages, scraped)

			// Discover more links if within depth
			if d < depth && len(visited) < limit {
				if scraped.Error == "" {
					// Parse the links from the scraped page
					htmlContent, _, _, ferr := fetchHTML(u)
					if ferr == nil {
						doc, derr := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
						if derr == nil {
							doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
								href, ok := s.Attr("href")
								if !ok || href == "" || strings.HasPrefix(href, "#") {
									return
								}
								resolved, rerr := parsedBase.Parse(href)
								if rerr != nil || resolved.Hostname() != baseDomain {
									return
								}
								resolved.Fragment = ""
								cleanURL := resolved.String()
								if !visited[cleanURL] && len(visited) < limit {
									visited[cleanURL] = true
									queue = append(queue, queueItem{url: cleanURL, depth: d + 1})
								}
							})
						}
					}
				}
			}
			mu.Unlock()
		}(item.url, item.depth)

		wg.Wait()
	}

	result.Pages = pages
	result.Total = len(pages)
	result.Duration = time.Since(startTime).Milliseconds()
	log.Printf("[Go Scraper] Crawl: %s → %d pages (%dms)", targetURL, result.Total, result.Duration)
	return result
}

// ─── Text Cleaning ───────────────────────────────────────────────────────────

func cleanText(text string) string {
	boxChars := []string{"┌", "┬", "┐", "├", "┼", "┤", "└", "┴", "┘", "│", "─", "━", "┏", "┳", "┓", "┣", "╋", "┫", "┗", "┻", "┛", "┃"}
	for _, char := range boxChars {
		text = strings.ReplaceAll(text, char, "")
	}

	lines := strings.Split(text, "\n")
	var cleaned []string
	prevEmpty := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		for strings.HasPrefix(line, "#") {
			line = strings.TrimSpace(strings.TrimLeft(line, "#"))
		}
		for strings.Contains(line, "  ") {
			line = strings.ReplaceAll(line, "  ", " ")
		}
		if line == "" {
			if !prevEmpty {
				cleaned = append(cleaned, "")
				prevEmpty = true
			}
			continue
		}
		prevEmpty = false
		cleaned = append(cleaned, line)
	}

	result := strings.Join(cleaned, " ")
	result = strings.ReplaceAll(result, "*", "")
	result = strings.ReplaceAll(result, "\n", " ")
	for strings.Contains(result, "  ") {
		result = strings.ReplaceAll(result, "  ", " ")
	}
	return strings.TrimSpace(result)
}

func countWords(text string) int {
	return len(strings.Fields(text))
}

func extractTitleFromHTML(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "No Title"
	}
	title := strings.TrimSpace(doc.Find("title").First().Text())
	if title == "" {
		title = strings.TrimSpace(doc.Find("h1").First().Text())
	}
	if title == "" {
		title = "No Title"
	}
	return title
}

// ─── HTTP Handlers ───────────────────────────────────────────────────────────

func scrapeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req ScrapeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid JSON"}`, http.StatusBadRequest)
		return
	}
	if req.URL == "" {
		http.Error(w, `{"error":"URL required"}`, http.StatusBadRequest)
		return
	}

	result := scrapeSingleURL(req.URL, req.Format)

	w.Header().Set("Content-Type", "application/json")
	if result.Error != "" {
		w.WriteHeader(http.StatusGatewayTimeout)
	}
	json.NewEncoder(w).Encode(result)
}

func batchScrapeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req BatchScrapeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid JSON"}`, http.StatusBadRequest)
		return
	}
	if len(req.URLs) == 0 {
		http.Error(w, `{"error":"URLs array required"}`, http.StatusBadRequest)
		return
	}

	maxConcurrent := 20
	if len(req.URLs) < maxConcurrent {
		maxConcurrent = len(req.URLs)
	}

	results := make([]ScrapeResult, len(req.URLs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrent)

	for i, u := range req.URLs {
		wg.Add(1)
		go func(index int, targetURL string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[index] = scrapeSingleURL(targetURL, req.Format)
		}(i, u)
	}

	wg.Wait()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func mapHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req MapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid JSON"}`, http.StatusBadRequest)
		return
	}
	if req.URL == "" {
		http.Error(w, `{"error":"URL required"}`, http.StatusBadRequest)
		return
	}

	result := mapSiteURLs(req.URL, req.Limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func crawlHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req CrawlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid JSON"}`, http.StatusBadRequest)
		return
	}
	if req.URL == "" {
		http.Error(w, `{"error":"URL required"}`, http.StatusBadRequest)
		return
	}

	// Safety caps
	if req.Limit > 50 {
		req.Limit = 50
	}
	if req.Depth > 3 {
		req.Depth = 3
	}

	result := crawlSite(req.URL, req.Limit, req.Depth, req.Format)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","engine":"go_scraper","endpoints":["/scrape","/scrape/batch","/map","/crawl","/health"]}`)
}

// ─── Main ────────────────────────────────────────────────────────────────────

func main() {
	port := "3002"

	mux := http.NewServeMux()
	mux.HandleFunc("/scrape", scrapeHandler)
	mux.HandleFunc("/scrape/batch", batchScrapeHandler)
	mux.HandleFunc("/map", mapHandler)
	mux.HandleFunc("/crawl", crawlHandler)
	mux.HandleFunc("/health", healthHandler)

	log.Printf("[Go Scraper] Starting on port %s", port)
	log.Printf("[Go Scraper] Endpoints: POST /scrape, POST /scrape/batch, POST /map, POST /crawl, GET /health")

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("[Go Scraper] Failed to start: %v", err)
	}
}
