package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type embeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type embeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

func getEmbeddingModel() string {
	if m := os.Getenv("OLLAMA_EMBEDDING_MODEL"); m != "" {
		return m
	}
	return "nomic-embed-text"
}

// GetVectorEmbedding requests vector embeddings from a local Ollama server.
func GetVectorEmbedding(text string) ([]float32, error) {
	url := getOllamaURL() + "/api/embeddings"
	model := getEmbeddingModel()

	reqPayload := embeddingRequest{
		Model:  model,
		Prompt: text,
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama embeddings returned HTTP %d", resp.StatusCode)
	}

	var res embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return res.Embedding, nil
}
