package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"src/chunker"
	"src/models"
	"src/scraper"
	"src/search"
	"src/utils"
)

// PipelineHandler coordinates Search → Fetch → Chunk → Rank flow.
func PipelineHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, utils.ErrCodeMethodNotAllowed, "Method not allowed")
		return
	}

	var req models.PipelineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeInvalidJSON, "Invalid JSON body")
		return
	}

	if req.Query == "" {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeMissingParam, "Query parameter 'query' is required")
		return
	}

	if req.MaxSources <= 0 {
		req.MaxSources = 5
	}
	if req.MaxSources > 10 {
		req.MaxSources = 10
	}

	startTime := time.Now()

	// 1. Discover top URLs using the search pipeline (without scraping inside search phase)
	searchResp := search.RunSearchPipeline(req.Query, req.MaxSources, false, req.BypassCache, 0, false, "")

	// 2. Scrape discovered URLs in parallel (with concurrency limit of 3)
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, 3)

	sources := make([]models.PipelineSource, 0, len(searchResp.Results))
	var allChunks []models.Chunk

	scrapeCtx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	for _, result := range searchResp.Results {
		wg.Add(1)
		go func(res models.SearchResult) {
			defer wg.Done()

			// Acquire concurrency slot or abort on timeout
			select {
			case <-scrapeCtx.Done():
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()

			scraped, _ := scraper.ScrapeSingleURL(res.URL, "markdown", req.BypassCache)

			if scraped.Error != "" || !scraped.Scraped {
				return
			}

			src := models.PipelineSource{
				URL:          scraped.URL,
				Title:        scraped.Title,
				RenderMethod: scraped.RenderMethod,
				Cached:       scraped.Cached,
				Chunks:       scraped.Chunks,
			}

			mu.Lock()
			sources = append(sources, src)
			allChunks = append(allChunks, scraped.Chunks...)
			mu.Unlock()
		}(result)
	}

	wg.Wait()

	// Guard: if no sources were scraped successfully, return an error response
	if len(sources) == 0 {
		utils.WriteError(w, http.StatusBadGateway, utils.ErrCodeScrapeFailed, "No sources could be fetched or scraped successfully")
		return
	}

	allChunks = chunker.DeduplicateChunks(allChunks)

	// 3. Score all chunks using BM25
	scoredChunks := chunker.ScoreChunks(allChunks, req.Query)

	// Group scored chunks back by URL
	urlToChunks := make(map[string][]models.Chunk)
	for _, chunk := range scoredChunks {
		url := chunk.Metadata.SourceURL
		urlToChunks[url] = append(urlToChunks[url], chunk)
	}

	// Update each source with its scored chunks and sort by BM25 score DESC
	for i := range sources {
		url := sources[i].URL
		sources[i].Chunks = urlToChunks[url]
		sort.Slice(sources[i].Chunks, func(j, k int) bool {
			return sources[i].Chunks[j].BM25Score > sources[i].Chunks[k].BM25Score
		})
	}

	// Optional: Sort sources based on their highest chunk score
	sort.Slice(sources, func(i, j int) bool {
		valI := 0.0
		valJ := 0.0
		if len(sources[i].Chunks) > 0 {
			valI = sources[i].Chunks[0].BM25Score
		}
		if len(sources[j].Chunks) > 0 {
			valJ = sources[j].Chunks[0].BM25Score
		}
		return valI > valJ
	})

	// Create globally sorted slice of all chunks to generate clean unified context
	globalScoredChunks := make([]models.Chunk, len(scoredChunks))
	copy(globalScoredChunks, scoredChunks)
	sort.Slice(globalScoredChunks, func(i, j int) bool {
		return globalScoredChunks[i].BM25Score > globalScoredChunks[j].BM25Score
	})

	// Build flat unified context string of top chunks (up to top 6)
	var contextBuilder strings.Builder
	limitChunks := 6
	if len(globalScoredChunks) < limitChunks {
		limitChunks = len(globalScoredChunks)
	}
	for i := 0; i < limitChunks; i++ {
		c := globalScoredChunks[i]
		if i > 0 {
			contextBuilder.WriteString("\n\n")
		}
		contextBuilder.WriteString(fmt.Sprintf("[%d] Source: %s (%s)\n%s", i+1, c.Metadata.SourceTitle, c.Metadata.SourceURL, c.Text))
	}

	response := models.PipelineResponse{
		Query:       req.Query,
		FetchedAt:   startTime.UTC().Format(time.RFC3339),
		Sources:     sources,
		TotalChunks: len(scoredChunks),
		DurationMS:  time.Since(startTime).Milliseconds(),
		Context:     contextBuilder.String(),
	}

	utils.WriteSuccess(w, response)
}
