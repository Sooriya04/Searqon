package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// ─── Models ──────────────────────────────────────────────────────────────────

type SearchResult struct {
	Title    string `json:"title"`
	URL      string `json:"url"`
	Snippet  string `json:"snippet"`
	Source   string `json:"source"`             // "searxng" | "duckduckgo"
	Content  string `json:"content,omitempty"`  // scraped plain text
	Markdown string `json:"markdown,omitempty"` // scraped markdown
	Scraped  bool   `json:"scraped"`
}

type SearchResponse struct {
	Query    string         `json:"query"`
	Results  []SearchResult `json:"results"`
	Total    int            `json:"total"`
	Duration int64          `json:"duration"`
	Provider string         `json:"provider"` // which provider succeeded
}

func getSearXNGBase() string {
	if u := os.Getenv("SEARXNG_URL"); u != "" {
		return u
	}
	return "http://localhost:4002"
}

type searxngResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"` // snippet
}

type searxngResponse struct {
	Results []searxngResult `json:"results"`
}

func searchSearXNG(query string, limit int) ([]SearchResult, error) {
	base := getSearXNGBase()
	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "json")
	params.Set("language", "en")
	params.Set("engines", "google,bing,duckduckgo,qwant")

	reqURL := base + "/search?" + params.Encode()

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("searxng request creation failed: %v", err)
	}
	req.Header.Set("User-Agent", "Searqon/2.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("searxng unreachable at %s: %v", base, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("searxng returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, fmt.Errorf("searxng read failed: %v", err)
	}

	var srResp searxngResponse
	if err := json.Unmarshal(body, &srResp); err != nil {
		return nil, fmt.Errorf("searxng JSON parse failed: %v", err)
	}

	var results []SearchResult
	for _, r := range srResp.Results {
		if r.URL == "" {
			continue
		}
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
			Source:  "searxng",
		})
		if len(results) >= limit {
			break
		}
	}

	return results, nil
}

// ─── DuckDuckGo Fallback ─────────────────────────────────────────────────────
// Uses html.duckduckgo.com — a lightweight HTML endpoint that reliably returns
// results without JavaScript obfuscation. Parsed with goquery.

func searchDDGFallback(query string, limit int) ([]SearchResult, error) {
	params := url.Values{"q": {query}}
	reqURL := "https://html.duckduckgo.com/html/?" + params.Encode()

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("DDG lite request creation failed: %v", err)
	}
	// Use a realistic desktop browser UA to avoid the anomaly/CAPTCHA page
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("DNT", "1")

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DDG lite fetch failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DDG lite returned HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("DDG lite HTML parse failed: %v", err)
	}

	var results []SearchResult
	doc.Find(".result").Each(func(i int, s *goquery.Selection) {
		if len(results) >= limit {
			return
		}

		title := strings.TrimSpace(s.Find(".result__title").Text())
		rawURL := strings.TrimSpace(s.Find(".result__url").Text())
		snippet := strings.TrimSpace(s.Find(".result__snippet").Text())

		// Reconstruct full URL if it's just a domain/path
		if rawURL != "" && !strings.HasPrefix(rawURL, "http") {
			rawURL = "https://" + rawURL
		}

		// Skip DDG internal links and ads
		if rawURL == "" || strings.Contains(rawURL, "duckduckgo.com") {
			return
		}

		results = append(results, SearchResult{
			Title:   title,
			URL:     rawURL,
			Snippet: snippet,
			Source:  "duckduckgo",
		})
	})

	if len(results) == 0 {
		return nil, fmt.Errorf("DDG lite returned 0 results (possible block)")
	}

	return results, nil
}

// ─── Unified Search Pipeline ──────────────────────────────────────────────────
// 1. Try PostgreSQL cache first → sub-millisecond response on cache hits
// 2. Try SearXNG (local instance) → fast, unblocked, multi-engine
// 3. Fallback to DDG lite HTML endpoint parsed with goquery
// 4. URL deduplication to prevent duplicate scraping/indexing
// 5. Concurrent page scraping (goroutines) governed by global 2.5s deadline

func runSearchPipeline(query string, limit int, scrape bool, bypassCache bool) SearchResponse {
	start := time.Now()
	if limit <= 0 || limit > 10 {
		limit = 5
	}

	response := SearchResponse{Query: query}

	// ── Stage 0: Check Postgres cache first ────────────────────────────────────
	if !bypassCache {
		if cachedResults, provider, found := getSearchCache(query); found {
			log.Printf("[Search] [CACHE HIT] Query=%q Provider=%s (%d results)", query, provider, len(cachedResults))
			response.Results = cachedResults
			response.Total = len(cachedResults)
			response.Provider = provider
			response.Duration = time.Since(start).Milliseconds()
			return response
		}
	}

	// ── Stage 1: Search Discovery (SearXNG -> DDG) ─────────────────────────────
	results, err := searchSearXNG(query, limit)
	if err != nil {
		log.Printf("[Search] SearXNG failed (%v), falling back to DuckDuckGo", err)

		// Stage 2: DDG fallback
		results, err = searchDDGFallback(query, limit)
		if err != nil {
			log.Printf("[Search] DDG fallback also failed: %v", err)
			response.Duration = time.Since(start).Milliseconds()
			response.Provider = "none"
			return response
		}
		response.Provider = "duckduckgo"
	} else {
		response.Provider = "searxng"
	}

	log.Printf("[Search] Provider=%s found %d results for: %q", response.Provider, len(results), query)

	// ── Stage 2.5: URL Deduplication ───────────────────────────────────────────
	var uniqueResults []SearchResult
	seenURLs := make(map[string]bool)
	for _, r := range results {
		cleanURL := strings.TrimRight(r.URL, "/")
		if !seenURLs[cleanURL] {
			seenURLs[cleanURL] = true
			uniqueResults = append(uniqueResults, r)
		} else {
			log.Printf("[Search] Deduplicated URL: %s", r.URL)
		}
	}
	results = uniqueResults

	// ── Stage 3: Concurrent scraping with a GLOBAL 2.5s deadline ───────────────
	scrapeLimit := 3
	if len(results) < scrapeLimit {
		scrapeLimit = len(results)
	}

	if scrape && scrapeLimit > 0 {
		scrapeCtx, scrapeCancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
		defer scrapeCancel()

		var mu sync.Mutex
		var wg sync.WaitGroup

		for i := 0; i < scrapeLimit; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				select {
				case <-scrapeCtx.Done():
					log.Printf("[Scrape] ✗ %s (deadline exceeded)", results[idx].URL)
					return
				default:
				}

				// scrapeSingleURL handles page caching inside scraper.go
				scraped, _ := scrapeSingleURL(results[idx].URL, "markdown", bypassCache)

				select {
				case <-scrapeCtx.Done():
					log.Printf("[Scrape] ✗ %s (deadline exceeded after fetch)", results[idx].URL)
					return
				default:
				}

				if scraped.Error == "" && scraped.WordCount > 50 {
					mu.Lock()
					results[idx].Content = scraped.Content
					results[idx].Markdown = scraped.Markdown
					results[idx].Scraped = true
					mu.Unlock()
					log.Printf("[Scrape] ✓ %s (%d words)", results[idx].URL, scraped.WordCount)
				} else {
					log.Printf("[Scrape] ✗ %s (snippet fallback: %s)", results[idx].URL, scraped.Error)
				}
			}(i)
		}

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		select {
		case <-done:
			log.Printf("[Scrape] All pages done in %dms", time.Since(start).Milliseconds())
		case <-scrapeCtx.Done():
			log.Printf("[Scrape] Global deadline hit at %dms, returning available results", time.Since(start).Milliseconds())
		}
	}

	response.Results = results
	response.Total = len(results)
	response.Duration = time.Since(start).Milliseconds()

	// ── Stage 4: Cache results for subsequent queries ──────────────────────────
	saveSearchCache(query, results, response.Provider)

	return response
}

// ─── HTTP Handler ─────────────────────────────────────────────────────────────

type SearchRequest struct {
	Query       string `json:"query"`
	Limit       int    `json:"limit"`
	Scrape      *bool  `json:"scrape"` // pointer so we can detect if it was set
	BypassCache bool   `json:"bypass_cache"`
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var req SearchRequest
	defaultScrape := true

	if r.Method == http.MethodGet {
		req.Query = r.URL.Query().Get("q")
		req.Limit = 5
		if s := r.URL.Query().Get("scrape"); s == "false" {
			defaultScrape = false
		}
		if b := r.URL.Query().Get("bypass_cache"); b == "true" {
			req.BypassCache = true
		}
	} else if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"Invalid JSON body"}`, http.StatusBadRequest)
			return
		}
		if req.Limit == 0 {
			req.Limit = 5
		}
		if req.Scrape != nil {
			defaultScrape = *req.Scrape
		}
	} else {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if strings.TrimSpace(req.Query) == "" {
		http.Error(w, `{"error":"query is required"}`, http.StatusBadRequest)
		return
	}

	log.Printf("[Search] Query=%q Limit=%d Scrape=%v BypassCache=%v", req.Query, req.Limit, defaultScrape, req.BypassCache)
	result := runSearchPipeline(req.Query, req.Limit, defaultScrape, req.BypassCache)
	json.NewEncoder(w).Encode(result)
}
