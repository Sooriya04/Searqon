package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"src/models"
	"src/scraper"
	"src/search"
	"src/utils"
)

// SearchHandler handles the web search query pipeline.
func SearchHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var req models.SearchRequest
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
			utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeInvalidJSON, "Invalid JSON body")
			return
		}
		if req.Limit == 0 {
			req.Limit = 5
		}
		if req.Scrape != nil {
			defaultScrape = *req.Scrape
		}
	} else {
		utils.WriteError(w, http.StatusMethodNotAllowed, utils.ErrCodeMethodNotAllowed, "Method not allowed")
		return
	}

	if strings.TrimSpace(req.Query) == "" {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeMissingParam, "Query parameter 'q' is required")
		return
	}

	result := search.RunSearchPipeline(req.Query, req.Limit, defaultScrape, req.BypassCache, req.MaxWords, req.Summarize, req.ExtractSchema)
	utils.WriteSuccess(w, result)
}

// ScrapeHandler handles scraping a single URL.
func ScrapeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, utils.ErrCodeMethodNotAllowed, "Method not allowed")
		return
	}

	var req models.ScrapeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeInvalidJSON, "Invalid JSON")
		return
	}
	if req.URL == "" {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeMissingParam, "URL is required")
		return
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = req.TokenLimit
	}

	result, _ := scraper.ScrapeSingleURLWithOptions(req.URL, scraper.ScrapeOptions{
		Format:        req.Format,
		BypassCache:   req.BypassCache,
		ExtractSchema: req.ExtractSchema,
		StealthLevel:  req.StealthLevel,
		MaxTokens:     maxTokens,
	})
	if result.Error != "" || result.StructuredError != nil {
		se := result.StructuredError
		if se == nil {
			se = models.ClassifyError(result.StatusCode, result.Content, fmt.Errorf("%s", result.Error))
		}
		if se == nil {
			se = &models.SearqonError{
				ErrorType:         models.ErrUpstreamUnavailable,
				StatusCode:        http.StatusBadGateway,
				Message:           result.Error,
				Retryable:         false,
				SuggestedFallback: models.FallbackLightpanda,
			}
		}
		utils.WriteSearqonError(w, se)
		return
	}
	if !req.Chunk {
		result.Chunks = nil
	}

	utils.WriteSuccess(w, result)
}

// HealthHandler returns API health status.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	utils.WriteSuccess(w, map[string]interface{}{
		"status":    "ok",
		"engine":    "src",
		"endpoints": []string{"/search", "/scrape", "/scrape/chunked", "/scrape/batch", "/map", "/crawl", "/health", "/r/", "/pipeline"},
	})
}
