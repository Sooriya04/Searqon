package handlers

import (
	"net/http"
	"strings"
	"time"

	"src/search"
	"src/utils"
)

// SearchExplainHandler analyzes and explains query decomposition, intent, discovery, and ranking breakdown.
func SearchExplainHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if strings.TrimSpace(query) == "" {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeMissingParam, "Query parameter 'q' is required")
		return
	}

	start := time.Now()
	intent := search.ClassifyIntent(query)
	expanded := search.ExpandQuery(query)

	res := search.RunSearchPipeline(query, 5, true, false, 0, false, "")

	var explanation []map[string]interface{}
	for idx, item := range res.Results {
		explanation = append(explanation, map[string]interface{}{
			"rank":        idx + 1,
			"title":       item.Title,
			"url":         item.URL,
			"source":      item.Source,
			"score":       item.Score,
			"scraped":      item.Scraped,
			"scrape_err":  item.ScrapeError,
		})
	}

	utils.WriteSuccess(w, map[string]interface{}{
		"query":          query,
		"intent":         intent,
		"expanded_query": expanded,
		"provider":       res.Provider,
		"total_results":  res.Total,
		"duration_ms":    time.Since(start).Milliseconds(),
		"rank_breakdown": explanation,
	})
}
