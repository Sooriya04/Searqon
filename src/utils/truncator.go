package utils

import (
	"strings"
)

// TruncateTextByWords truncates text to a maximum number of words.
func TruncateTextByWords(text string, maxWords int) string {
	if maxWords <= 0 {
		return text
	}
	words := strings.Fields(text)
	if len(words) <= maxWords {
		return text
	}
	return strings.Join(words[:maxWords], " ") + " ... [Content truncated]"
}
