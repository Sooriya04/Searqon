package search

import (
	"hash/fnv"
	"math/bits"

	"src/models"
)

// SimHash computes a 64-bit SimHash value.
func SimHash(text string) uint64 {
	words := tokenize(text)
	if len(words) == 0 {
		return 0
	}

	var features []string
	for i := 0; i < len(words)-1; i++ {
		features = append(features, words[i]+" "+words[i+1])
	}
	if len(features) == 0 {
		features = words
	}

	vector := make([]int, 64)

	for _, f := range features {
		h := fnv.New64a()
		h.Write([]byte(f))
		hashVal := h.Sum64()

		for i := 0; i < 64; i++ {
			bit := (hashVal >> i) & 1
			if bit == 1 {
				vector[i]++
			} else {
				vector[i]--
			}
		}
	}

	var simhash uint64
	for i := 0; i < 64; i++ {
		if vector[i] > 0 {
			simhash |= (1 << i)
		}
	}

	return simhash
}

// HammingDistance calculates Hamming distance between two SimHash keys.
func HammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// DeduplicateResults filters out near-duplicate results.
func DeduplicateResults(results []models.SearchResult) []models.SearchResult {
	if len(results) <= 1 {
		return results
	}

	var clean []models.SearchResult
	var hashes []uint64

	for _, r := range results {
		text := r.Content
		if text == "" {
			text = r.Snippet + " " + r.Title
		}
		if text == "" {
			clean = append(clean, r)
			continue
		}

		sh := SimHash(text)
		isDuplicate := false

		for _, existingHash := range hashes {
			if HammingDistance(sh, existingHash) <= 4 {
				isDuplicate = true
				break
			}
		}

		if !isDuplicate {
			clean = append(clean, r)
			hashes = append(hashes, sh)
		}
	}

	return clean
}
