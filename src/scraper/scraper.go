package scraper

import (
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/PuerkitoBio/goquery"
	readability "github.com/go-shiori/go-readability"

	"src/db"
	"src/extractor"
	"src/models"
	"src/utils"
)

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
	result.Scraped = true
	return result
}

// ScrapeSingleURL scrapes a single URL, resolving cache and robots.txt.
func ScrapeSingleURL(targetURL string, format string, bypassCache bool) (models.ScrapeResult, string) {
	return scrapeSingleURLInternal(targetURL, format, bypassCache, false)
}

// ScrapeSingleURLNative forces the use of Go's native HTTP scraper.
func ScrapeSingleURLNative(targetURL string, format string, bypassCache bool) (models.ScrapeResult, string) {
	return scrapeSingleURLInternal(targetURL, format, bypassCache, true)
}

func scrapeSingleURLInternal(targetURL string, format string, bypassCache bool, forceNative bool) (models.ScrapeResult, string) {
	startTime := time.Now()
	startISO := startTime.UTC().Format(time.RFC3339)

	// 1. Check database cache
	if !bypassCache {
		if cached, found := db.GetScrapeCache(targetURL); found {
			cached.Duration = time.Since(startTime).Milliseconds()
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
	if !forceNative {
		if enabled, binaryPath := utils.LoadLightpandaConfig(); enabled && binaryPath != "" {
			scraped, rawOut, err := ScrapeWithLightpanda(targetURL, userAgent, binaryPath, format, startTime)
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
	scraped := ScrapeHTMLContent(htmlContent, targetURL, finalURL, format, startTime)
	scraped.StatusCode = statusCode
	scraped.ContentType = contentType

	db.SaveScrapeCache(scraped)

	return scraped, htmlContent
}

func htmlToMarkdown(htmlContent string, baseURL string) (string, error) {
	return md.ConvertString(htmlContent)
}
