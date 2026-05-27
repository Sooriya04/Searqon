package main

import (
	"compress/flate"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/brotli"
	readability "github.com/go-shiori/go-readability"
)

// ─── Stealth HTTP Client ─────────────────────────────────────────────────────

var httpClient = &http.Client{
	Timeout: 8 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

var defaultHeaders = map[string]string{
	"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
	"Accept-Language":           "en-US,en;q=0.9",
	"Cache-Control":             "no-cache",
	"Sec-Fetch-Dest":            "document",
	"Sec-Fetch-Mode":            "navigate",
	"Sec-Fetch-Site":            "none",
	"Sec-Fetch-User":            "?1",
	"Upgrade-Insecure-Requests": "1",
}

// ─── HTML Fetch Helper ───────────────────────────────────────────────────────

func fetchHTML(targetURL string) (string, *url.URL, int, error) {
	parsedURL, err := url.Parse(targetURL)
	if err != nil || parsedURL.Scheme == "" {
		return "", nil, 0, fmt.Errorf("invalid URL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return "", nil, 0, fmt.Errorf("request creation failed: %v", err)
	}
	for key, value := range defaultHeaders {
		req.Header.Set(key, value)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", nil, 0, fmt.Errorf("fetch failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", nil, resp.StatusCode, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if contentType != "" && !strings.Contains(contentType, "text/") &&
		!strings.Contains(contentType, "xml") && !strings.Contains(contentType, "json") {
		return "", nil, resp.StatusCode, fmt.Errorf("binary content-type: %s", contentType)
	}

	var reader io.Reader = resp.Body
	switch strings.ToLower(resp.Header.Get("Content-Encoding")) {
	case "gzip":
		gzReader, err := gzip.NewReader(resp.Body)
		if err == nil {
			defer gzReader.Close()
			reader = gzReader
		}
	case "br":
		reader = brotli.NewReader(resp.Body)
	case "deflate":
		reader = flate.NewReader(resp.Body)
	}

	body, err := io.ReadAll(io.LimitReader(reader, 5*1024*1024))
	if err != nil {
		return "", nil, 0, fmt.Errorf("read failed: %v", err)
	}

	return string(body), parsedURL, resp.StatusCode, nil
}

// ─── Core Scraping Function ──────────────────────────────────────────────────

func scrapeHTMLContent(htmlContent string, targetURL string, format string, startTime time.Time) ScrapeResult {
	startISO := startTime.UTC().Format(time.RFC3339)
	result := ScrapeResult{URL: targetURL, StartTime: startISO}

	if format == "" {
		format = "markdown"
	}

	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		parsedURL = &url.URL{}
	}

	// ── Strategy 1: go-readability ────────────────────────────────────────────
	article, err := readability.FromReader(strings.NewReader(htmlContent), parsedURL)
	if err == nil && len(strings.TrimSpace(article.TextContent)) > 100 {
		plainText := cleanText(article.TextContent)
		title := article.Title
		if title == "" {
			title = extractTitleFromHTML(htmlContent)
		}

		result.Title = title
		result.Content = plainText
		result.WordCount = countWords(plainText)

		if format == "markdown" {
			if markdownStr, merr := htmlToMarkdown(article.Content, parsedURL.String()); merr == nil {
				result.Markdown = markdownStr
			} else {
				result.Markdown = plainText
			}
		}

		result.EndTime = time.Now().UTC().Format(time.RFC3339)
		result.Duration = time.Since(startTime).Milliseconds()
		log.Printf("[Go Scraper] Readability+MD: %s (%d words, %dms)", targetURL, result.WordCount, result.Duration)
		return result
	}

	// ── Strategy 2: goquery fallback ──────────────────────────────────────────
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		result.Error = fmt.Sprintf("parse failed: %v", err)
		result.EndTime = time.Now().UTC().Format(time.RFC3339)
		result.Duration = time.Since(startTime).Milliseconds()
		return result
	}

	for _, selector := range noiseSelectors {
		doc.Find(selector).Remove()
	}

	title := strings.TrimSpace(doc.Find("title").First().Text())
	if title == "" {
		title = strings.TrimSpace(doc.Find("h1").First().Text())
	}

	var textParts []string
	doc.Find("body").Find("h1, h2, h3, h4, h5, h6, p, li, blockquote, pre, td, th").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if len(text) < 3 {
			return
		}
		textParts = append(textParts, text)
	})

	content := cleanText(strings.Join(textParts, " "))
	if countWords(content) < 20 {
		content = cleanText(doc.Find("body").Text())
	}

	result.Title = title
	result.Content = content
	result.WordCount = countWords(content)

	if format == "markdown" {
		cleanedHTML, _ := doc.Find("body").Html()
		if markdownStr, merr := htmlToMarkdown(cleanedHTML, parsedURL.String()); merr == nil {
			result.Markdown = markdownStr
		} else {
			result.Markdown = content
		}
	}

	result.EndTime = time.Now().UTC().Format(time.RFC3339)
	result.Duration = time.Since(startTime).Milliseconds()
	log.Printf("[Go Scraper] GoQuery+MD: %s (%d words, %dms)", targetURL, result.WordCount, result.Duration)
	return result
}

func scrapeSingleURL(targetURL string, format string) (ScrapeResult, string) {
	startTime := time.Now()
	startISO := startTime.UTC().Format(time.RFC3339)

	result := ScrapeResult{URL: targetURL, StartTime: startISO}

	htmlContent, _, _, err := fetchHTML(targetURL)
	if err != nil {
		result.Error = err.Error()
		result.EndTime = time.Now().UTC().Format(time.RFC3339)
		result.Duration = time.Since(startTime).Milliseconds()
		return result, ""
	}

	scraped := scrapeHTMLContent(htmlContent, targetURL, format, startTime)
	return scraped, htmlContent
}

// ─── HTML → Markdown ─────────────────────────────────────────────────────────

func htmlToMarkdown(htmlContent string, baseURL string) (string, error) {
	return md.ConvertString(htmlContent)
}
