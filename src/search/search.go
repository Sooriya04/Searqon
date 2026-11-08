package search

import (
	"context"
	"crypto/sha256"
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

	"src/config"
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
	cfg := config.Get()
	if cfg.SearXNG.URL != "" {
		return cfg.SearXNG.URL
	}
	if u := os.Getenv("SEARXNG_URL"); u != "" {
		return u
	}
	return "http://localhost:4002"
}

func searchSearXNG(query string, limit int) ([]models.SearchResult, error) {
	cfg := config.Get()
	if !cfg.SearXNG.Enabled {
		return nil, nil
	}
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
			Title:       r.Title,
			URL:         r.URL,
			Snippet:     r.Content,
			Source:      "searxng",
			FetchedAt:   time.Now().UTC().Format(time.RFC3339),
			ContentHash: computeContentHash(r.Content),
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
			Title:       title,
			URL:         rawURL,
			Snippet:     snippet,
			Source:      "duckduckgo",
			FetchedAt:   time.Now().UTC().Format(time.RFC3339),
			ContentHash: computeContentHash(snippet),
		})
	})

	return results, nil
}

// SearchEvent represents an incremental streaming event during pipeline execution.
type SearchEvent struct {
	Type     string               `json:"type"` // "discovery", "scrape", "result", "synthesis", "summary", "done"
	Data     interface{}          `json:"data,omitempty"`
	Result   *models.SearchResult `json:"result,omitempty"`
	Summary  string               `json:"summary,omitempty"`
	Provider string               `json:"provider,omitempty"`
	Total    int                  `json:"total,omitempty"`
}

// SearchEventHandler is a callback invoked as events occur in the search pipeline.
type SearchEventHandler func(SearchEvent)

// RunSearchPipeline coordinates query expansion, intent classification, web discovery, parallel scraping, and ranking.
func RunSearchPipeline(query string, limit int, scrape bool, bypassCache bool, maxWords int, summarize bool, extractSchema string) models.SearchResponse {
	return RunSearchPipelineWithContext(context.Background(), query, limit, scrape, bypassCache, maxWords, summarize, extractSchema)
}

func RunSearchPipelineWithContext(ctx context.Context, query string, limit int, scrape bool, bypassCache bool, maxWords int, summarize bool, extractSchema string) models.SearchResponse {
	return RunSearchPipelineStream(ctx, query, limit, scrape, bypassCache, maxWords, summarize, extractSchema, nil)
}

// RunSearchPipelineStream executes the search pipeline and invokes onEvent for incremental streaming updates.
func RunSearchPipelineStream(ctx context.Context, query string, limit int, scrape bool, bypassCache bool, maxWords int, summarize bool, extractSchema string, onEvent SearchEventHandler) models.SearchResponse {
	start := time.Now()
	if limit <= 0 || limit > 10 {
		limit = 5
	}

	response := models.SearchResponse{Query: query}

	if err := ctx.Err(); err != nil {
		response.Duration = time.Since(start).Milliseconds()
		response.Provider = "aborted"
		if onEvent != nil {
			onEvent(SearchEvent{
				Type: "done",
				Data: map[string]interface{}{
					"duration_ms": response.Duration,
					"error":       "aborted",
				},
			})
		}
		return response
	}

	// 1. Check exact cache first
	var queryEmbedding []float32
	if !bypassCache {
		if cachedResults, provider, found := db.GetSearchCache(query); found {
			utils.RecordSearchCacheHit()
			response.Results = cachedResults
			response.Total = len(cachedResults)
			response.Provider = provider
			response.Duration = time.Since(start).Milliseconds()

			if onEvent != nil {
				onEvent(SearchEvent{
					Type:     "discovery",
					Provider: provider,
					Total:    len(cachedResults),
					Data: map[string]interface{}{
						"query":    query,
						"provider": provider,
						"total":    len(cachedResults),
					},
				})
				for _, r := range cachedResults {
					item := r
					onEvent(SearchEvent{
						Type:   "result",
						Result: &item,
						Data:   item,
					})
				}
				onEvent(SearchEvent{
					Type: "done",
					Data: map[string]interface{}{
						"duration_ms": response.Duration,
					},
				})
			}
			return response
		}

		// 1b. Semantic query cache near-match lookup (Ollama / vector embeddings)
		if utils.IsOllamaEnabled() {
			if emb, err := GetVectorEmbedding(query); err == nil && len(emb) > 0 {
				queryEmbedding = emb
				if semResults, _, sim, found := db.GetSemanticSearchCache(query, queryEmbedding, 0.92); found {
					utils.RecordSearchCacheHit()
					response.Results = semResults
					response.Total = len(semResults)
					response.Provider = "semantic_cache"
					response.Duration = time.Since(start).Milliseconds()

					if onEvent != nil {
						onEvent(SearchEvent{
							Type:     "discovery",
							Provider: "semantic_cache",
							Total:    len(semResults),
							Data: map[string]interface{}{
								"query":      query,
								"provider":   "semantic_cache",
								"similarity": sim,
								"total":      len(semResults),
							},
						})
						for _, r := range semResults {
							item := r
							onEvent(SearchEvent{
								Type:   "result",
								Result: &item,
								Data:   item,
							})
						}
						onEvent(SearchEvent{
							Type: "done",
							Data: map[string]interface{}{
								"duration_ms": response.Duration,
							},
						})
					}
					return response
				}
			}
		}

		utils.RecordSearchCacheMiss()
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
		if onEvent != nil {
			onEvent(SearchEvent{
				Type:     "discovery",
				Provider: "none",
				Total:    0,
				Data: map[string]interface{}{
					"query":    query,
					"provider": "none",
					"total":    0,
				},
			})
			onEvent(SearchEvent{
				Type: "done",
				Data: map[string]interface{}{
					"duration_ms": response.Duration,
				},
			})
		}
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

	// Emit discovery event immediately upon search discovery & initial RRF
	if onEvent != nil {
		onEvent(SearchEvent{
			Type:     "discovery",
			Provider: response.Provider,
			Total:    len(results),
			Data: map[string]interface{}{
				"query":    query,
				"provider": response.Provider,
				"total":    len(results),
			},
		})
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

		scrapeCtx, scrapeCancel := context.WithTimeout(ctx, 15000*time.Millisecond)
		defer scrapeCancel()

		var mu sync.Mutex
		var wg sync.WaitGroup

		for i := 0; i < scrapeLimit; i++ {
			targetURL := results[i].URL
			wg.Add(1)
			go func(idx int, targetURL string) {
				defer wg.Done()

				select {
				case <-scrapeCtx.Done():
					return
				default:
				}

				scraped, _ := scraper.ScrapeSingleURLWithOptionsContext(scrapeCtx, targetURL, scraper.ScrapeOptions{
					Format:        "markdown",
					BypassCache:   bypassCache,
					ExtractSchema: extractSchema,
				})

				select {
				case <-scrapeCtx.Done():
					slog.Warn("Scrape Timeout", "url", targetURL)
					return
				default:
				}

				if scraped.Error == "" && scraped.WordCount > 50 {
					// Apply truncation control
					content := scraped.Content
					if maxWords > 0 {
						content = utils.TruncateTextByWords(content, maxWords)
					}

					var itemCopy models.SearchResult
					mu.Lock()
					if idx < len(results) {
						results[idx].Content = content
						results[idx].Markdown = scraped.Markdown
						results[idx].Metadata = scraped.Metadata
						results[idx].Scraped = true
						results[idx].ContentHash = scraped.ContentHash
						if scraped.ScrapedAt != "" {
							results[idx].FetchedAt = scraped.ScrapedAt
						} else {
							results[idx].FetchedAt = time.Now().UTC().Format(time.RFC3339)
						}
						itemCopy = results[idx]
					}
					mu.Unlock()

					slog.Info("Scrape Success", "url", targetURL, "word_count", scraped.WordCount, "duration_ms", scraped.Duration)

					if onEvent != nil && itemCopy.URL != "" {
						onEvent(SearchEvent{
							Type:   "scrape",
							Result: &itemCopy,
							Data:   itemCopy,
						})
					}

					// Async save embeddings if vector database is active
					go func(urlStr, txt string) {
						if emb, err := GetVectorEmbedding(txt); err == nil {
							db.SavePageEmbedding(urlStr, emb)
						}
					}(targetURL, scraped.Content)
				} else {
					var itemCopy models.SearchResult
					errReason := scraped.Error
					if errReason == "" {
						errReason = "Content too short (word count < 50)"
					}

					mu.Lock()
					if idx < len(results) {
						results[idx].Scraped = false
						results[idx].ScrapeError = errReason
						itemCopy = results[idx]
					}
					mu.Unlock()

					slog.Error("Scrape Failed", "url", targetURL, "error", errReason, "duration_ms", scraped.Duration)

					if onEvent != nil && itemCopy.URL != "" {
						onEvent(SearchEvent{
							Type:   "scrape",
							Result: &itemCopy,
							Data:   itemCopy,
						})
					}
				}
			}(i, targetURL)
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

	// Stream final ranked results
	if onEvent != nil {
		for _, item := range results {
			it := item
			onEvent(SearchEvent{
				Type:   "result",
				Result: &it,
				Data:   it,
			})
		}
	}

	response.Results = results
	response.Total = len(results)
	if summarize && len(results) > 0 {
		if onEvent != nil {
			onEvent(SearchEvent{
				Type: "synthesis",
				Data: map[string]interface{}{"status": "synthesizing"},
			})
		}
		if summary, err := SynthesizeAnswer(query, results); err == nil {
			response.Summary = summary
			if onEvent != nil {
				onEvent(SearchEvent{
					Type:    "summary",
					Summary: summary,
					Data: map[string]interface{}{
						"summary": summary,
					},
				})
			}
		}
	}
	response.Duration = time.Since(start).Milliseconds()

	// 6. Save cache (only if we got results, to prevent caching transient timeouts)
	if len(results) > 0 {
		if len(queryEmbedding) == 0 && utils.IsOllamaEnabled() {
			if emb, err := GetVectorEmbedding(query); err == nil && len(emb) > 0 {
				queryEmbedding = emb
			}
		}
		db.SaveSearchCacheWithEmbedding(query, results, response.Provider, queryEmbedding)
	}

	if onEvent != nil {
		onEvent(SearchEvent{
			Type: "done",
			Data: map[string]interface{}{
				"duration_ms": response.Duration,
				"summary":     response.Summary,
			},
		})
	}

	return response
}

func computeContentHash(text string) string {
	if text == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(text))
	return fmt.Sprintf("%x", sum)
}
