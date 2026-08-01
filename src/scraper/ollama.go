package scraper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
}

func getOllamaURL() string {
	if u := os.Getenv("OLLAMA_URL"); u != "" {
		return u
	}
	return "http://localhost:11434"
}

func getOllamaModel() string {
	if m := os.Getenv("OLLAMA_MODEL"); m != "" {
		return m
	}
	return "gemma:2b"
}

func queryOllamaJSON(prompt string) ([]byte, error) {
	body, err := queryOllama(prompt)
	if err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSpace([]byte(body))
	trimmed = bytes.TrimPrefix(trimmed, []byte("```json"))
	trimmed = bytes.TrimPrefix(trimmed, []byte("```"))
	trimmed = bytes.TrimSuffix(trimmed, []byte("```"))
	return bytes.TrimSpace(trimmed), nil
}

func queryOllama(prompt string) (string, error) {
	url := getOllamaURL() + "/api/generate"
	model := getOllamaModel()

	reqPayload := ollamaRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to contact Ollama: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned HTTP %d", resp.StatusCode)
	}

	var res ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	return res.Response, nil
}
