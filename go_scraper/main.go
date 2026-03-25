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

	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/brotli"
	readability "github.com/go-shiori/go-readability"
)

// ─── Request / Response Models ───────────────────────────────────────────────

type ScrapeRequest struct {
	URL string `json:"url"`
}

type BatchScrapeRequest struct {
	URLs []string `json:"urls"`
}

type ScrapeResult struct {
	Title     string `json:"title"`
	Content   string `json:"content"`
	URL       string `json:"url"`
	WordCount int    `json:"wordCount"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Duration  int64  `json:"duration"`
	Error     string `json:"error,omitempty"`
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
	Timeout: 10 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

var defaultHeaders = map[string]string{
	"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	"Accept":                   "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
	"Accept-Language":          "en-US,en;q=0.9",
	"Cache-Control":            "no-cache",
	"Sec-Ch-Ua":                `"Not A(Brand";v="99", "Google Chrome";v="121", "Chromium";v="121"`,
	"Sec-Ch-Ua-Mobile":         "?0",
	"Sec-Ch-Ua-Platform":       `"Windows"`,
	"Sec-Fetch-Dest":           "document",
	"Sec-Fetch-Mode":           "navigate",
	"Sec-Fetch-Site":           "none",
	"Sec-Fetch-User":           "?1",
	"Upgrade-Insecure-Requests": "1",
}

// ─── Core Scraping Function ──────────────────────────────────────────────────

func scrapeSingleURL(targetURL string) ScrapeResult {
	startTime := time.Now()
	startISO := startTime.UTC().Format(time.RFC3339)

	result := ScrapeResult{
		URL:       targetURL,
		StartTime: startISO,
	}

	// Validate URL
	parsedURL, err := url.Parse(targetURL)
	if err != nil || parsedURL.Scheme == "" {
		result.Error = "Invalid URL"
		result.EndTime = time.Now().UTC().Format(time.RFC3339)
		result.Duration = time.Since(startTime).Milliseconds()
		return result
	}

	// Create request with stealth headers
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		result.Error = fmt.Sprintf("Request creation failed: %v", err)
		result.EndTime = time.Now().UTC().Format(time.RFC3339)
		result.Duration = time.Since(startTime).Milliseconds()
		return result
	}

	for key, value := range defaultHeaders {
		req.Header.Set(key, value)
	}

	// Fetch
	resp, err := httpClient.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("Fetch failed: %v", err)
		result.EndTime = time.Now().UTC().Format(time.RFC3339)
		result.Duration = time.Since(startTime).Milliseconds()
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		result.EndTime = time.Now().UTC().Format(time.RFC3339)
		result.Duration = time.Since(startTime).Milliseconds()
		return result
	}

	// Check Content-Type to avoid parsing binary files (PDFs, images, zips)
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if contentType != "" && !strings.Contains(contentType, "text/") && !strings.Contains(contentType, "xml") && !strings.Contains(contentType, "json") {
		result.Error = fmt.Sprintf("Skipped binary Content-Type: %s", contentType)
		result.EndTime = time.Now().UTC().Format(time.RFC3339)
		result.Duration = time.Since(startTime).Milliseconds()
		return result
	}

	// Decompress body if needed (some servers ignore Accept-Encoding and force compression)
	var reader io.Reader = resp.Body
	switch strings.ToLower(resp.Header.Get("Content-Encoding")) {
	case "gzip":
		importGzip := true
		_ = importGzip
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

	// Read body (limit to 5MB to prevent OOM)
	body, err := io.ReadAll(io.LimitReader(reader, 5*1024*1024))
	if err != nil {
		result.Error = fmt.Sprintf("Read failed: %v", err)
		result.EndTime = time.Now().UTC().Format(time.RFC3339)
		result.Duration = time.Since(startTime).Milliseconds()
		return result
	}

	htmlContent := string(body)

	// Strategy 1: Try go-readability (article extraction like Mozilla Readability)
	article, err := readability.FromReader(strings.NewReader(htmlContent), parsedURL)
	if err == nil && len(strings.TrimSpace(article.TextContent)) > 100 {
		content := cleanText(article.TextContent)
		title := article.Title
		if title == "" {
			title = extractTitleFromHTML(htmlContent)
		}

		result.Title = title
		result.Content = content
		result.WordCount = countWords(content)
		result.EndTime = time.Now().UTC().Format(time.RFC3339)
		result.Duration = time.Since(startTime).Milliseconds()
		log.Printf("[Go Scraper] Readability success: %s (%d words, %dms)", targetURL, result.WordCount, result.Duration)
		return result
	}

	// Strategy 2: Fallback to goquery noise-removal
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		result.Error = fmt.Sprintf("Parse failed: %v", err)
		result.EndTime = time.Now().UTC().Format(time.RFC3339)
		result.Duration = time.Since(startTime).Milliseconds()
		return result
	}

	// Remove noise elements
	for _, selector := range noiseSelectors {
		doc.Find(selector).Remove()
	}

	// Extract title
	title := doc.Find("title").First().Text()
	title = strings.TrimSpace(title)
	if title == "" {
		title = doc.Find("h1").First().Text()
		title = strings.TrimSpace(title)
	}

	// Extract structured text
	var textParts []string

	// Get headings and paragraphs in order
	doc.Find("body").Find("h1, h2, h3, h4, h5, h6, p, li, blockquote, pre, td, th").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if len(text) < 3 {
			return
		}
		textParts = append(textParts, text)
	})

	content := cleanText(strings.Join(textParts, " "))

	// If goquery extraction is too short, fall back to body text
	if countWords(content) < 20 {
		content = cleanText(doc.Find("body").Text())
	}

	result.Title = title
	result.Content = content
	result.WordCount = countWords(content)
	result.EndTime = time.Now().UTC().Format(time.RFC3339)
	result.Duration = time.Since(startTime).Milliseconds()

	log.Printf("[Go Scraper] GoQuery success: %s (%d words, %dms)", targetURL, result.WordCount, result.Duration)
	return result
}

// ─── Text Cleaning ───────────────────────────────────────────────────────────

func cleanText(text string) string {
	// Remove ASCII box drawing characters often found in diagrams
	boxChars := []string{"┌", "┬", "┐", "├", "┼", "┤", "└", "┴", "┘", "│", "─", "━", "┏", "┳", "┓", "┣", "╋", "┫", "┗", "┻", "┛", "┃"}
	for _, char := range boxChars {
		text = strings.ReplaceAll(text, char, "")
	}

	// Split into lines, trim each, remove empty
	lines := strings.Split(text, "\n")
	var cleaned []string
	prevEmpty := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Remove markdown hash headers if present
		for strings.HasPrefix(line, "#") {
			line = strings.TrimSpace(strings.TrimLeft(line, "#"))
		}

		// Collapse multiple spaces
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
	
	// Final pass to collapse double spaces made by joining
	for strings.Contains(result, "  ") {
		result = strings.ReplaceAll(result, "  ", " ")
	}
	
	result = strings.TrimSpace(result)
	return result
}

func countWords(text string) int {
	fields := strings.Fields(text)
	return len(fields)
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

	result := scrapeSingleURL(req.URL)

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

	// Cap at 20 concurrent scrapes to prevent resource exhaustion
	maxConcurrent := 20
	if len(req.URLs) < maxConcurrent {
		maxConcurrent = len(req.URLs)
	}

	results := make([]ScrapeResult, len(req.URLs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrent) // Semaphore for concurrency control

	for i, u := range req.URLs {
		wg.Add(1)
		go func(index int, targetURL string) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			results[index] = scrapeSingleURL(targetURL)
		}(i, u)
	}

	wg.Wait()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","engine":"go_scraper","goroutines_max":20}`)
}

// ─── Main ────────────────────────────────────────────────────────────────────

func main() {
	port := "3002"

	mux := http.NewServeMux()
	mux.HandleFunc("/scrape", scrapeHandler)
	mux.HandleFunc("/scrape/batch", batchScrapeHandler)
	mux.HandleFunc("/health", healthHandler)

	log.Printf("[Go Scraper] Server starting on port %s", port)
	log.Printf("[Go Scraper] Endpoints: POST /scrape, POST /scrape/batch, GET /health")

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("[Go Scraper] Failed to start: %v", err)
	}
}
