package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"src/config"
	"src/models"
)

var (
	ollamaClient     *http.Client
	ollamaClientOnce sync.Once
)

func getOllamaHTTPClient() *http.Client {
	ollamaClientOnce.Do(func() {
		ollamaClient = &http.Client{Timeout: 30 * time.Second}
	})
	return ollamaClient
}

// GetOllamaURL returns the configured Ollama base URL.
func GetOllamaURL() string {
	if u := os.Getenv("OLLAMA_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	if u := os.Getenv("OLLAMA_HOST"); u != "" {
		return strings.TrimRight(u, "/")
	}
	cfg := config.Get()
	if cfg != nil && cfg.Ollama.URL != "" {
		return strings.TrimRight(cfg.Ollama.URL, "/")
	}
	return "http://localhost:11434"
}

// GetOllamaModel returns the configured LLM generation model.
func GetOllamaModel() string {
	if m := os.Getenv("OLLAMA_MODEL"); m != "" {
		return m
	}
	cfg := config.Get()
	if cfg != nil && cfg.Ollama.Model != "" {
		return cfg.Ollama.Model
	}
	return "gemma:2b"
}

// GetOllamaEmbeddingModel returns the configured vector embedding model.
func GetOllamaEmbeddingModel() string {
	if m := os.Getenv("OLLAMA_EMBEDDING_MODEL"); m != "" {
		return m
	}
	cfg := config.Get()
	if cfg != nil && cfg.Ollama.EmbeddingModel != "" {
		return cfg.Ollama.EmbeddingModel
	}
	return "nomic-embed-text"
}

// IsOllamaEnabled checks whether local Ollama queries are enabled in settings and environment.
func IsOllamaEnabled() bool {
	if oe := os.Getenv("OLLAMA_ENABLED"); oe != "" {
		return oe == "true" || oe == "1"
	}
	cfg := config.Get()
	if cfg != nil {
		return cfg.Ollama.Enabled
	}
	return true
}

type ollamaGenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaGenerateResponse struct {
	Response string `json:"response"`
}

type ollamaEmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

// QueryOllama sends a prompt to Ollama's generate endpoint.
func QueryOllama(prompt string) (string, error) {
	if !IsOllamaEnabled() {
		return "", fmt.Errorf("ollama is disabled in configuration")
	}

	url := GetOllamaURL() + "/api/generate"
	model := GetOllamaModel()

	reqPayload := ollamaGenerateRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return "", err
	}

	client := getOllamaHTTPClient()
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to contact Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned HTTP %d", resp.StatusCode)
	}

	var res ollamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	return res.Response, nil
}

// QueryOllamaJSON queries Ollama and strips Markdown code block formatting.
func QueryOllamaJSON(prompt string) ([]byte, error) {
	body, err := QueryOllama(prompt)
	if err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSpace([]byte(body))
	trimmed = bytes.TrimPrefix(trimmed, []byte("```json"))
	trimmed = bytes.TrimPrefix(trimmed, []byte("```"))
	trimmed = bytes.TrimSuffix(trimmed, []byte("```"))
	return bytes.TrimSpace(trimmed), nil
}

// QueryOllamaEmbedding requests vector embeddings for the given text.
func QueryOllamaEmbedding(text string) ([]float32, error) {
	if !IsOllamaEnabled() {
		return nil, fmt.Errorf("ollama is disabled in configuration")
	}

	url := GetOllamaURL() + "/api/embeddings"
	model := GetOllamaEmbeddingModel()

	reqPayload := ollamaEmbeddingRequest{
		Model:  model,
		Prompt: text,
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, err
	}

	client := getOllamaHTTPClient()
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to request Ollama embeddings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama embeddings returned HTTP %d", resp.StatusCode)
	}

	var res ollamaEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return res.Embedding, nil
}

// CosineSimilarity computes cosine similarity between two float32 vectors.
func CosineSimilarity(a, b []float32) float64 {
	return models.CosineSimilarity(a, b)
}
