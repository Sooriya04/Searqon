package chunker

import (
	"hash/fnv"
	"math/bits"
	"strings"

	"src/models"
)

func chunkSimHash(text string) uint64 {
	words := strings.Fields(strings.ToLower(text))
	if len(words) == 0 {
		return 0
	}

	vector := make([]int, 64)
	for i := 0; i < len(words); i++ {
		token := words[i]
		if i < len(words)-1 {
			token += " " + words[i+1]
		}
		h := fnv.New64a()
		_, _ = h.Write([]byte(token))
		hashVal := h.Sum64()
		for bit := 0; bit < 64; bit++ {
			if (hashVal>>bit)&1 == 1 {
				vector[bit]++
			} else {
				vector[bit]--
			}
		}
	}

	var sig uint64
	for bit := 0; bit < 64; bit++ {
		if vector[bit] > 0 {
			sig |= 1 << bit
		}
	}
	return sig
}

func chunkHammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// DeduplicateChunks removes near-duplicate chunks while preserving order.
func DeduplicateChunks(chunks []models.Chunk) []models.Chunk {
	if len(chunks) <= 1 {
		return chunks
	}

	var clean []models.Chunk
	var hashes []uint64
	for _, chunk := range chunks {
		text := strings.TrimSpace(chunk.Text)
		if text == "" {
			continue
		}
		sig := chunkSimHash(text)
		duplicate := false
		for _, existing := range hashes {
			if chunkHammingDistance(sig, existing) <= 4 {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		hashes = append(hashes, sig)
		clean = append(clean, chunk)
	}
	return clean
}
