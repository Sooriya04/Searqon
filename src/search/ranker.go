package search

import (
	"math"
	"net/url"
	"sort"
	"strings"

	"src/models"
	"src/utils"
)

const (
	weightTitle   = 4.0
	weightSnippet = 2.0
	weightContent = 1.5

	boostAuthoritative = 0.15
	boostWikipedia     = 0.20

	positionDecayFactor = 0.03
	contentQualityMin   = 50
)

var authoritativeTLDs = []string{".edu", ".gov", ".org"}

var authoritativeDomains = map[string]float64{
	"wikipedia.org":         boostWikipedia,
	"en.wikipedia.org":      boostWikipedia,
	"stackoverflow.com":     boostAuthoritative,
	"stackexchange.com":     boostAuthoritative,
	"github.com":            boostAuthoritative,
	"arxiv.org":             boostAuthoritative,
	"docs.python.org":       boostAuthoritative,
	"developer.mozilla.org": boostAuthoritative,
	"go.dev":                boostAuthoritative,
	"rust-lang.org":         boostAuthoritative,
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	for _, ch := range []string{".", ",", "!", "?", ":", ";", "(", ")", "[", "]", "{", "}", "\"", "'", "`", "/", "\\", "|", "<", ">", "-", "_"} {
		text = strings.ReplaceAll(text, ch, " ")
	}
	return strings.Fields(text)
}

func queryTerms(query string) []string {
	raw := tokenize(query)
	stopWords := map[string]bool{
		"a": true, "an": true, "the": true, "is": true, "it": true,
		"in": true, "on": true, "at": true, "to": true, "of": true,
		"for": true, "and": true, "or": true, "but": true, "not": true,
		"with": true, "from": true, "by": true, "as": true, "this": true,
		"that": true, "are": true, "was": true, "were": true, "be": true,
		"been": true, "being": true, "have": true, "has": true, "had": true,
		"do": true, "does": true, "did": true, "will": true, "would": true,
		"can": true, "could": true, "should": true, "may": true, "might": true,
		"what": true, "how": true, "which": true, "who": true, "where": true,
		"when": true, "why": true,
	}

	var terms []string
	for _, t := range raw {
		if len(t) >= 2 && !stopWords[t] {
			terms = append(terms, t)
		}
	}
	if len(terms) == 0 {
		return raw
	}
	return terms
}

func termFrequencyScore(text string, terms []string) float64 {
	if len(terms) == 0 || text == "" {
		return 0
	}

	textLower := strings.ToLower(text)
	textTokens := tokenize(text)
	totalTokens := float64(len(textTokens))
	if totalTokens == 0 {
		return 0
	}

	var matchedTerms int
	var totalTF float64

	for _, term := range terms {
		count := strings.Count(textLower, term)
		if count > 0 {
			matchedTerms++
			tf := 1.0 + math.Log(float64(count))
			totalTF += tf
		}
	}

	coverage := float64(matchedTerms) / float64(len(terms))
	density := totalTF / (float64(len(terms)) * (1.0 + math.Log(totalTokens)))

	return math.Min(1.0, coverage*0.7+density*0.3)
}

func exactPhraseBonus(text string, query string) float64 {
	if strings.Contains(strings.ToLower(text), strings.ToLower(query)) {
		return 0.15
	}
	return 0
}

func domainBoost(rawURL string) float64 {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return 0
	}

	host := strings.ToLower(parsed.Hostname())

	for domain, boost := range authoritativeDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return boost
		}
	}

	for _, tld := range authoritativeTLDs {
		if strings.HasSuffix(host, tld) {
			return boostAuthoritative * 0.7
		}
	}

	return 0
}

func contentQualityScore(result models.SearchResult) float64 {
	if !result.Scraped || result.Content == "" {
		return 0
	}

	words := utils.CountWords(result.Content)
	if words < contentQualityMin {
		return 0.05
	}

	return math.Min(0.15, 0.05+0.1*math.Log(float64(words)/float64(contentQualityMin)))
}

func rankResults(results []models.SearchResult, query string) []models.SearchResult {
	if len(results) <= 1 {
		return results
	}

	terms := queryTerms(query)
	type scored struct {
		result models.SearchResult
		score  float64
	}

	scoredResults := make([]scored, len(results))

	for i, r := range results {
		titleScore := termFrequencyScore(r.Title, terms)
		snippetScore := termFrequencyScore(r.Snippet, terms)

		var contentScore float64
		if r.Content != "" {
			contentScore = termFrequencyScore(r.Content, terms)
		}

		relevance := (titleScore * weightTitle) +
			(snippetScore * weightSnippet) +
			(contentScore * weightContent)

		maxPossible := weightTitle + weightSnippet + weightContent
		relevance = relevance / maxPossible

		relevance += exactPhraseBonus(r.Title, query)
		relevance += exactPhraseBonus(r.Snippet, query) * 0.5
		relevance += domainBoost(r.URL)
		relevance += contentQualityScore(r)

		positionBonus := positionDecayFactor * math.Max(0, float64(len(results)-i)) / float64(len(results))
		relevance += positionBonus

		scoredResults[i] = scored{result: r, score: relevance}
	}

	sort.SliceStable(scoredResults, func(i, j int) bool {
		return scoredResults[i].score > scoredResults[j].score
	})

	ranked := make([]models.SearchResult, len(scoredResults))
	for i, s := range scoredResults {
		ranked[i] = s.result
		ranked[i].Score = math.Round(s.score*1000) / 1000
	}

	return ranked
}

// HybridRankResults applies lexical scoring first, then semantic reranking if embeddings are available.
func HybridRankResults(results []models.SearchResult, query string) []models.SearchResult {
	if len(results) <= 1 {
		return results
	}
	results = rankResults(results, query)
	return semanticRankResults(results, query)
}
