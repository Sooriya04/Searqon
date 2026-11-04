package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"src/models"
	"src/scraper"
	"src/utils"
)

// ChunkedScrapeHandler returns a scrape response with chunking forced on.
func ChunkedScrapeHandler(w http.ResponseWriter, r *http.Request) {
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

	result, _ := scraper.ScrapeSingleURLWithOptions(req.URL, scraper.ScrapeOptions{
		Format:        req.Format,
		BypassCache:   req.BypassCache,
		ExtractSchema: req.ExtractSchema,
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

	utils.WriteSuccess(w, result)
}
