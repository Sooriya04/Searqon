package chunker

import (
	"strings"
	"testing"

	"src/models"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		words int
		want  int
	}{
		{0, 0},
		{10, 13},
		{100, 133},
		{385, 512},
	}

	for _, tt := range tests {
		got := EstimateTokens(tt.words)
		if got != tt.want {
			t.Errorf("EstimateTokens(%d) = %d, want %d", tt.words, got, tt.want)
		}
	}
}

func TestChunkMarkdown(t *testing.T) {
	// Empty content should return nil
	if chunks := ChunkMarkdown("", "https://example.com", "Title", ""); chunks != nil {
		t.Fatalf("expected nil for empty content, got %v", chunks)
	}

	// Long content split into sentences
	var sb strings.Builder
	for i := 0; i < 40; i++ {
		sb.WriteString("This is sentence number ")
		sb.WriteString(string(rune('A' + (i % 26))))
		sb.WriteString(" providing sufficient words to exceed minimum chunk thresholds. ")
	}
	content := sb.String()

	chunks := ChunkMarkdown(content, "https://example.com/test", "Test Page", "2026-11-09T00:00:00Z")
	if len(chunks) == 0 {
		t.Fatalf("expected chunks to be created, got 0")
	}

	for i, c := range chunks {
		if c.Metadata.SourceURL != "https://example.com/test" {
			t.Errorf("chunk %d URL mismatch: %s", i, c.Metadata.SourceURL)
		}
		if c.Index != i {
			t.Errorf("chunk %d Index mismatch: %d", i, c.Index)
		}
		if c.WordCount <= 0 {
			t.Errorf("chunk %d WordCount must be > 0", i)
		}
		if c.TokenCount <= 0 {
			t.Errorf("chunk %d TokenCount must be > 0", i)
		}
	}
}

func TestDeduplicateChunks(t *testing.T) {
	chunk1 := models.Chunk{
		Index:     0,
		Text:      "The quick brown fox jumps over the lazy dog in the sunny park.",
		WordCount: 12,
	}
	chunk2 := models.Chunk{
		Index:     1,
		Text:      "The quick brown fox jumps over the lazy dog in the sunny park.", // Exact duplicate
		WordCount: 12,
	}
	chunk3 := models.Chunk{
		Index:     2,
		Text:      "Quantum computing leverages superposition and entanglement to perform complex parallel calculations.",
		WordCount: 12,
	}

	chunks := []models.Chunk{chunk1, chunk2, chunk3}
	deduped := DeduplicateChunks(chunks)

	if len(deduped) != 2 {
		t.Fatalf("expected 2 unique chunks after deduplication, got %d", len(deduped))
	}
	if deduped[0].Text != chunk1.Text || deduped[1].Text != chunk3.Text {
		t.Errorf("unexpected deduped chunk contents: %+v", deduped)
	}

	// Single chunk or empty slice
	if len(DeduplicateChunks(nil)) != 0 {
		t.Errorf("expected empty result for nil input")
	}
	if len(DeduplicateChunks([]models.Chunk{chunk1})) != 1 {
		t.Errorf("expected 1 result for 1 chunk input")
	}
}

func TestScoreChunks_BM25(t *testing.T) {
	c1 := models.Chunk{
		Index:     0,
		Text:      "Machine learning and deep neural networks have transformed artificial intelligence.",
		WordCount: 10,
	}
	c2 := models.Chunk{
		Index:     1,
		Text:      "Traditional gardening requires fertile soil, adequate sunlight, and daily watering.",
		WordCount: 10,
	}

	chunks := []models.Chunk{c1, c2}
	scored := ScoreChunks(chunks, "neural networks intelligence")

	if len(scored) != 2 {
		t.Fatalf("expected 2 scored chunks, got %d", len(scored))
	}
	if scored[0].BM25Score <= 0 {
		t.Errorf("expected positive BM25 score for matching chunk, got %f", scored[0].BM25Score)
	}
	if scored[0].BM25Score <= scored[1].BM25Score {
		t.Errorf("expected matching chunk score (%f) > non-matching chunk score (%f)", scored[0].BM25Score, scored[1].BM25Score)
	}

	// Empty query returns unmodified chunks
	unscored := ScoreChunks(chunks, "")
	if len(unscored) != 2 {
		t.Errorf("expected 2 chunks for empty query")
	}
}
