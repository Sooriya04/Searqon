package extractor

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestGenerateSlugAnchor(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"Getting Started with API v2.0!", "getting-started-with-api-v20"},
		{"  Spaces and --- dashes --- ", "spaces-and-dashes"},
		{"", ""},
	}

	for _, tc := range tests {
		actual := GenerateSlugAnchor(tc.input)
		if actual != tc.expected {
			t.Errorf("GenerateSlugAnchor(%q) = %q; expected %q", tc.input, actual, tc.expected)
		}
	}
}

func TestEstimateTokens(t *testing.T) {
	// ~4 characters per token
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"abcd", 1},
		{"abcdefgh", 2},
		{strings.Repeat("a", 4000), 1000},
	}

	for _, tc := range tests {
		actual := EstimateTokens(tc.input)
		if actual != tc.expected {
			t.Errorf("EstimateTokens(%d chars) = %d; expected %d", len(tc.input), actual, tc.expected)
		}
	}
}

func TestCleanTruncateMarkdown(t *testing.T) {
	html := "<h1>Title</h1>\n\n<p>Paragraph one with enough length to test truncation.</p>\n\n<p>Paragraph two here.</p>"
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	headings, outline := ExtractHeadingsAndOutline(doc)
	if len(headings) != 1 || headings[0].Text != "Title" {
		t.Errorf("Expected 1 heading 'Title', got %v", headings)
	}
	if outline == "" {
		t.Errorf("Expected outline to be generated")
	}

	markdown := "# Title\n\nParagraph one with enough length to test truncation.\n\nParagraph two here."
	// Max tokens = 5 -> ~20 chars
	truncated, isTrunc := CleanTruncateMarkdown(markdown, 5)
	if !isTrunc {
		t.Errorf("Expected isTruncated = true")
	}
	if truncated == "" {
		t.Errorf("Expected non-empty truncated text")
	}
}
