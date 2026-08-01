package search

import (
	"math"
	"sort"
	"strings"

	"src/models"
)

func cosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	n := len(a)
	if len(b) < n {
		n = len(b)
	}

	var dot, magA, magB float64
	for i := 0; i < n; i++ {
		af := float64(a[i])
		bf := float64(b[i])
		dot += af * bf
		magA += af * af
		magB += bf * bf
	}
	if magA == 0 || magB == 0 {
		return 0
	}
	return dot / (math.Sqrt(magA) * math.Sqrt(magB))
}

func semanticRankResults(results []models.SearchResult, query string) []models.SearchResult {
	if len(results) <= 1 || strings.TrimSpace(query) == "" {
		return results
	}

	queryEmbedding, err := GetVectorEmbedding(query)
	if err != nil {
		return results
	}

	type scored struct {
		result models.SearchResult
		score  float64
	}

	scoredResults := make([]scored, 0, len(results))
	for _, result := range results {
		text := result.Content
		if text == "" {
			text = result.Snippet + " " + result.Title
		}
		if text == "" {
			scoredResults = append(scoredResults, scored{result: result, score: result.Score})
			continue
		}

		emb, err := GetVectorEmbedding(truncateEmbeddingText(text))
		if err != nil {
			scoredResults = append(scoredResults, scored{result: result, score: result.Score})
			continue
		}

		semantic := cosineSimilarity(queryEmbedding, emb)
		hybrid := (result.Score * 0.7) + (semantic * 0.3)
		scoredResults = append(scoredResults, scored{result: result, score: hybrid})
	}

	sort.SliceStable(scoredResults, func(i, j int) bool {
		return scoredResults[i].score > scoredResults[j].score
	})

	out := make([]models.SearchResult, len(scoredResults))
	for i, scored := range scoredResults {
		scored.result.Score = math.Round(scored.score*1000) / 1000
		out[i] = scored.result
	}
	return out
}

func truncateEmbeddingText(text string) string {
	text = strings.TrimSpace(text)
	if len(text) > 1800 {
		return text[:1800]
	}
	return text
}
