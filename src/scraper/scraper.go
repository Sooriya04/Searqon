package scraper

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	readability "github.com/go-shiori/go-readability"

	"src/chunker"
	"src/db"
	"src/extractor"
	"src/models"
	"src/utils"
)

type ScrapeOptions struct {
	Format        string
	BypassCache   bool
	ForceNative   bool
	ExtractSchema string
	StealthLevel  string // "auto", "fast_http", "spoofed_headers", "headless", "proxy"
	MaxTokens     int
}

var rawHTMLCache sync.Map // targetURL -> string

// ScrapeHTMLContent extracts structural text from HTML content directly.
func ScrapeHTMLContent(htmlContent string, targetURL string, finalURL string, format string, startTime time.Time) models.ScrapeResult {
	return ScrapeHTMLContentWithOptions(htmlContent, targetURL, finalURL, startTime, ScrapeOptions{Format: format})
}

// ScrapeHTMLContentWithSchema extracts structural text and optionally runs schema-guided structured extraction.
func ScrapeHTMLContentWithSchema(htmlContent string, targetURL string, finalURL string, format string, startTime time.Time, extractSchema string) models.ScrapeResult {
	return ScrapeHTMLContentWithOptions(htmlContent, targetURL, finalURL, startTime, ScrapeOptions{Format: format, ExtractSchema: extractSchema})
}

// ScrapeHTMLContentWithOptions extracts structural text, headings, outline, estimates tokens, and applies token-bounded truncation.
func ScrapeHTMLContentWithOptions(htmlContent string, targetURL string, finalURL string, startTime time.Time, opts ScrapeOptions) models.ScrapeResult {
	startISO := startTime.UTC().Format(time.RFC3339)
	result := models.ScrapeResult{URL: targetURL, StartTime: startISO}

	format := opts.Format
	if format == "" {
		format = "markdown"
	}

	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		parsedURL = &url.URL{}
	}

	// 0. Extract headings and outline upfront, and reuse the parsed doc
	docRaw, docErr := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if docErr == nil && docRaw != nil {
		headings, outline := extractor.ExtractHeadingsAndOutline(docRaw)
		result.Headings = headings
		result.Outline = outline
	}

	// 1. Extract metadata
	meta := extractor.ParseMetadata(htmlContent, targetURL, finalURL)
	result.Title = meta.Title
	result.CanonicalURL = meta.CanonicalURL
	result.Domain = parsedURL.Hostname()
	result.Description = meta.Description
	result.Author = meta.Author
	result.PublishedAt = meta.PublishedAt
	result.Language = meta.Language
	result.OutboundLinks = meta.OutboundLinks
	result.Metadata = meta

	// 2. Extract media images
	result.Images = ExtractImages(htmlContent, targetURL)

	// Convert tables to markdown on the parsed document directly if available
	var tableCleanedHTML string
	if docRaw != nil {
		ConvertTablesInDocument(docRaw)
		if h, err := docRaw.Html(); err == nil && h != "" {
			tableCleanedHTML = h
		}
	}
	if tableCleanedHTML == "" {
		tableCleanedHTML = htmlContent
	}

	// 3. Strategy 1: go-readability
	article, err := readability.FromReader(strings.NewReader(tableCleanedHTML), parsedURL)
	if err == nil && len(strings.TrimSpace(article.TextContent)) > 100 {
		plainText := utils.CleanText(article.TextContent)
		if article.Title != "" {
			result.Title = article.Title
		}

		result.Content = plainText
		result.WordCount = utils.CountWords(plainText)

		if format == "markdown" {
			maxBytes := 0
			if opts.MaxTokens > 0 {
				maxBytes = opts.MaxTokens * 4
			}
			if markdownStr, merr := htmlToMarkdownBounded(article.Content, parsedURL.String(), maxBytes); merr == nil {
				result.Markdown = markdownStr
			} else {
				result.Markdown = plainText
			}
		} else {
			result.Markdown = plainText
		}

		finalizeScrapeResult(&result, startTime, opts, "readability")
		return result
	}

	// 4. Strategy 2: goquery fallback (reuse docRaw if available, else reparse)
	var cleanDoc *goquery.Document
	if docRaw != nil {
		cleanDoc = docRaw
	} else {
		cleanDoc, docErr = goquery.NewDocumentFromReader(strings.NewReader(tableCleanedHTML))
		if docErr != nil || cleanDoc == nil {
			result.Error = fmt.Sprintf("parse failed: %v", docErr)
			result.EndTime = time.Now().UTC().Format(time.RFC3339)
			result.Duration = time.Since(startTime).Milliseconds()
			result.FetchDurationMS = int(result.Duration)
			result.ExtractionMethod = "failed"
			result.Scraped = false
			return result
		}
	}

	for _, selector := range utils.NoiseSelectors {
		cleanDoc.Find(selector).Remove()
	}

	cleanDoc.Find("table, img, pre, code").Remove()

	if result.Title == "" {
		title := strings.TrimSpace(cleanDoc.Find("title").First().Text())
		if title == "" {
			title = strings.TrimSpace(cleanDoc.Find("h1").First().Text())
		}
		result.Title = title
	}

	var textParts []string
	cleanDoc.Find("body").Find("h1, h2, h3, h4, h5, h6, p, li, blockquote, pre, td, th").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if len(text) < 3 {
			return
		}
		textParts = append(textParts, text)
	})

	content := utils.CleanText(strings.Join(textParts, " "))
	if utils.CountWords(content) < 20 {
		content = utils.CleanText(cleanDoc.Find("body").Text())
	}

	result.Content = content
	result.WordCount = utils.CountWords(content)

	if format == "markdown" {
		maxBytes := 0
		if opts.MaxTokens > 0 {
			maxBytes = opts.MaxTokens * 4
		}
		cleanedHTML, _ := cleanDoc.Find("body").Html()
		if markdownStr, merr := htmlToMarkdownBounded(cleanedHTML, parsedURL.String(), maxBytes); merr == nil {
			result.Markdown = markdownStr
		} else {
			result.Markdown = content
		}
	} else {
		result.Markdown = content
	}

	finalizeScrapeResult(&result, startTime, opts, "goquery")
	return result
}

func finalizeScrapeResult(result *models.ScrapeResult, startTime time.Time, opts ScrapeOptions, method string) {
	result.EndTime = time.Now().UTC().Format(time.RFC3339)
	result.Duration = time.Since(startTime).Milliseconds()
	result.FetchDurationMS = int(result.Duration)
	result.ExtractionMethod = method
	result.Scraped = true
	result.ContentHash = computeContentHash(result.Content)
	if result.ScrapedAt == "" {
		result.ScrapedAt = startTime.UTC().Format(time.RFC3339)
	}

	textForTokens := result.Markdown
	if textForTokens == "" {
		textForTokens = result.Content
	}
	result.TokenEstimate = extractor.EstimateTokens(textForTokens)

	maxTokens := opts.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4000 // default agent token budget
	}

	if result.TokenEstimate > maxTokens {
		truncated, isTruncated := extractor.CleanTruncateMarkdown(textForTokens, maxTokens)
		if isTruncated {
			if result.Markdown != "" {
				result.Markdown = truncated
			} else {
				result.Content = truncated
			}
			result.IsTruncated = true
			result.TokenEstimate = extractor.EstimateTokens(truncated)
		}
	}

	if opts.ExtractSchema != "" {
		if structured, err := extractStructuredData(opts.ExtractSchema, *result); err == nil {
			result.StructuredData = structured
		}
	}
}

func computeContentHash(text string) string {
	if text == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(text))
	return fmt.Sprintf("%x", sum)
}

// ScrapeSingleURL scrapes a single URL, resolving cache and robots.txt.
func ScrapeSingleURL(targetURL string, format string, bypassCache bool) (models.ScrapeResult, string) {
	return ScrapeSingleURLContext(context.Background(), targetURL, format, bypassCache)
}

func ScrapeSingleURLContext(ctx context.Context, targetURL string, format string, bypassCache bool) (models.ScrapeResult, string) {
	return ScrapeSingleURLWithOptionsContext(ctx, targetURL, ScrapeOptions{Format: format, BypassCache: bypassCache})
}

// ScrapeSingleURLWithOptions scrapes a single URL using optional schema extraction and native-render overrides.
func ScrapeSingleURLWithOptions(targetURL string, opts ScrapeOptions) (models.ScrapeResult, string) {
	return ScrapeSingleURLWithOptionsContext(context.Background(), targetURL, opts)
}

func ScrapeSingleURLWithOptionsContext(ctx context.Context, targetURL string, opts ScrapeOptions) (models.ScrapeResult, string) {
	res, raw := scrapeSingleURLInternalContext(ctx, targetURL, opts)
	if res.Scraped {
		if res.RenderMethod == "" {
			if res.ExtractionMethod == "lightpanda" {
				res.RenderMethod = "lightpanda"
			} else {
				res.RenderMethod = "go"
			}
		}
		if res.ScrapedAt == "" {
			res.ScrapedAt = res.StartTime
		}
		if len(res.Chunks) == 0 {
			textToChunk := res.Markdown
			if textToChunk == "" {
				textToChunk = res.Content
			}
			res.Chunks = chunker.ChunkMarkdown(textToChunk, res.URL, res.Title, res.ScrapedAt)
		}
	}
	return res, raw
}

// ScrapeSingleURLNative forces the use of Go's native HTTP scraper.
func ScrapeSingleURLNative(targetURL string, format string, bypassCache bool) (models.ScrapeResult, string) {
	res, raw := scrapeSingleURLInternalContext(context.Background(), targetURL, ScrapeOptions{Format: format, BypassCache: bypassCache, ForceNative: true})
	if res.Scraped {
		if res.RenderMethod == "" {
			res.RenderMethod = "go"
		}
		if res.ScrapedAt == "" {
			res.ScrapedAt = res.StartTime
		}
		if len(res.Chunks) == 0 {
			textToChunk := res.Markdown
			if textToChunk == "" {
				textToChunk = res.Content
			}
			res.Chunks = chunker.ChunkMarkdown(textToChunk, res.URL, res.Title, res.ScrapedAt)
		}
	}
	return res, raw
}

func scrapeSingleURLInternal(targetURL string, opts ScrapeOptions) (models.ScrapeResult, string) {
	return scrapeSingleURLInternalContext(context.Background(), targetURL, opts)
}

func scrapeSingleURLInternalContext(ctx context.Context, targetURL string, opts ScrapeOptions) (models.ScrapeResult, string) {
	startTime := time.Now()
	startISO := startTime.UTC().Format(time.RFC3339)

	if err := ctx.Err(); err != nil {
		se := &models.SearqonError{
			ErrorType:         models.ErrTimeout,
			StatusCode:        499,
			Message:           err.Error(),
			Retryable:         true,
			SuggestedFallback: models.FallbackNone,
		}
		return models.ScrapeResult{
			URL:             targetURL,
			StartTime:       startISO,
			EndTime:         time.Now().UTC().Format(time.RFC3339),
			Error:           err.Error(),
			StructuredError: se,
			Scraped:         false,
			ScrapedAt:       startISO,
		}, ""
	}

	// 1. Check database cache
	if !opts.BypassCache {
		if cached, found := db.GetScrapeCache(targetURL); found && cached.Scraped && cached.Error == "" {
			utils.RecordScrapeCacheHit()
			cached.Duration = time.Since(startTime).Milliseconds()
			cached.Cached = true
			populateScrapeMetadata(&cached)
			cachedRaw := ""
			if val, ok := rawHTMLCache.Load(targetURL); ok {
				cachedRaw = val.(string)
			}
			return cached, cachedRaw
		}
		utils.RecordScrapeCacheMiss()
	}

	result := models.ScrapeResult{URL: targetURL, StartTime: startISO}

	parsedBase, pErr := url.Parse(targetURL)
	var userAgent string
	var delay time.Duration
	var allowed bool = true

	if pErr == nil && parsedBase.Scheme != "" && parsedBase.Host != "" {
		robotsData := getRobotsData(parsedBase)
		userAgent, delay, allowed = findAllowedAgent(targetURL, robotsData)
		if !allowed {
			se := &models.SearqonError{
				ErrorType:         models.ErrCloudflareBlocked,
				StatusCode:        403,
				Message:           "Disallowed by robots.txt",
				Retryable:         false,
				SuggestedFallback: models.FallbackNone,
			}
			result.Error = "disallowed by robots.txt"
			result.StructuredError = se
			result.Domain = parsedBase.Hostname()
			result.CanonicalURL = targetURL
			result.StatusCode = 403
			result.Scraped = false
			result.ExtractionMethod = "failed"
			result.EndTime = time.Now().UTC().Format(time.RFC3339)
			result.Duration = time.Since(startTime).Milliseconds()
			result.FetchDurationMS = int(result.Duration)
			db.SaveScrapeCache(result)
			return result, ""
		}
		if delay > 0 {
			time.Sleep(delay)
		}
	}

	if userAgent == "" {
		userAgent = defaultHeaders["User-Agent"]
	}

	// 2. Multi-tier Smart Stealth & Anti-Bot Escalation:
	// Tier 1: Fast HTTP -> Tier 2: Anti-bot spoofed headers -> Tier 3: Lightpanda / Camoufox -> Tier 4: Residential Proxy
	scraped, rawOut, err := ExecuteSmartEscalationScrape(targetURL, userAgent, opts, startTime)
	if err != nil {
		result.Error = err.Error()
		if scraped.StructuredError != nil {
			result.StructuredError = scraped.StructuredError
		} else {
			result.StructuredError = models.ClassifyError(scraped.StatusCode, rawOut, err)
		}
		if parsed, pErr := url.Parse(targetURL); pErr == nil {
			result.Domain = parsed.Hostname()
		}
		result.StatusCode = scraped.StatusCode
		if result.StatusCode == 0 && result.StructuredError != nil {
			result.StatusCode = result.StructuredError.StatusCode
		}
		result.ContentType = scraped.ContentType
		result.Scraped = false
		result.BotDetected = scraped.BotDetected
		result.EscalationTier = scraped.EscalationTier
		result.ExtractionMethod = "failed"
		result.EndTime = time.Now().UTC().Format(time.RFC3339)
		result.Duration = time.Since(startTime).Milliseconds()
		result.FetchDurationMS = int(result.Duration)
		return result, ""
	}

	prevCached, prevFound := db.GetScrapeCache(targetURL)
	if prevFound && prevCached.ContentHash != "" && scraped.ContentHash != "" {
		scraped.ContentChanged = (scraped.ContentHash != prevCached.ContentHash)
	}

	if rawOut != "" {
		rawHTMLCache.Store(targetURL, rawOut)
	}

	db.SaveScrapeCache(scraped)
	return scraped, rawOut
}

func htmlToMarkdown(htmlContent string, baseURL string) (string, error) {
	return htmlToMarkdownBounded(htmlContent, baseURL, 0)
}

func htmlToMarkdownBounded(htmlContent string, baseURL string, maxBytes int) (string, error) {
	mdStr, _, err := ConvertHTMLToMarkdownStream(strings.NewReader(htmlContent), baseURL, maxBytes)
	if err != nil {
		return "", err
	}
	return mdStr, nil
}


func extractStructuredData(schema string, result models.ScrapeResult) (json.RawMessage, error) {
	prompt := fmt.Sprintf(`Extract the requested structured data from the page content.
Return only valid JSON. If a field is unavailable, use null or an empty array/object.

Requested schema:
%s

Page title: %s
URL: %s
Description: %s
Author: %s
Language: %s

Content:
%s`, schema, result.Title, result.URL, result.Description, result.Author, result.Language, truncateForExtraction(result.Markdown, result.Content))

	raw, err := queryOllamaJSON(prompt)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty structured extraction response")
	}
	return raw, nil
}

func truncateForExtraction(markdown string, content string) string {
	text := markdown
	if text == "" {
		text = content
	}
	if len(text) > 6000 {
		text = text[:6000]
	}
	return text
}

func populateScrapeMetadata(result *models.ScrapeResult) {
	if result == nil {
		return
	}
	if result.Metadata.Title == "" {
		result.Metadata.Title = result.Title
	}
	if result.Metadata.CanonicalURL == "" {
		result.Metadata.CanonicalURL = result.CanonicalURL
	}
	if result.Metadata.Description == "" {
		result.Metadata.Description = result.Description
	}
	if result.Metadata.Author == "" {
		result.Metadata.Author = result.Author
	}
	if result.Metadata.PublishedAt == nil {
		result.Metadata.PublishedAt = result.PublishedAt
	}
	if result.Metadata.Language == "" {
		result.Metadata.Language = result.Language
	}
	if len(result.Metadata.OutboundLinks) == 0 {
		result.Metadata.OutboundLinks = append([]string(nil), result.OutboundLinks...)
	}
}
