package search

import (
	"bytes"
	"fmt"

	"src/models"
	"src/utils"
)

func queryOllama(prompt string) (string, error) {
	return utils.QueryOllama(prompt)
}

// SummarizePage content generates a TL;DR summary.
func SummarizePage(content string) (string, error) {
	if !utils.IsOllamaEnabled() {
		return "", fmt.Errorf("ollama is disabled")
	}
	if len(content) > 3000 {
		content = content[:3000]
	}
	prompt := fmt.Sprintf("Summarize the following text concisely. Keep the summary under 150 words:\n\n%s", content)
	return queryOllama(prompt)
}

// SynthesizeAnswer compiles an answer of search results.
func SynthesizeAnswer(query string, results []models.SearchResult) (string, error) {
	if !utils.IsOllamaEnabled() {
		return "", fmt.Errorf("ollama is disabled")
	}
	var ctx bytes.Buffer
	for i, r := range results {
		text := r.Snippet
		if r.Content != "" {
			if len(r.Content) > 600 {
				text = r.Content[:600]
			} else {
				text = r.Content
			}
		}
		fmt.Fprintf(&ctx, "[%d] Title: %s\nURL: %s\nSnippet: %s\n\n", i+1, r.Title, r.URL, text)
	}

	prompt := fmt.Sprintf(`You are Searqon synthesis assistant. Synthesize a structured, objective, direct answer to the user's query using only the provided search results below. Cite sources using bracketed numbers corresponding to the results (e.g. [1], [2]).

Query: %s

Search Results:
%s

Direct Answer:`, query, ctx.String())

	return queryOllama(prompt)
}
