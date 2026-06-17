package search

import (
	"testing"

	"src/models"
)

func TestTokenize(t *testing.T) {
	input := "Hello, World! Go-lang is awesome."
	expected := []string{"hello", "world", "go", "lang", "is", "awesome"}
	actual := tokenize(input)

	if len(actual) != len(expected) {
		t.Fatalf("tokenize length mismatch: got %v, expected %v", actual, expected)
	}

	for i := range actual {
		if actual[i] != expected[i] {
			t.Errorf("tokenize mismatch at index %d: got %s, expected %s", i, actual[i], expected[i])
		}
	}
}

func TestQueryTerms(t *testing.T) {
	input := "what is the go language?"
	expected := []string{"go", "language"}
	actual := queryTerms(input)

	if len(actual) != len(expected) {
		t.Fatalf("queryTerms length mismatch: got %v, expected %v", actual, expected)
	}

	for i := range actual {
		if actual[i] != expected[i] {
			t.Errorf("queryTerms mismatch at index %d: got %s, expected %s", i, actual[i], expected[i])
		}
	}
}

func TestRankResults(t *testing.T) {
	results := []models.SearchResult{
		{
			Title:   "Unrelated webpage",
			Snippet: "This is about something completely different.",
			URL:     "https://example.com/unrelated",
		},
		{
			Title:   "Go Language Reference Manual",
			Snippet: "Official reference documentation for the Go programming language.",
			URL:     "https://go.dev/doc/reference",
		},
		{
			Title:   "Wikipedia: Go (programming language)",
			Snippet: "Go is a statically typed, compiled programming language designed at Google.",
			URL:     "https://en.wikipedia.org/wiki/Go_(programming_language)",
		},
	}

	query := "go programming language"
	ranked := rankResults(results, query)

	if len(ranked) != len(results) {
		t.Fatalf("ranked results length mismatch: got %d, expected %d", len(ranked), len(results))
	}

	// The Wikipedia article or go.dev reference should be ranked higher than the unrelated one
	if ranked[0].Title == "Unrelated webpage" {
		t.Errorf("Unrelated webpage ranked first; expected Go programming language topic first")
	}
}
