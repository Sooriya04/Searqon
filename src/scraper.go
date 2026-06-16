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
	"os/exec"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/PuerkitoBio/goquery"
	"github.com/andybalholm/brotli"
	readability "github.com/go-shiori/go-readability"
)

// ─── Stealth HTTP Client ─────────────────────────────────────────────────────

var httpClient = &http.Client{
	Timeout: 3 * time.Second, // fast-fail: blocked/slow pages fall back to snippets
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

var defaultHeaders = map[string]string{
	"User-Agent":                "SearqonBot/1.0 (+https://sooriya04.github.io/Searqon/)",
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

func fetchHTML(targetURL string, userAgent string) (string, *url.URL, int, error) {
	parsedURL, err := url.Parse(targetURL)
	if err != nil || parsedURL.Scheme == "" {
		return "", nil, 0, fmt.Errorf("invalid URL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return "", nil, 0, fmt.Errorf("request creation failed: %v", err)
	}
	for key, value := range defaultHeaders {
		req.Header.Set(key, value)
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
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
			result.EndTime = time.Now().UTC().Format(time.RFC3339)
			result.Duration = time.Since(startTime).Milliseconds()
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

	htmlContent, _, _, err := fetchHTML(targetURL, userAgent)
	if err != nil {
		result.Error = err.Error()
		result.EndTime = time.Now().UTC().Format(time.RFC3339)
		result.Duration = time.Since(startTime).Milliseconds()
		// Cache the failure so we don't waste retry limits immediately
		saveScrapeCache(result)
		return result, ""
	}

	scraped := scrapeHTMLContent(htmlContent, targetURL, format, startTime)

	// Save successfully scraped page to DB
	saveScrapeCache(scraped)

	return scraped, htmlContent
}

// scrapeWithLightpanda runs the Lightpanda CLI to execute JS and fetch markdown.
func scrapeWithLightpanda(targetURL string, userAgent string, binaryPath string, format string, startTime time.Time) (ScrapeResult, string, error) {
	startISO := startTime.UTC().Format(time.RFC3339)
	result := ScrapeResult{URL: targetURL, StartTime: startISO}

	// Fetch targetURL evaluating JavaScript, stripping script/ui/css, and dumping markdown.
	// We use networkalmostidle so it finishes as soon as resources load, rather than hard waiting.
	args := []string{
		"fetch", targetURL,
		"--dump", "markdown",
		"--wait-until", "networkalmostidle",
		"--strip-mode", "js,ui,css",
	}

	if userAgent != "" {
		args = append(args, "--user-agent", userAgent)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		return result, "", fmt.Errorf("lightpanda error: %v, output: %s", err, string(outputBytes))
	}

	markdownOutput := string(outputBytes)
	if strings.TrimSpace(markdownOutput) == "" {
		return result, "", fmt.Errorf("empty output from lightpanda")
	}

	// Attempt to extract title from the first heading in the markdown
	title := "Scraped Page"
	lines := strings.Split(markdownOutput, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			title = strings.TrimPrefix(trimmed, "# ")
			break
		}
	}

	// Clean markdown formatting to produce standard plain text content
	plainText := markdownToPlainText(markdownOutput)

	result.Title = title
	result.Content = plainText
	result.Markdown = markdownOutput
	result.WordCount = countWords(plainText)
	result.EndTime = time.Now().UTC().Format(time.RFC3339)
	result.Duration = time.Since(startTime).Milliseconds()

	log.Printf("[Lightpanda] Successfully scraped page (%d words, %dms)", result.WordCount, result.Duration)
	return result, markdownOutput, nil
}

// markdownToPlainText removes common markdown symbols to construct standard clean content.
func markdownToPlainText(md string) string {
	lines := strings.Split(md, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Strip headings
		line = strings.TrimLeft(line, "#")
		line = strings.TrimSpace(line)
		// Strip bold/italic/links
		line = strings.ReplaceAll(line, "**", "")
		line = strings.ReplaceAll(line, "*", "")
		line = strings.ReplaceAll(line, "`", "")
		cleaned = append(cleaned, line)
	}
	return strings.Join(cleaned, " ")
}

// ─── HTML → Markdown ─────────────────────────────────────────────────────────

func htmlToMarkdown(htmlContent string, baseURL string) (string, error) {
	return md.ConvertString(htmlContent)
}
