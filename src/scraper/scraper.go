package scraper

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown/v2"
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
}

func stripHTMLTagsForMarkdown(htmlStr string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil || doc == nil {
		return htmlStr
	}
	doc.Find("table, img, pre, code").Remove()
	html, err := doc.Html()
	if err != nil || html == "" {
		return htmlStr
	}
	return html
}

// ScrapeHTMLContent extracts structural text from HTML content directly.
func ScrapeHTMLContent(htmlContent string, targetURL string, finalURL string, format string, startTime time.Time) models.ScrapeResult {
	return ScrapeHTMLContentWithSchema(htmlContent, targetURL, finalURL, format, startTime, "")
}

// ScrapeHTMLContentWithSchema extracts structural text and optionally runs schema-guided structured extraction.
func ScrapeHTMLContentWithSchema(htmlContent string, targetURL string, finalURL string, format string, startTime time.Time, extractSchema string) models.ScrapeResult {
	startISO := startTime.UTC().Format(time.RFC3339)
	result := models.ScrapeResult{URL: targetURL, StartTime: startISO}

	if format == "" {
		format = "markdown"
	}

	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		parsedURL = &url.URL{}
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

	// Convert tables to markdown before main text extraction
	tableCleanedHTML := ConvertTablesToMarkdown(htmlContent)

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
			cleanedHTML := stripHTMLTagsForMarkdown(article.Content)
			if markdownStr, merr := htmlToMarkdown(cleanedHTML, parsedURL.String()); merr == nil {
				result.Markdown = markdownStr
			} else {
				result.Markdown = plainText
			}
		}

		result.EndTime = time.Now().UTC().Format(time.RFC3339)
		result.Duration = time.Since(startTime).Milliseconds()
		result.FetchDurationMS = int(result.Duration)
		result.ExtractionMethod = "readability"
		result.Scraped = true
		result.ContentHash = computeContentHash(result.Content)
		if extractSchema != "" {
			if structured, err := extractStructuredData(extractSchema, result); err == nil {
				result.StructuredData = structured
			}
		}
		return result
	}

	// 4. Strategy 2: goquery fallback
	doc, docErr := goquery.NewDocumentFromReader(strings.NewReader(tableCleanedHTML))
	if docErr != nil || doc == nil {
		result.Error = fmt.Sprintf("parse failed: %v", docErr)
		result.EndTime = time.Now().UTC().Format(time.RFC3339)
		result.Duration = time.Since(startTime).Milliseconds()
		result.FetchDurationMS = int(result.Duration)
		result.ExtractionMethod = "failed"
		result.Scraped = false
		return result
	}

	cleanDoc := doc
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
		cleanedHTML, _ := cleanDoc.Find("body").Html()
		if markdownStr, merr := htmlToMarkdown(cleanedHTML, parsedURL.String()); merr == nil {
			result.Markdown = markdownStr
		} else {
			result.Markdown = content
		}
	}

	result.EndTime = time.Now().UTC().Format(time.RFC3339)
	result.Duration = time.Since(startTime).Milliseconds()
	result.FetchDurationMS = int(result.Duration)
	result.ExtractionMethod = "goquery"
	result.ContentHash = computeContentHash(result.Content)
	if extractSchema != "" {
		if structured, err := extractStructuredData(extractSchema, result); err == nil {
			result.StructuredData = structured
		}
	}
	return result
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
	res, raw := ScrapeSingleURLWithOptions(targetURL, ScrapeOptions{Format: format, BypassCache: bypassCache})
	return res, raw
}

// ScrapeSingleURLWithOptions scrapes a single URL using optional schema extraction and native-render overrides.
func ScrapeSingleURLWithOptions(targetURL string, opts ScrapeOptions) (models.ScrapeResult, string) {
	res, raw := scrapeSingleURLInternal(targetURL, opts)
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
	res, raw := scrapeSingleURLInternal(targetURL, ScrapeOptions{Format: format, BypassCache: bypassCache, ForceNative: true})
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
	startTime := time.Now()
	startISO := startTime.UTC().Format(time.RFC3339)

	// 1. Check database cache
	if !opts.BypassCache {
		if cached, found := db.GetScrapeCache(targetURL); found {
			cached.Duration = time.Since(startTime).Milliseconds()
			cached.Cached = true
			populateScrapeMetadata(&cached)
			return cached, ""
		}
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
			result.Error = "disallowed by robots.txt"
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

	// 2. Try Lightpanda Scraper if enabled and native mode is not forced
	if !opts.ForceNative {
		if enabled, binaryPath := utils.LoadLightpandaConfig(); enabled && binaryPath != "" {
			scraped, rawOut, err := ScrapeWithLightpanda(targetURL, userAgent, binaryPath, opts.Format, startTime, opts.ExtractSchema)
			if err == nil {
				db.SaveScrapeCache(scraped)
				return scraped, rawOut
			}
			log.Printf("[Scraper] Lightpanda failed: %v. Falling back to native Go scraper...", err)
		}
	}

	htmlContent, parsedURL, statusCode, contentType, err := FetchHTML(targetURL, userAgent)
	if err != nil {
		result.Error = err.Error()
		if parsedURL != nil {
			result.Domain = parsedURL.Hostname()
		} else if parsed, pErr := url.Parse(targetURL); pErr == nil {
			result.Domain = parsed.Hostname()
		}
		result.StatusCode = statusCode
		result.ContentType = contentType
		result.Scraped = false
		result.ExtractionMethod = "failed"
		result.EndTime = time.Now().UTC().Format(time.RFC3339)
		result.Duration = time.Since(startTime).Milliseconds()
		result.FetchDurationMS = int(result.Duration)
		db.SaveScrapeCache(result)
		return result, ""
	}

	finalURL := targetURL
	if parsedURL != nil {
		finalURL = parsedURL.String()
	}

	prevCached, prevFound := db.GetScrapeCache(targetURL)

	scraped := ScrapeHTMLContentWithSchema(htmlContent, targetURL, finalURL, opts.Format, startTime, opts.ExtractSchema)
	scraped.StatusCode = statusCode
	scraped.ContentType = contentType

	if prevFound && prevCached.ContentHash != "" && scraped.ContentHash != "" {
		scraped.ContentChanged = (scraped.ContentHash != prevCached.ContentHash)
	}

	db.SaveScrapeCache(scraped)

	return scraped, htmlContent
}

func htmlToMarkdown(htmlContent string, baseURL string) (string, error) {
	return md.ConvertString(htmlContent)
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
