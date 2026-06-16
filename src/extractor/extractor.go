package extractor

import (
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// PageMetadata holds all elements scraped and parsed from an HTML document.
type PageMetadata struct {
	Title            string
	CanonicalURL     string
	Description      string
	Author           string
	PublishedAt      *time.Time
	Language         string
	OutboundLinks    []string
}

// ParseMetadata extracts SEO, metadata, JSON-LD, and outbound links from an HTML document.
func ParseMetadata(htmlContent string, targetURL string, finalURL string) PageMetadata {
	var meta PageMetadata
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
func extractFromJSONLD(val interface{}, meta *PageMetadata) {
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
