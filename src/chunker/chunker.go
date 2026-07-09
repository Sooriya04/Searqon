package chunker

import (
	"strings"

	"src/models"
)

const (
	ChunkTokenTarget = 512
	ChunkOverlap     = 128
	MinChunkWords    = 60
	ChunkWordTarget  = 385 // 512 / 1.33
	ChunkWordOverlap = 96  // 128 / 1.33
)

// EstimateTokens approximates token count (approx 1.33 tokens per word)
func EstimateTokens(wordCount int) int {
	return int(float64(wordCount) * 1.33)
}

// ChunkMarkdown splits markdown text into overlapping sentence-boundary chunks.
func ChunkMarkdown(content string, url string, title string, scrapedAt string) []models.Chunk {
	if content == "" {
		return nil
	}

	rawSentences := splitSentences(content)

	var sentences []string
	for _, s := range rawSentences {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			sentences = append(sentences, trimmed)
		}
	}

	if len(sentences) == 0 {
		return nil
	}

	var chunks []models.Chunk
	var currentSentences []string
	currentWords := 0
	chunkIndex := 0

	for i := 0; i < len(sentences); i++ {
		sent := sentences[i]
		sentWords := len(strings.Fields(sent))
		
		if currentWords+sentWords > ChunkWordTarget && len(currentSentences) > 0 {
			chunkText := strings.Join(currentSentences, " ")
			wordCount := len(strings.Fields(chunkText))
			
			if wordCount >= MinChunkWords {
				chunks = append(chunks, models.Chunk{
					Index:      chunkIndex,
					Text:       chunkText,
					TokenCount: EstimateTokens(wordCount),
					WordCount:  wordCount,
					Metadata: models.ChunkMetadata{
						SourceURL:   url,
						SourceTitle: title,
						ChunkIndex:  chunkIndex,
						ScrapedAt:   scrapedAt,
					},
				})
				chunkIndex++
			}

			// Backtrack for overlap (~128 tokens / 96 words)
			overlapWords := 0
			overlapIdx := len(currentSentences) - 1
			for overlapIdx >= 0 {
				w := len(strings.Fields(currentSentences[overlapIdx]))
				if overlapWords+w > ChunkWordOverlap {
					break
				}
				overlapWords += w
				overlapIdx--
			}
			if overlapIdx < 0 {
				overlapIdx = 0
			}
			
			currentSentences = currentSentences[overlapIdx:]
			currentWords = overlapWords
		}

		currentSentences = append(currentSentences, sent)
		currentWords += sentWords
	}

	if len(currentSentences) > 0 {
		chunkText := strings.Join(currentSentences, " ")
		wordCount := len(strings.Fields(chunkText))
		if wordCount >= MinChunkWords {
			chunks = append(chunks, models.Chunk{
				Index:      chunkIndex,
				Text:       chunkText,
				TokenCount: EstimateTokens(wordCount),
				WordCount:  wordCount,
				Metadata: models.ChunkMetadata{
					SourceURL:   url,
					SourceTitle: title,
					ChunkIndex:  chunkIndex,
					ScrapedAt:   scrapedAt,
				},
			})
		}
	}

	return chunks
}

func splitSentences(text string) []string {
	var sentences []string
	var current strings.Builder

	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		current.WriteRune(runes[i])

		if runes[i] == '.' || runes[i] == '!' || runes[i] == '?' {
			if i == len(runes)-1 {
				sentences = append(sentences, current.String())
				current.Reset()
				continue
			}

			next := runes[i+1]
			if next == ' ' || next == '\n' || next == '\r' || next == '\t' {
				str := strings.ToLower(current.String())
				isAbbrev := false
				abbrevs := []string{"mr.", "ms.", "dr.", "prof.", "inc.", "co.", "ltd.", "e.g.", "i.e.", "vs.", "al."}
				for _, abbrev := range abbrevs {
					if strings.HasSuffix(str, " "+abbrev) || strings.HasPrefix(str, abbrev) {
						isAbbrev = true
						break
					}
				}
				if !isAbbrev {
					sentences = append(sentences, current.String())
					current.Reset()
				}
			}
		} else if runes[i] == '\n' {
			if i < len(runes)-1 && runes[i+1] == '\n' {
				sentences = append(sentences, current.String())
				current.Reset()
				i++
			}
		}
	}

	if current.Len() > 0 {
		sentences = append(sentences, current.String())
	}

	return sentences
}
