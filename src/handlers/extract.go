package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"src/models"
	"src/scraper"
	"src/utils"
)

type ExtractRequest struct {
	URL         string            `json:"url"`
	Selectors   map[string]string `json:"selectors"`
	BypassCache bool              `json:"bypass_cache"`
}

type ExtractResponse struct {
	URL  string            `json:"url"`
	Data map[string]string `json:"data"`
}

// ExtractHandler parses specific elements on a scraped page using CSS selectors.
func ExtractHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, utils.ErrCodeMethodNotAllowed, "Method not allowed")
		return
	}

	var req ExtractRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeInvalidJSON, "Invalid JSON request body")
		return
	}
	if req.URL == "" {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeMissingParam, "URL is required")
		return
	}
	if len(req.Selectors) == 0 {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeMissingParam, "Selectors map is required")
		return
	}

	res, htmlContent := scraper.ScrapeSingleURL(req.URL, "text", req.BypassCache)
	if !res.Scraped || htmlContent == "" {
		if res.Scraped && res.Content != "" {
			htmlContent = res.Content
		} else {
			se := res.StructuredError
			if se == nil {
				se = models.ClassifyError(res.StatusCode, res.Content, fmt.Errorf("%s", res.Error))
			}
			if se == nil {
				se = &models.SearqonError{
					ErrorType:         models.ErrUpstreamUnavailable,
					StatusCode:        http.StatusBadGateway,
					Message:           "Failed to retrieve page content",
					Retryable:         false,
					SuggestedFallback: models.FallbackLightpanda,
				}
			}
			utils.WriteSearqonError(w, se)
			return
		}
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil || doc == nil {
		se := &models.SearqonError{
			ErrorType:         models.ErrParsingFailed,
			StatusCode:        http.StatusInternalServerError,
			Message:           "Failed to parse document HTML",
			Retryable:         false,
			SuggestedFallback: models.FallbackNone,
		}
		utils.WriteSearqonError(w, se)
		return
	}

	extracted := make(map[string]string)
	for key, selector := range req.Selectors {
		val := strings.TrimSpace(doc.Find(selector).First().Text())
		extracted[key] = val
	}

	utils.WriteSuccess(w, ExtractResponse{
		URL:  req.URL,
		Data: extracted,
	})
}
