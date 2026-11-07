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

type ExtractRequest = models.ExtractRequest

type ExtractResponse struct {
	Title         string            `json:"title"`
	URL           string            `json:"url"`
	Markdown      string            `json:"markdown,omitempty"`
	TokenEstimate int               `json:"token_estimate"`
	Headings      []models.Heading  `json:"headings,omitempty"`
	Outline       string            `json:"outline,omitempty"`
	IsTruncated   bool              `json:"is_truncated"`
	Data          map[string]string `json:"data,omitempty"`
	Structured    json.RawMessage   `json:"structured_data,omitempty"`
	ContentHash   string            `json:"content_hash,omitempty"`
	FetchedAt     string            `json:"fetched_at,omitempty"`
}

// ExtractHandler parses specific elements on a scraped page using CSS selectors or structured markdown extraction.
func ExtractHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, utils.ErrCodeMethodNotAllowed, "Method not allowed")
		return
	}

	var req models.ExtractRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeInvalidJSON, "Invalid JSON request body")
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
	if maxTokens <= 0 {
		maxTokens = 4000
	}

	opts := scraper.ScrapeOptions{
		Format:        "markdown",
		BypassCache:   req.BypassCache,
		ExtractSchema: req.Schema,
		StealthLevel:  "auto",
		MaxTokens:     maxTokens,
	}

	res, rawHTML := scraper.ScrapeSingleURLWithOptions(req.URL, opts)
	if !res.Scraped && rawHTML == "" {
		if res.Content != "" {
			rawHTML = res.Content
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

	htmlToParse := rawHTML
	if strings.TrimSpace(htmlToParse) == "" {
		if res.Content != "" {
			htmlToParse = res.Content
		} else if res.Markdown != "" {
			htmlToParse = res.Markdown
		}
	}

	var doc *goquery.Document
	if strings.TrimSpace(htmlToParse) != "" {
		doc, _ = goquery.NewDocumentFromReader(strings.NewReader(htmlToParse))
	}

	extracted := make(map[string]string)
	if len(req.Selectors) > 0 && doc != nil {
		for key, selector := range req.Selectors {
			val := strings.TrimSpace(doc.Find(selector).First().Text())
			extracted[key] = val
		}
	}

	utils.WriteSuccess(w, ExtractResponse{
		Title:         res.Title,
		URL:           req.URL,
		Markdown:      res.Markdown,
		TokenEstimate: res.TokenEstimate,
		Headings:      res.Headings,
		Outline:       res.Outline,
		IsTruncated:   res.IsTruncated,
		Data:          extracted,
		Structured:    res.StructuredData,
		ContentHash:   res.ContentHash,
		FetchedAt:     res.ScrapedAt,
	})
}
