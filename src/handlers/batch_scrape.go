package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"src/models"
	"src/scraper"
	"src/utils"
)

// BatchScrapeHandler processes multiple URLs concurrently.
func BatchScrapeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, utils.ErrCodeMethodNotAllowed, "Method not allowed")
		return
	}

	var req models.BatchScrapeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeInvalidJSON, "Invalid JSON")
		return
	}
	if len(req.URLs) == 0 {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeMissingParam, "URLs array is required")
		return
	}

	maxConcurrent := 20
	if len(req.URLs) < maxConcurrent {
		maxConcurrent = len(req.URLs)
	}

	results := make([]models.ScrapeResult, len(req.URLs))
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for i, u := range req.URLs {
		wg.Add(1)
		go func(index int, targetURL string) {
			defer wg.Done()
			
			defer func() {
				if r := recover(); r != nil {
					results[index] = models.ScrapeResult{
						URL:       targetURL,
						StartTime: time.Now().UTC().Format(time.RFC3339),
						EndTime:   time.Now().UTC().Format(time.RFC3339),
						Error:     fmt.Sprintf("panic recovered: %v", r),
					}
				}
			}()

			sem <- struct{}{}
			defer func() { <-sem }()

			scraped, _ := scraper.ScrapeSingleURL(targetURL, req.Format, req.BypassCache)
			results[index] = scraped
		}(i, u)
	}

	wg.Wait()
	utils.WriteSuccess(w, results)
}
