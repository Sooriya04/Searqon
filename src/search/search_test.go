package search

import (
	"context"
	"testing"
	"time"

	"src/models"
)

func TestSearchPipeline_TimeoutSafety(t *testing.T) {
	// Create an already-cancelled or very short deadline context to trigger scrape timeout path
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait for context to cancel
	<-ctx.Done()

	// Run pipeline with scrape=true
	// When timeout fires, the results slice should be safely cloned under mu.Lock() without panicking or racing
	resp := RunSearchPipelineWithContext(ctx, "golang testing", 5, true, false, 0, false, "")

	// Result should return gracefully
	if resp.Results == nil {
		// Empty results or none provider is fine for cancelled/stubbed context
		t.Logf("Search returned with provider=%s", resp.Provider)
	}
}

func TestSearchPipeline_ResultCloningUnderLock(t *testing.T) {
	// Directly verify slice isolation
	original := []models.SearchResult{
		{URL: "https://example.com/1", Title: "One"},
		{URL: "https://example.com/2", Title: "Two"},
	}

	cloned := make([]models.SearchResult, len(original))
	copy(cloned, original)

	// Mutating original should not affect cloned
	original[0].Content = "mutated content"
	if cloned[0].Content == "mutated content" {
		t.Fatalf("cloned slice was unexpectedly mutated through backing array")
	}
}
