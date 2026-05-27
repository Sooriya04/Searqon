package main

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ─── Noise Removal Selectors ─────────────────────────────────────────────────

var noiseSelectors = []string{
	"script", "style", "noscript", "iframe", "svg",
	"nav", "header", "footer", "aside",
	"form", "button", "input", "select", "textarea",
	".ad", ".ads", ".advert", ".advertisement",
	".sidebar", ".side-bar",
	".nav", ".navbar", ".navigation", ".menu",
	".footer", ".header",
	".cookie", ".cookie-banner", ".cookie-notice",
	".popup", ".modal", ".overlay",
	".social", ".share", ".sharing",
	".comment", ".comments",
	".related", ".recommended",
	".breadcrumb", ".breadcrumbs",
	"[role='navigation']", "[role='banner']", "[role='contentinfo']",
	"[role='complementary']", "[aria-hidden='true']",
}

// ─── Text Cleaning ───────────────────────────────────────────────────────────

func cleanText(text string) string {
	boxChars := []string{"┌", "┬", "┐", "├", "┼", "┤", "└", "┴", "┘", "│", "─", "━", "┏", "┳", "┓", "┣", "╋", "┫", "┗", "┻", "┛", "┃"}
	for _, char := range boxChars {
		text = strings.ReplaceAll(text, char, "")
	}

	lines := strings.Split(text, "\n")
	var cleaned []string
	prevEmpty := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		for strings.HasPrefix(line, "#") {
			line = strings.TrimSpace(strings.TrimLeft(line, "#"))
		}
		for strings.Contains(line, "  ") {
			line = strings.ReplaceAll(line, "  ", " ")
		}
		if line == "" {
			if !prevEmpty {
				cleaned = append(cleaned, "")
				prevEmpty = true
			}
			continue
		}
		prevEmpty = false
		cleaned = append(cleaned, line)
	}

	result := strings.Join(cleaned, " ")
	result = strings.ReplaceAll(result, "*", "")
	result = strings.ReplaceAll(result, "\n", " ")
	for strings.Contains(result, "  ") {
		result = strings.ReplaceAll(result, "  ", " ")
	}
	return strings.TrimSpace(result)
}

func countWords(text string) int {
	return len(strings.Fields(text))
}

func extractTitleFromHTML(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "No Title"
	}
	title := strings.TrimSpace(doc.Find("title").First().Text())
	if title == "" {
		title = strings.TrimSpace(doc.Find("h1").First().Text())
	}
	if title == "" {
		title = "No Title"
	}
	return title
}
