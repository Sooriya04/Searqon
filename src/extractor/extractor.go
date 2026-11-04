package extractor

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"src/models"
)

// GenerateSlugAnchor converts heading text into a URL slug anchor.
func GenerateSlugAnchor(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	var sb strings.Builder
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			sb.WriteRune(r)
		} else if r == ' ' || r == '	' || r == '\n' {
			sb.WriteRune('-')
		}
	}
	slug := sb.String()
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	return strings.Trim(slug, "-")
}

// ExtractHeadingsAndOutline extracts H1-H6 headings with anchors and builds a table of contents outline.
func ExtractHeadingsAndOutline(doc *goquery.Document) ([]models.Heading, string) {
	var headings []models.Heading
	var outlineBuilder strings.Builder
	outlineBuilder.WriteString("### Table of Contents\n")

	doc.Find("h1, h2, h3, h4, h5, h6").Each(func(i int, s *goquery.Selection) {
		tagName := ""
		if len(s.Nodes) > 0 && s.Nodes[0] != nil {
			tagName = strings.ToLower(s.Nodes[0].Data)
		}
		level := 1
		if len(tagName) == 2 && tagName[0] == 'h' {
			if l := int(tagName[1] - '0'); l >= 1 && l <= 6 {
				level = l
			}
		}
		text := strings.TrimSpace(s.Text())
		if text == "" {
			return
		}
		anchor := GenerateSlugAnchor(text)
		headings = append(headings, models.Heading{
			Level:  level,
			Text:   text,
			Anchor: anchor,
		})

		indent := strings.Repeat("  ", level-1)
		outlineBuilder.WriteString(fmt.Sprintf("%s- [%s](#%s)\n", indent, text, anchor))
	})

	outline := ""
	if len(headings) > 0 {
		outline = outlineBuilder.String()
	}
	return headings, outline
}

// EstimateTokens calculates approximate token count (~4 characters per token).
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return (len(text) + 3) / 4
}

// CleanTruncateMarkdown truncates markdown at paragraph, heading, or sentence boundaries without cutting mid-sentence/code block.
func CleanTruncateMarkdown(markdown string, maxTokens int) (string, bool) {
	if maxTokens <= 0 {
		return markdown, false
	}
	maxChars := maxTokens * 4
	if len(markdown) <= maxChars {
		return markdown, false
	}

	truncated := markdown[:maxChars]

	// 1. Try paragraph boundary \n\n in second half
	if idx := strings.LastIndex(truncated, "\n\n"); idx > maxChars/2 {
		return strings.TrimSpace(markdown[:idx]), true
	}
	// 2. Try heading boundary \n#
	if idx := strings.LastIndex(truncated, "\n#"); idx > maxChars/2 {
		return strings.TrimSpace(markdown[:idx]), true
	}
	// 3. Try sentence boundary . 
	if idx := strings.LastIndex(truncated, ". "); idx > maxChars/2 {
		return strings.TrimSpace(markdown[:idx+1]), true
	}
	// 4. Fallback to last space
	if idx := strings.LastIndex(truncated, " "); idx > maxChars/2 {
		return strings.TrimSpace(markdown[:idx]), true
	}

	return strings.TrimSpace(truncated), true
}

// ParseMetadata extracts SEO, metadata, JSON-LD, and outbound links from an HTML document.
func ParseMetadata(htmlContent string, targetURL string, finalURL string) models.DocumentMetadata {
	var meta models.DocumentMetadata
	meta.CanonicalURL = finalURL
	if meta.CanonicalURL == "" {
		meta.CanonicalURL = targetURL
	}

	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		parsedURL = &url.URL{}
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil || doc == nil {
		return meta
	}

	// 1. Title tag fallback
	meta.Title = strings.TrimSpace(doc.Find("title").First().Text())
	if meta.Title == "" {
		meta.Title = strings.TrimSpace(doc.Find("h1").First().Text())
	}

	// 2. Canonical URL tag
	if canonical, ok := doc.Find("link[rel='canonical']").Attr("href"); ok {
		canonical = strings.TrimSpace(canonical)
		if canonical != "" {
			// Resolve relative canonical if necessary
			if u, pErr := url.Parse(canonical); pErr == nil {
				if !u.IsAbs() && parsedURL.Scheme != "" {
					meta.CanonicalURL = parsedURL.ResolveReference(u).String()
				} else {
					meta.CanonicalURL = canonical
				}
			}
		}
	}

	// 3. Description
	if desc, ok := doc.Find("meta[name='description']").Attr("content"); ok {
		meta.Description = strings.TrimSpace(desc)
	}
	if meta.Description == "" {
		if desc, ok := doc.Find("meta[property='og:description']").Attr("content"); ok {
			meta.Description = strings.TrimSpace(desc)
		}
	}

	// 4. Language
	if lang, ok := doc.Find("html").Attr("lang"); ok {
		meta.Language = strings.TrimSpace(lang)
	}
	if meta.Language == "" {
		if lang, ok := doc.Find("meta[http-equiv='content-language']").Attr("content"); ok {
			meta.Language = strings.TrimSpace(lang)
		}
	}

	// 5. Outbound Links (resolved to absolute and deduped)
	linkMap := make(map[string]bool)
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if !ok {
			return
		}
		href = strings.TrimSpace(href)
		if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") {
			return
		}
		if u, pErr := url.Parse(href); pErr == nil {
			resolved := u
			if !u.IsAbs() && parsedURL.Scheme != "" {
				resolved = parsedURL.ResolveReference(u)
			}
			absURL := resolved.String()
			if strings.HasPrefix(absURL, "http://") || strings.HasPrefix(absURL, "https://") {
				linkMap[absURL] = true
			}
		}
	})
	for link := range linkMap {
		meta.OutboundLinks = append(meta.OutboundLinks, link)
	}

	// 6. Meta Author
	if aut, ok := doc.Find("meta[name='author']").Attr("content"); ok {
		meta.Author = strings.TrimSpace(aut)
	}
	if meta.Author == "" {
		if aut, ok := doc.Find("meta[property='article:author']").Attr("content"); ok {
			meta.Author = strings.TrimSpace(aut)
		}
	}
	if meta.Author == "" {
		if aut, ok := doc.Find("meta[name='twitter:creator']").Attr("content"); ok {
			meta.Author = strings.TrimSpace(aut)
		}
	}

	// 7. Meta Published Time
	var pubStr string
	if pub, ok := doc.Find("meta[property='article:published_time']").Attr("content"); ok {
		pubStr = pub
	}
	if pubStr == "" {
		if pub, ok := doc.Find("meta[name='pubdate']").Attr("content"); ok {
			pubStr = pub
		}
	}
	if pubStr == "" {
		if pub, ok := doc.Find("meta[name='publish-date']").Attr("content"); ok {
			pubStr = pub
		}
	}
	if pubStr != "" {
		meta.PublishedAt = parseDateString(pubStr)
	}

	// 8. JSON-LD parsing fallback for author and publication date
	doc.Find("script[type='application/ld+json']").Each(func(i int, s *goquery.Selection) {
		jsld := strings.TrimSpace(s.Text())
		if jsld == "" {
			return
		}
		var parsed interface{}
		if err := json.Unmarshal([]byte(jsld), &parsed); err != nil {
			return
		}
		extractFromJSONLD(parsed, &meta)
	})

	return meta
}

// parseDateString tries multiple date formats to parse time strings.
func parseDateString(str string) *time.Time {
	str = strings.TrimSpace(str)
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
		time.RFC1123,
		time.RFC1123Z,
		time.RubyDate,
	}
	for _, fmtStr := range formats {
		if t, err := time.Parse(fmtStr, str); err == nil {
			return &t
		}
	}
	return nil
}

// extractFromJSONLD recursively traverses JSON-LD data looking for publication dates and author info.
func extractFromJSONLD(val interface{}, meta *models.DocumentMetadata) {
	switch m := val.(type) {
	case map[string]interface{}:
		// Date published
		if dp, ok := m["datePublished"].(string); ok && dp != "" {
			if t := parseDateString(dp); t != nil && meta.PublishedAt == nil {
				meta.PublishedAt = t
			}
		}

		// Author name
		if auth, ok := m["author"]; ok {
			switch a := auth.(type) {
			case string:
				if meta.Author == "" {
					meta.Author = strings.TrimSpace(a)
				}
			case map[string]interface{}:
				if name, ok := a["name"].(string); ok && name != "" {
					if meta.Author == "" {
						meta.Author = strings.TrimSpace(name)
					}
				}
			case []interface{}:
				if len(a) > 0 {
					if first, ok := a[0].(map[string]interface{}); ok {
						if name, ok := first["name"].(string); ok && name != "" {
							if meta.Author == "" {
								meta.Author = strings.TrimSpace(name)
							}
						}
					} else if firstStr, ok := a[0].(string); ok {
						if meta.Author == "" {
							meta.Author = strings.TrimSpace(firstStr)
						}
					}
				}
			}
		}

		// Traverse child fields/objects
		for _, v := range m {
			extractFromJSONLD(v, meta)
		}

	case []interface{}:
		for _, item := range m {
			extractFromJSONLD(item, meta)
		}
	}
}
