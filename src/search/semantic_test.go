package search

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateEmbeddingText(t *testing.T) {
	// Short text remains intact
	short := "Hello world"
	if got := truncateEmbeddingText(short); got != short {
		t.Fatalf("expected %q, got %q", short, got)
	}

	// Long ASCII string exactly capped at 1800 bytes
	longASCII := strings.Repeat("a", 2000)
	gotASCII := truncateEmbeddingText(longASCII)
	if len(gotASCII) != 1800 {
		t.Fatalf("expected length 1800, got %d", len(gotASCII))
	}

	// Multi-byte character crossing byte boundary 1800
	// 1799 ASCII 'a' bytes followed by a 4-byte emoji (e.g., '🚀' which is 4 bytes: 0xF0, 0x9F, 0x99, 0x80)
	prefix := strings.Repeat("a", 1799)
	rocket := "🚀"
	testString := prefix + rocket + "suffix"
	gotUTF8 := truncateEmbeddingText(testString)

	if !utf8.ValidString(gotUTF8) {
		t.Fatalf("truncated string contains invalid UTF-8")
	}
	// The rocket started at byte 1799 and would have been cut at byte 1800 (1 byte in).
	// With backtracking, it should be cleanly trimmed before the rocket at 1799.
	if len(gotUTF8) != 1799 {
		t.Fatalf("expected length 1799, got %d", len(gotUTF8))
	}
	if gotUTF8 != prefix {
		t.Fatalf("expected prefix of 1799 'a's, got %q", gotUTF8)
	}
}
