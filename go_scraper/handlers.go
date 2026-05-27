package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
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
	Limit  int    `json:"limit"`  // max pages, default 30
	Depth  int    `json:"depth"`  // link depth, default 2
	Format string `json:"format"`
	Stream bool   `json:"stream"` // Enable SSE real-time streaming
}

type HTMLScrapeRequest struct {
	HTML   string `json:"html"`
	URL    string `json:"url"`
	Format string `json:"format"`
}

type ScrapeResult struct {
	Title     string `json:"title"`
	Content   string `json:"content"`            // plain text (always present)
	Markdown  string `json:"markdown,omitempty"` // markdown formatted version
	URL       string `json:"url"`
	WordCount int    `json:"wordCount"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Duration  int64  `json:"duration"` // ms
	Error     string `json:"error,omitempty"`
}

type MapLink struct {
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
}

type MapResult struct {
	SourceURL string    `json:"sourceUrl"`
	Links     []MapLink `json:"links"`
	Count     int       `json:"count"`
	Duration  int64     `json:"duration"` // ms
	Error     string    `json:"error,omitempty"`
}

type CrawlResult struct {
	SourceURL string         `json:"sourceUrl"`
	Pages     []ScrapeResult `json:"pages"`
	Total     int            `json:"total"`
	Duration  int64          `json:"duration"`
	Error     string         `json:"error,omitempty"`
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

	result, _ := scrapeSingleURL(req.URL, req.Format)

	w.Header().Set("Content-Type", "application/json")
	if result.Error != "" {
		w.WriteHeader(http.StatusGatewayTimeout)
	}
	json.NewEncoder(w).Encode(result)
}

func scrapeHTMLHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req HTMLScrapeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid JSON"}`, http.StatusBadRequest)
		return
	}
	if req.HTML == "" {
		http.Error(w, `{"error":"HTML required"}`, http.StatusBadRequest)
		return
	}
	if req.URL == "" {
		req.URL = "https://example.com"
	}

	startTime := time.Now()
	result := scrapeHTMLContent(req.HTML, req.URL, req.Format, startTime)

	w.Header().Set("Content-Type", "application/json")
	if result.Error != "" {
		w.WriteHeader(http.StatusInternalServerError)
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
	sem := make(chan struct{}, maxConcurrent)

	// Stream execution wait group
	for i, u := range req.URLs {
		go func(index int, targetURL string) {
			sem <- struct{}{}
			defer func() { <-sem }()
			scraped, _ := scrapeSingleURL(targetURL, req.Format)
			results[index] = scraped
		}(i, u)
	}

	// Simple check loop for batch results
	for {
		time.Sleep(10 * time.Millisecond)
		allDone := true
		for _, res := range results {
			if res.StartTime == "" {
				allDone = false
				break
			}
		}
		if allDone {
			break
		}
	}

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

	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Transfer-Encoding", "chunked")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, `Streaming unsupported`, http.StatusInternalServerError)
			return
		}

		onPageScraped := func(page ScrapeResult) {
			data, err := json.Marshal(page)
			if err == nil {
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}

		crawlSite(req.URL, req.Limit, req.Depth, req.Format, onPageScraped)

		// Send final done event
		fmt.Fprintf(w, "event: done\ndata: {}\n\n")
		flusher.Flush()
	} else {
		result := crawlSite(req.URL, req.Limit, req.Depth, req.Format, nil)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","engine":"go_scraper","endpoints":["/scrape","/scrape/html","/scrape/batch","/map","/crawl","/health"]}`)
}
