package handlers

import (
	"encoding/json"
	"net/http"

	"src/scraper"
	"src/search"
	"src/utils"
)

type SummarizeRequest struct {
	URL         string `json:"url"`
	BypassCache bool   `json:"bypass_cache"`
}

type SummarizeResponse struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

// SummarizeHandler leverages Ollama to generate a structured content summary.
func SummarizeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, utils.ErrCodeMethodNotAllowed, "Method not allowed")
		return
	}

	var req SummarizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeInvalidJSON, "Invalid JSON request body")
		return
	}
	if req.URL == "" {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeMissingParam, "URL is required")
		return
	}

	result, _ := scraper.ScrapeSingleURL(req.URL, "text", req.BypassCache)
	if result.Error != "" {
		utils.WriteError(w, http.StatusBadGateway, utils.ErrCodeScrapeFailed, result.Error)
		return
	}

	summary, err := search.SummarizePage(result.Content)
	if err != nil {
		utils.WriteError(w, http.StatusBadGateway, utils.ErrCodeInternalError, "Ollama summarization failed: "+err.Error())
		return
	}

	utils.WriteSuccess(w, SummarizeResponse{
		URL:     result.URL,
		Title:   result.Title,
		Summary: summary,
	})
}
