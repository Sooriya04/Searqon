package utils

import (
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var NoiseSelectors = []string{
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

// CleanText removes formatting and noise characters from raw text.
func CleanText(text string) string {
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

// CountWords returns the number of words in a string.
func CountWords(text string) int {
	return len(strings.Fields(text))
}

// ExtractTitleFromHTML parses HTML content to find the page title.
func ExtractTitleFromHTML(html string) string {
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

// LoadLightpandaConfig parses config.yaml to determine if Lightpanda is enabled.
func LoadLightpandaConfig() (bool, string) {
	if envVal := os.Getenv("LIGHTPANDA_ENABLED"); envVal != "" {
		return envVal == "true", os.Getenv("LIGHTPANDA_PATH")
	}

	paths := []string{
		"lightpanda/config.yaml", "../lightpanda/config.yaml",
		"lightpanda/config.yml", "../lightpanda/config.yml",
		"config.yaml", "../config.yaml",
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		var enabled bool
		path := "./lightpanda/lightpanda"

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if idx := strings.Index(trimmed, "#"); idx != -1 {
				trimmed = strings.TrimSpace(trimmed[:idx])
			}
			if trimmed == "" {
				continue
			}

			if strings.HasPrefix(trimmed, "enabled:") {
				val := strings.TrimSpace(strings.TrimPrefix(trimmed, "enabled:"))
				enabled = (val == "true" || val == "yes" || val == "1")
			}
			if strings.HasPrefix(trimmed, "path:") {
				val := strings.TrimSpace(strings.TrimPrefix(trimmed, "path:"))
				path = strings.Trim(val, `"'`)
				
				if !strings.HasPrefix(path, "/") {
					if _, err := os.Stat(path); err != nil {
						parentPath := "../" + strings.TrimPrefix(path, "./")
						if _, err := os.Stat(parentPath); err == nil {
							path = parentPath
						}
					}
				}
			}
		}
		return enabled, path
	}
	return false, ""
}
