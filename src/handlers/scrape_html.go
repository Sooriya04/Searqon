package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"src/models"
	"src/scraper"
	"src/utils"
)

// HTMLScrapeHandler parses raw HTML provided in the request body.
func HTMLScrapeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, utils.ErrCodeMethodNotAllowed, "Method not allowed")
		return
	}

	var req models.HTMLScrapeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeInvalidJSON, "Invalid JSON")
		return
	}
	if req.HTML == "" {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeMissingParam, "HTML is required")
		return
	}
	if req.URL == "" {
		req.URL = "https://example.com"
	}

	startTime := time.Now()
	result := scraper.ScrapeHTMLContent(req.HTML, req.URL, req.URL, req.Format, startTime)

	if result.Error != "" {
		utils.WriteError(w, http.StatusInternalServerError, utils.ErrCodeInternalError, result.Error)
		return
	}

	utils.WriteSuccess(w, result)
}
