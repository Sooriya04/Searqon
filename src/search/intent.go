package search

import (
	"strings"
)

type SearchIntent string

const (
	IntentNavigational SearchIntent = "navigational"
	IntentInformational SearchIntent = "informational"
	IntentAcademic      SearchIntent = "academic"
	IntentGeneric       SearchIntent = "generic"
)

// ClassifyIntent parses query terms to guess search intent.
func ClassifyIntent(query string) SearchIntent {
	q := strings.ToLower(query)

	academicKeywords := []string{
		"arxiv", "paper", "theorem", "doi", "research paper", "journal",
		"survey paper", "benchmark dataset", "proof", "citation",
	}

	navigationalKeywords := []string{
		"github", "reddit", "wikipedia", "twitter", "youtube", "login", "signin",
		"download", "homepage", "official site", "facebook", "linkedin",
	}

	informationalKeywords := []string{
		"how to", "what is", "why", "guide", "tutorial", "documentation",
		"install", "error", "failed", "setup", "explain", "difference between",
		"example", "tutorial", "api reference",
	}

	for _, kw := range academicKeywords {
		if strings.Contains(q, kw) {
			return IntentAcademic
		}
	}

	for _, kw := range navigationalKeywords {
		if strings.Contains(q, kw) {
			return IntentNavigational
		}
	}

	for _, kw := range informationalKeywords {
		if strings.Contains(q, kw) {
			return IntentInformational
		}
	}

	return IntentGeneric
}

// AdjustParamsByIntent modifies limit and scraping parameters based on user intent.
func AdjustParamsByIntent(intent SearchIntent, limit int, scrape bool) (int, bool) {
	if intent == IntentNavigational {
		return 1, scrape
	}
	if intent == IntentInformational && scrape {
		return limit, true
	}
	return limit, scrape
}
