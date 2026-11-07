package search

import (
	"src/utils"
)

func getEmbeddingModel() string {
	return utils.GetOllamaEmbeddingModel()
}

// GetVectorEmbedding requests vector embeddings from the configured Ollama server.
func GetVectorEmbedding(text string) ([]float32, error) {
	return utils.QueryOllamaEmbedding(text)
}
