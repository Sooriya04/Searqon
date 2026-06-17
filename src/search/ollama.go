package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"src/models"
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

// SummarizePage content generates a TL;DR summary.
func SummarizePage(content string) (string, error) {
	if len(content) > 3000 {
		content = content[:3000]
	}
	prompt := fmt.Sprintf("Summarize the following text concisely. Keep the summary under 150 words:\n\n%s", content)
	return queryOllama(prompt)
}

// SynthesizeAnswer compiles a answer of search results.
func SynthesizeAnswer(query string, results []models.SearchResult) (string, error) {
	var ctx bytes.Buffer
	for i, r := range results {
		text := r.Snippet
		if r.Content != "" {
			if len(r.Content) > 600 {
				text = r.Content[:600]
			} else {
				text = r.Content
			}
		}
		fmt.Fprintf(&ctx, "[%d] Title: %s\nURL: %s\nSnippet: %s\n\n", i+1, r.Title, r.URL, text)
	}

	prompt := fmt.Sprintf(`You are Searqon synthesis assistant. Synthesize a structured, objective, direct answer to the user's query using only the provided search results below. Cite sources using bracketed numbers corresponding to the results (e.g. [1], [2]).

Query: %s

Search Results:
%s
Answer:`, query, ctx.String())

	return queryOllama(prompt)
}
