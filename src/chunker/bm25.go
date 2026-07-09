package chunker

import (
	"math"
	"strings"

	"src/models"
)

const (
	bm25K1 = 1.5
	bm25B  = 0.75
)

// ScoreChunks scores a slice of chunks against a query using BM25.
// Mutates chunk.BM25Score in-place and returns the scored chunks.
func ScoreChunks(chunks []models.Chunk, query string) []models.Chunk {
	if len(chunks) == 0 || query == "" {
		return chunks
	}

	queryTerms := strings.Fields(strings.ToLower(query))
	N := float64(len(chunks))

	var totalWords float64
	for _, c := range chunks {
		totalWords += float64(c.WordCount)
	}
	var avgdl float64
	if N > 0 {
		avgdl = totalWords / N
	}
	if avgdl <= 0 {
		avgdl = 1.0
	}

	dfMap := make(map[string]float64)
	for _, term := range queryTerms {
		var df float64
		for _, chunk := range chunks {
			doc := strings.ToLower(chunk.Text)
			if strings.Contains(doc, term) {
				df++
			}
		}
		dfMap[term] = df
	}

	for i, chunk := range chunks {
		doc := strings.ToLower(chunk.Text)
		words := strings.Fields(doc)
		
		tf := make(map[string]int)
		for _, w := range words {
			wClean := cleanWord(w)
			tf[wClean]++
		}

		var score float64
		for _, term := range queryTerms {
			df := dfMap[term]
			idf := math.Log((N-df+0.5)/(df+0.5) + 1.0)
			if idf < 0 {
				idf = 0.0001
			}

			cleanTerm := cleanWord(term)
			f := float64(tf[cleanTerm])
			dl := float64(chunk.WordCount)

			tfNorm := (f * (bm25K1 + 1.0)) / (f + bm25K1*(1.0-bm25B+bm25B*(dl/avgdl)))
			score += idf * tfNorm
		}
		chunks[i].BM25Score = score
	}

	return chunks
}

func cleanWord(w string) string {
	w = strings.ToLower(w)
	w = strings.TrimFunc(w, func(r rune) bool {
		return r == '.' || r == ',' || r == '!' || r == '?' || r == ';' || r == ':' || r == '(' || r == ')' || r == '[' || r == ']' || r == '{' || r == '}' || r == '"' || r == '\'' || r == '`'
	})
	return w
}
