package handlers

import (
	"encoding/json"
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
	if result.Error != "" {
		utils.WriteError(w, http.StatusGatewayTimeout, utils.ErrCodeScrapeFailed, result.Error)
		return
	}

	utils.WriteSuccess(w, result)
}
