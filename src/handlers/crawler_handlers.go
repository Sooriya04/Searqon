package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"src/crawler"
	"src/models"
	"src/utils"
)

// MapHandler generates a domain sitemap links list.
func MapHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, utils.ErrCodeMethodNotAllowed, "Method not allowed")
		return
	}

	var req models.MapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeInvalidJSON, "Invalid JSON")
		return
	}
	if req.URL == "" {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeMissingParam, "URL is required")
		return
	}

	result := crawler.MapSiteURLs(req.URL, req.Limit)
	if result.Error != "" {
		utils.WriteError(w, http.StatusInternalServerError, utils.ErrCodeInternalError, result.Error)
		return
	}

	utils.WriteSuccess(w, result)
}

// CrawlHandler recursively scrapes target pages under a domain.
func CrawlHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, utils.ErrCodeMethodNotAllowed, "Method not allowed")
		return
	}

	var req models.CrawlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeInvalidJSON, "Invalid JSON")
		return
	}
	if req.URL == "" {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeMissingParam, "URL is required")
		return
	}

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
			utils.WriteError(w, http.StatusInternalServerError, utils.ErrCodeInternalError, "Streaming unsupported")
			return
		}

		onPageScraped := func(page models.ScrapeResult) {
			data, err := json.Marshal(page)
			if err == nil {
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}

		crawler.CrawlSite(req.URL, req.Limit, req.Depth, req.Format, onPageScraped)

		fmt.Fprintf(w, "event: done\ndata: {}\n\n")
		flusher.Flush()
	} else {
		result := crawler.CrawlSite(req.URL, req.Limit, req.Depth, req.Format, nil)
		if result.Error != "" {
			utils.WriteError(w, http.StatusInternalServerError, utils.ErrCodeInternalError, result.Error)
			return
		}
		utils.WriteSuccess(w, result)
	}
}
