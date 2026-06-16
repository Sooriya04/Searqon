package main

import (
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/PuerkitoBio/goquery"
	readability "github.com/go-shiori/go-readability"
	"src/extractor"
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

// ─── Core Scraping Function ──────────────────────────────────────────────────

func scrapeHTMLContent(htmlContent string, targetURL string, finalURL string, format string, startTime time.Time) ScrapeResult {
	startISO := startTime.UTC().Format(time.RFC3339)
	result := ScrapeResult{URL: targetURL, StartTime: startISO}

	if format == "" {
		format = "markdown"
	}

	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		parsedURL = &url.URL{}
	}

	// ── Stage 1: Extract general page metadata using the modular extractor ──
	meta := extractor.ParseMetadata(htmlContent, targetURL, finalURL)
	result.Title = meta.Title
	result.CanonicalURL = meta.CanonicalURL
	result.Domain = parsedURL.Hostname()
	result.Description = meta.Description
	result.Author = meta.Author
	result.PublishedAt = meta.PublishedAt
	result.Language = meta.Language
	result.OutboundLinks = meta.OutboundLinks

	// ── Strategy 1: go-readability ────────────────────────────────────────────
	article, err := readability.FromReader(strings.NewReader(htmlContent), parsedURL)
	if err == nil && len(strings.TrimSpace(article.TextContent)) > 100 {
		plainText := cleanText(article.TextContent)
		if article.Title != "" {
			result.Title = article.Title
		}

		result.Content = plainText
		result.WordCount = countWords(plainText)

		if format == "markdown" {
			// Strip tables, images, and code blocks from the HTML before converting to markdown
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
		log.Printf("[Go Scraper] Readability+MD: %s (%d words, %dms)", targetURL, result.WordCount, result.Duration)
		return result
	}

	// ── Strategy 2: goquery fallback ──────────────────────────────────────────
	doc, docErr := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
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
	for _, selector := range noiseSelectors {
		cleanDoc.Find(selector).Remove()
	}

	// Strip tables, images, and code blocks from cleanDoc for content extraction and markdown conversion
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

	content := cleanText(strings.Join(textParts, " "))
	if countWords(content) < 20 {
		content = cleanText(cleanDoc.Find("body").Text())
	}

	result.Content = content
	result.WordCount = countWords(content)

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
	log.Printf("[Go Scraper] GoQuery+MD: %s (%d words, %dms)", targetURL, result.WordCount, result.Duration)
	return result
}

func scrapeSingleURL(targetURL string, format string, bypassCache bool) (ScrapeResult, string) {
	startTime := time.Now()
	startISO := startTime.UTC().Format(time.RFC3339)

	// ── Stage 0: Check Postgres cache first ────────────────────────────────────
	if !bypassCache {
		if cached, found := getScrapeCache(targetURL); found {
			log.Printf("[Scrape] [CACHE HIT] URL=%s (%d words)", targetURL, cached.WordCount)
			cached.Duration = time.Since(startTime).Milliseconds()
			return cached, ""
		}
	}

	result := ScrapeResult{URL: targetURL, StartTime: startISO}

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
			// Cache the exclusion so we don't crawl it again
			saveScrapeCache(result)
			return result, ""
		}
		if delay > 0 {
			time.Sleep(delay)
		}
	}

	if userAgent == "" {
		userAgent = defaultHeaders["User-Agent"]
	}

	// ── Stage 0.5: Try Lightpanda Scraper if enabled ──────────────────────────
	if enabled, binaryPath := loadLightpandaConfig(); enabled && binaryPath != "" {
		log.Printf("[Scrape] Attempting Lightpanda fetch: %s", targetURL)
		scraped, rawOut, err := scrapeWithLightpanda(targetURL, userAgent, binaryPath, format, startTime)
		if err == nil {
			// Save successfully scraped page to DB
			saveScrapeCache(scraped)
			return scraped, rawOut
		}
		log.Printf("[Scrape] Lightpanda failed: %v. Falling back to native Go scraper...", err)
	}

	htmlContent, parsedURL, statusCode, contentType, err := fetchHTML(targetURL, userAgent)
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
		// Cache the failure so we don't waste retry limits immediately
		saveScrapeCache(result)
		return result, ""
	}

	finalURL := targetURL
	if parsedURL != nil {
		finalURL = parsedURL.String()
	}
	scraped := scrapeHTMLContent(htmlContent, targetURL, finalURL, format, startTime)
	scraped.StatusCode = statusCode
	scraped.ContentType = contentType

	// Save successfully scraped page to DB
	saveScrapeCache(scraped)

	return scraped, htmlContent
}

// ─── HTML → Markdown ─────────────────────────────────────────────────────────

func htmlToMarkdown(htmlContent string, baseURL string) (string, error) {
	return md.ConvertString(htmlContent)
}
