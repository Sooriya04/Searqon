package search

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"

	"src/db"
	"src/models"
	"src/scraper"
	"src/utils"
)

type searxngResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

type searxngResponse struct {
	Results []searxngResult `json:"results"`
}

func getSearXNGBase() string {
	if u := os.Getenv("SEARXNG_URL"); u != "" {
		return u
	}
	return "http://localhost:4002"
}

func searchSearXNG(query string, limit int) ([]models.SearchResult, error) {
	base := getSearXNGBase()
	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "json")
	params.Set("language", "en")
	params.Set("engines", "google,bing,duckduckgo,qwant")

	reqURL := base + "/search?" + params.Encode()

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Searqon/2.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("searxng returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, err
	}

	var srResp searxngResponse
	if err := json.Unmarshal(body, &srResp); err != nil {
		return nil, err
	}

	var results []models.SearchResult
	for _, r := range srResp.Results {
		if r.URL == "" {
			continue
		}
		results = append(results, models.SearchResult{
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

func searchDDGFallback(query string, limit int) ([]models.SearchResult, error) {
	params := url.Values{"q": {query}}
	reqURL := "https://html.duckduckgo.com/html/?" + params.Encode()

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DDG returned HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []models.SearchResult
	doc.Find(".result").Each(func(i int, s *goquery.Selection) {
		if len(results) >= limit {
			return
		}

		title := strings.TrimSpace(s.Find(".result__title").Text())
		rawURL := strings.TrimSpace(s.Find(".result__url").Text())
		snippet := strings.TrimSpace(s.Find(".result__snippet").Text())

		if rawURL != "" && !strings.HasPrefix(rawURL, "http") {
			rawURL = "https://" + rawURL
		}

		if rawURL == "" || strings.Contains(rawURL, "duckduckgo.com") {
			return
		}

		results = append(results, models.SearchResult{
			Title:   title,
			URL:     rawURL,
			Snippet: snippet,
			Source:  "duckduckgo",
		})
	})

	return results, nil
}

// RunSearchPipeline coordinates query expansion, intent classification, web discovery, parallel scraping, and ranking.
func RunSearchPipeline(query string, limit int, scrape bool, bypassCache bool, maxWords int, summarize bool, extractSchema string) models.SearchResponse {
	start := time.Now()
	if limit <= 0 || limit > 10 {
		limit = 5
	}

	response := models.SearchResponse{Query: query}

	// 1. Check PostgreSQL cache first
	if !bypassCache {
		if cachedResults, provider, found := db.GetSearchCache(query); found {
			response.Results = cachedResults
			response.Total = len(cachedResults)
			response.Provider = provider
			response.Duration = time.Since(start).Milliseconds()
			return response
		}
	}

	// 2. Query Expansion & Intent Classification
	expandedQuery := ExpandQuery(query)
	intent := ClassifyIntent(query)
	limit, scrape = AdjustParamsByIntent(intent, limit, scrape)

	// 3. Web Discovery
	results, provider := discoverSearchResultsWithIntent(expandedQuery, limit, intent)
	response.Provider = provider
	if len(results) == 0 {
		response.Duration = time.Since(start).Milliseconds()
		response.Provider = "none"
		return response
	}

	// Deduplicate discovery URLs
	var uniqueResults []models.SearchResult
	seenURLs := make(map[string]bool)
	for _, r := range results {
		cleanURL := strings.TrimRight(r.URL, "/")
		if !seenURLs[cleanURL] {
			seenURLs[cleanURL] = true
			uniqueResults = append(uniqueResults, r)
		}
	}
	results = uniqueResults

	slog.Info("Search Pipeline Discovered URLs", "query", query, "count", len(results), "provider", response.Provider)
	for _, r := range results {
		slog.Info("Discovered URL", "url", r.URL, "title", r.Title, "source", r.Source)
	}

	// 4. Concurrent Scrape with 15s Deadline
	scrapeLimit := 10
	if len(results) < scrapeLimit {
		scrapeLimit = len(results)
	}

	if scrape && scrapeLimit > 0 {
		var urlsToPreResolve []string
		for i := 0; i < scrapeLimit; i++ {
			urlsToPreResolve = append(urlsToPreResolve, results[i].URL)
		}
		scraper.PreResolveDNS(urlsToPreResolve)

		scrapeCtx, scrapeCancel := context.WithTimeout(context.Background(), 15000*time.Millisecond)
		defer scrapeCancel()

		var mu sync.Mutex
		var wg sync.WaitGroup

		for i := 0; i < scrapeLimit; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				select {
				case <-scrapeCtx.Done():
					return
				default:
				}

				scraped, _ := scraper.ScrapeSingleURLWithOptions(results[idx].URL, scraper.ScrapeOptions{
					Format:        "markdown",
					BypassCache:   bypassCache,
					ExtractSchema: extractSchema,
				})

				select {
				case <-scrapeCtx.Done():
					slog.Warn("Scrape Timeout", "url", results[idx].URL)
					return
				default:
				}

				if scraped.Error == "" && scraped.WordCount > 50 {
					// Apply truncation control
					content := scraped.Content
					if maxWords > 0 {
						content = utils.TruncateTextByWords(content, maxWords)
					}

					mu.Lock()
					results[idx].Content = content
					results[idx].Markdown = scraped.Markdown
					results[idx].Metadata = scraped.Metadata
					results[idx].Scraped = true
					mu.Unlock()

					slog.Info("Scrape Success", "url", results[idx].URL, "word_count", scraped.WordCount, "duration_ms", scraped.Duration)

					// Async save embeddings if vector database is active
					go func(urlStr, txt string) {
						if emb, err := GetVectorEmbedding(txt); err == nil {
							db.SavePageEmbedding(urlStr, emb)
						}
					}(results[idx].URL, scraped.Content)
				} else {
					mu.Lock()
					results[idx].Scraped = false
					errReason := scraped.Error
					if errReason == "" {
						errReason = "Content too short (word count < 50)"
					}
					results[idx].ScrapeError = errReason
					mu.Unlock()

					slog.Error("Scrape Failed", "url", results[idx].URL, "error", errReason, "duration_ms", scraped.Duration)
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
		case <-scrapeCtx.Done():
		}
	}

	// 5. Content Deduplication & Relevance Ranking
	results = DeduplicateResults(results)
	results = HybridRankResults(results, query)

	response.Results = results
	response.Total = len(results)
	if summarize && len(results) > 0 {
		if summary, err := SynthesizeAnswer(query, results); err == nil {
			response.Summary = summary
		}
	}
	response.Duration = time.Since(start).Milliseconds()

	// 6. Save cache (only if we got results, to prevent caching transient timeouts)
	if len(results) > 0 {
		db.SaveSearchCache(query, results, response.Provider)
	}

	return response
}
