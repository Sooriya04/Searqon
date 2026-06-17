package scraper

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"src/models"
)

// ExtractImages extracts image references from page HTML.
func ExtractImages(htmlContent string, baseURL string) []models.ScrapedImage {
	var images []models.ScrapedImage
	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return images
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil || doc == nil {
		return images
	}

	seen := make(map[string]bool)

	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		src, ok := s.Attr("src")
		if !ok {
			return
		}

		src = strings.TrimSpace(src)
		if src == "" || strings.HasPrefix(src, "data:") {
			return
		}

		resolvedURL := src
		if u, err := url.Parse(src); err == nil {
			if !u.IsAbs() && parsedBase.Scheme != "" {
				resolvedURL = parsedBase.ResolveReference(u).String()
			}
		}

		if seen[resolvedURL] {
			return
		}
		seen[resolvedURL] = true

		alt := strings.TrimSpace(s.AttrOr("alt", ""))
		images = append(images, models.ScrapedImage{
			URL: resolvedURL,
			Alt: alt,
		})
	})

	return images
}
