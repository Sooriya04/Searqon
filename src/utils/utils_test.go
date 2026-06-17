package utils

import (
	"testing"
)

func TestCountWords(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"hello", 1},
		{"hello world", 2},
		{"  hello   world  with   spaces ", 4},
	}

	for _, tc := range tests {
		actual := CountWords(tc.input)
		if actual != tc.expected {
			t.Errorf("CountWords(%q) = %d; expected %d", tc.input, actual, tc.expected)
		}
	}
}

func TestCleanText(t *testing.T) {
	input := "┌─── Title ───┐\n# Hello World  \n  This is   a test. \n\n  Another line."
	expected := "Title Hello World This is a test. Another line."
	actual := CleanText(input)
	if actual != expected {
		t.Errorf("CleanText() =\n%q\nexpected:\n%q", actual, expected)
	}
}

func TestTruncateTextByWords(t *testing.T) {
	input := "one two three four five six"
	tests := []struct {
		maxWords int
		expected string
	}{
		{0, "one two three four five six"},
		{-5, "one two three four five six"},
		{3, "one two three ... [Content truncated]"},
		{10, "one two three four five six"},
	}

	for _, tc := range tests {
		actual := TruncateTextByWords(input, tc.maxWords)
		if actual != tc.expected {
			t.Errorf("TruncateTextByWords(%d) = %q; expected %q", tc.maxWords, actual, tc.expected)
		}
	}
}
