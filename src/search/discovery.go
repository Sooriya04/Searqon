package search

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"

	"src/models"
)

type discoveryEngine struct {
	name string
	fn   func(string, int) ([]models.SearchResult, error)
}

type aggregatedResult struct {
	result     models.SearchResult
	score      float64
	sourceHits map[string]bool
}

func discoverSearchResults(query string, limit int) ([]models.SearchResult, string) {
	queries := decomposeQueries(query)
	if len(queries) == 0 {
		queries = []string{query}
	}
	if len(queries) > 3 {
		queries = queries[:3]
	}

	engines := []discoveryEngine{
		{name: "searxng", fn: searchSearXNG},
		{name: "wikipedia", fn: searchWikipedia},
		{name: "arxiv", fn: searchArxiv},
		{name: "duckduckgo", fn: searchDDGFallback},
	}

	perEngineLimit := limit
	if perEngineLimit < 5 {
		perEngineLimit = 5
	}

	merged := make(map[string]*aggregatedResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, q := range queries {
		q := q
		for _, engine := range engines {
			engine := engine
			wg.Add(1)
			go func() {
				defer wg.Done()
				results, err := engine.fn(q, perEngineLimit)
				if err != nil || len(results) == 0 {
					return
				}
				for idx, res := range results {
					key := normalizeDiscoveryURL(res.URL)
					if key == "" {
						continue
					}
					rrf := 1.0 / (60.0 + float64(idx+1))
					mu.Lock()
					entry, found := merged[key]
					if !found {
						entry = &aggregatedResult{
							result:     res,
							sourceHits: make(map[string]bool),
						}
						merged[key] = entry
					}
					entry.score += rrf
					entry.sourceHits[engine.name] = true
					if len(res.Title) > len(entry.result.Title) {
						entry.result.Title = res.Title
					}
					if len(res.Snippet) > len(entry.result.Snippet) {
						entry.result.Snippet = res.Snippet
					}
					if entry.result.Source == "" {
						entry.result.Source = engine.name
					}
					if !strings.Contains(entry.result.Source, engine.name) {
						entry.result.Source += "," + engine.name
					}
					mu.Unlock()
				}
			}()
		}
	}

	wg.Wait()

	if len(merged) == 0 {
		return nil, "none"
	}

	scored := make([]aggregatedResult, 0, len(merged))
	for _, entry := range merged {
		entry.result.URL = normalizeDiscoveryURL(entry.result.URL)
		entry.result.Source = "multi:" + entry.result.Source
		scored = append(scored, *entry)
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].result.Title < scored[j].result.Title
		}
		return scored[i].score > scored[j].score
	})

	out := make([]models.SearchResult, 0, minInt(limit, len(scored)))
	for _, entry := range scored {
		entry.result.Score = entry.score
		out = append(out, entry.result)
		if len(out) >= limit {
			break
		}
	}

	return out, "multi"
}

func decomposeQueries(query string) []string {
	seen := map[string]bool{}
	add := func(q string, out *[]string) {
		q = strings.TrimSpace(q)
		q = strings.Trim(q, ",;|")
		if q == "" || seen[strings.ToLower(q)] {
			return
		}
		seen[strings.ToLower(q)] = true
		*out = append(*out, q)
	}

	var out []string
	add(query, &out)

	parts := splitQueryParts(query)
	for _, part := range parts {
		add(part, &out)
	}

	if len(out) == 0 {
		return []string{query}
	}
	return out
}

func splitQueryParts(query string) []string {
	separators := []string{" and ", " or ", " | ", ",", ";"}
	var current []string
	for _, sep := range separators {
		if strings.Contains(strings.ToLower(query), sep) {
			parts := strings.Split(query, sep)
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part != "" {
					current = append(current, part)
				}
			}
		}
	}
	quoted := extractQuotedPhrases(query)
	current = append(current, quoted...)
	return current
}

func extractQuotedPhrases(query string) []string {
	var phrases []string
	inQuote := false
	var current strings.Builder
	for _, r := range query {
		switch r {
		case '"':
			if inQuote {
				phrase := strings.TrimSpace(current.String())
				if phrase != "" {
					phrases = append(phrases, phrase)
				}
				current.Reset()
				inQuote = false
			} else {
				inQuote = true
			}
		default:
			if inQuote {
				current.WriteRune(r)
			}
		}
	}
	return phrases
}

func normalizeDiscoveryURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return strings.TrimRight(raw, "/")
	}
	parsed.Fragment = ""
	parsed.RawQuery = ""
	return strings.TrimRight(parsed.String(), "/")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func searchWikipedia(query string, limit int) ([]models.SearchResult, error) {
	params := url.Values{}
	params.Set("action", "query")
	params.Set("list", "search")
	params.Set("srsearch", query)
	params.Set("srlimit", fmt.Sprintf("%d", limit))
	params.Set("format", "json")
	params.Set("origin", "*")

	reqURL := "https://en.wikipedia.org/w/api.php?" + params.Encode()
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Searqon/1.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wikipedia returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Query struct {
			Search []struct {
				Title   string `json:"title"`
				Snippet string `json:"snippet"`
				PageID  int    `json:"pageid"`
			} `json:"search"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	out := make([]models.SearchResult, 0, limit)
	for _, item := range parsed.Query.Search {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			continue
		}
		snippet := strings.TrimSpace(html.UnescapeString(stripHTMLMarkup(item.Snippet)))
		out = append(out, models.SearchResult{
			Title:   title,
			URL:     "https://en.wikipedia.org/wiki/" + url.PathEscape(strings.ReplaceAll(title, " ", "_")),
			Snippet: snippet,
			Source:  "wikipedia",
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

type arxivFeed struct {
	Entries []struct {
		Title   string `xml:"title"`
		ID      string `xml:"id"`
		Summary string `xml:"summary"`
	} `xml:"entry"`
}

func searchArxiv(query string, limit int) ([]models.SearchResult, error) {
	params := url.Values{}
	params.Set("search_query", "all:"+query)
	params.Set("start", "0")
	params.Set("max_results", fmt.Sprintf("%d", limit))

	reqURL := "https://export.arxiv.org/api/query?" + params.Encode()
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Searqon/1.0")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("arxiv returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, err
	}

	var feed arxivFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, err
	}

	out := make([]models.SearchResult, 0, limit)
	for _, entry := range feed.Entries {
		title := strings.TrimSpace(entry.Title)
		if title == "" || entry.ID == "" {
			continue
		}
		snippet := strings.TrimSpace(entry.Summary)
		if len(snippet) > 400 {
			snippet = snippet[:400]
		}
		out = append(out, models.SearchResult{
			Title:   title,
			URL:     entry.ID,
			Snippet: snippet,
			Source:  "arxiv",
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func stripHTMLMarkup(input string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<div>" + input + "</div>"))
	if err != nil || doc == nil {
		return input
	}
	return strings.TrimSpace(doc.Text())
}

