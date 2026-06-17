package search

import (
	"strings"
)

var developerThesaurus = map[string][]string{
	"ml":       {"machine learning"},
	"ai":       {"artificial intelligence"},
	"nlp":      {"natural language processing"},
	"k8s":      {"kubernetes"},
	"db":       {"database"},
	"golang":   {"go language"},
	"js":       {"javascript"},
	"py":       {"python"},
	"rust":     {"rustlang"},
	"docker":   {"container"},
	"scrape":   {"crawler", "scraping"},
	"api":      {"endpoint", "rest"},
	"crypto":   {"cryptocurrency", "blockchain"},
	"security": {"cybersecurity", "pentest"},
}

// ExpandQuery expands standard query abbreviations.
func ExpandQuery(query string) string {
	words := strings.Fields(strings.ToLower(query))
	var expansions []string

	for _, word := range words {
		clean := strings.Trim(word, `.,!?;:"'()[]{}*`)
		if syns, found := developerThesaurus[clean]; found {
			for _, syn := range syns {
				if !strings.Contains(strings.ToLower(query), syn) {
					expansions = append(expansions, syn)
				}
			}
		}
	}

	if len(expansions) > 0 {
		return query + " (" + strings.Join(expansions, " OR ") + ")"
	}
	return query
}
