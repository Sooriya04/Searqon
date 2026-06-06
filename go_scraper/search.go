package main

import (
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
)

// ─── Models ──────────────────────────────────────────────────────────────────

type SearchResult struct {
	Title    string `json:"title"`
	URL      string `json:"url"`
	Snippet  string `json:"snippet"`
	Source   string `json:"source"`            // "searxng" | "duckduckgo"
	Content  string `json:"content,omitempty"` // scraped plain text
	Markdown string `json:"markdown,omitempty"` // scraped markdown
	Scraped  bool   `json:"scraped"`
}

type SearchResponse struct {
	Query      string         `json:"query"`
	Results    []SearchResult `json:"results"`
	Total      int            `json:"total"`
	Duration   int64          `json:"duration"`
	Provider   string         `json:"provider"` // which provider succeeded
}

// ─── SearXNG Provider ─────────────────────────────────────────────────────────
// Expects a locally running SearXNG instance (e.g. docker run -p 8080:8080 searxng/searxng)

const searxngBase = "http://localhost:8080"

type searxngResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"` // snippet
}

type searxngResponse struct {
	Results []searxngResult `json:"results"`
}

func searchSearXNG(query string, limit int) ([]SearchResult, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "json")
	params.Set("language", "en")
	params.Set("engines", "google,bing,duckduckgo,qwant")

	reqURL := searxngBase + "/search?" + params.Encode()

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("searxng request creation failed: %v", err)
	}
	req.Header.Set("User-Agent", "Searqon/2.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("searxng unreachable at %s: %v", searxngBase, err)
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
	reqURL := "https://html.duckduckgo.com/html/?"+params.Encode()

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
// 1. Try SearXNG (local instance) → fast, unblocked, multi-engine
// 2. Fallback to DDG lite HTML endpoint parsed with goquery
// 3. Concurrent page scraping (goroutines) — capped at top 3 for speed
//    Each scrape has a 3s hard timeout; slow pages fall back to snippet.

func runSearchPipeline(query string, limit int, scrape bool) SearchResponse {
	start := time.Now()
	if limit <= 0 || limit > 10 {
		limit = 5
	}

	response := SearchResponse{Query: query}

	// Stage 1: SearXNG
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

	// Stage 3: Concurrent scraping with a GLOBAL 2.5s deadline across all pages.
	// This means: whatever pages complete within 2.5s are included.
	// Slow/Cloudflare-blocked pages automatically fall back to their snippets.
	// Total latency = DDG_search_time + 2.5s (not per_page_timeout × N)
	scrapeLimit := 3
	if len(results) < scrapeLimit {
		scrapeLimit = len(results)
	}

	if scrape && scrapeLimit > 0 {
		// Global deadline: all scrapes must finish within 2.5s or are skipped
		scrapeCtx, scrapeCancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
		defer scrapeCancel()

		var mu sync.Mutex
		var wg sync.WaitGroup

		for i := 0; i < scrapeLimit; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				// If the global deadline has already passed, skip this page immediately
				select {
				case <-scrapeCtx.Done():
					log.Printf("[Scrape] ✗ %s (deadline exceeded)", results[idx].URL)
					return
				default:
				}

				scraped, _ := scrapeSingleURL(results[idx].URL, "markdown")

				// Check again after scrape returns (in case deadline passed during scrape)
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

		// Wait for all goroutines OR the global deadline, whichever comes first
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
	return response
}

// ─── HTTP Handler ─────────────────────────────────────────────────────────────

type SearchRequest struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit"`
	Scrape *bool  `json:"scrape"` // pointer so we can detect if it was set
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

	log.Printf("[Search] Query=%q Limit=%d Scrape=%v", req.Query, req.Limit, defaultScrape)
	result := runSearchPipeline(req.Query, req.Limit, defaultScrape)
	json.NewEncoder(w).Encode(result)
}
