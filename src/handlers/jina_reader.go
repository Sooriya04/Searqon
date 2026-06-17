package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"src/scraper"
	"src/utils"
)

// JinaReaderHandler implements compatibility with the Jina Reader API protocol.
func JinaReaderHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != http.MethodGet {
		utils.WriteError(w, http.StatusMethodNotAllowed, utils.ErrCodeMethodNotAllowed, "Method not allowed")
		return
	}

	var targetURL string
	if len(r.URL.Path) > len("/r/") {
		targetURL = r.URL.Path[len("/r/"):]
	} else {
		targetURL = r.URL.Query().Get("url")
	}

	if targetURL == "" {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeMissingParam, "url query parameter or path segment is required")
		return
	}

	bypassCache := r.URL.Query().Get("bypass_cache") == "true"
	scraped, _ := scraper.ScrapeSingleURL(targetURL, "markdown", bypassCache)

	acceptHeader := r.Header.Get("Accept")
	if strings.Contains(acceptHeader, "application/json") || r.URL.Query().Get("json") == "true" {
		w.Header().Set("Content-Type", "application/json")
		if scraped.Error != "" {
			w.WriteHeader(http.StatusGatewayTimeout)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":   http.StatusGatewayTimeout,
				"status": "failed",
				"error":  scraped.Error,
				"data": map[string]interface{}{
					"url": targetURL,
				},
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":   200,
			"status": "success",
			"data": map[string]interface{}{
				"title":    scraped.Title,
				"url":      scraped.URL,
				"content":  scraped.Markdown,
				"raw":      scraped.Content,
				"usage": map[string]interface{}{
					"tokens": scraped.WordCount,
				},
			},
		})
	} else {
		if scraped.Error != "" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusGatewayTimeout)
			fmt.Fprintf(w, "Error scraping page: %s", scraped.Error)
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, scraped.Markdown)
	}
}
