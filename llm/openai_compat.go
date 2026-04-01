package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAICompatProvider implements Provider using the OpenAI-compatible chat completions API.
// Works with OpenAI, Groq, and Ollama.
type OpenAICompatProvider struct {
	apiURL      string
	apiKey      string
	model       string
	projectTags []string
	client      *http.Client
}

// NewOpenAIProvider creates a provider for the OpenAI API.
func NewOpenAIProvider(apiKey string, tags []string) *OpenAICompatProvider {
	return &OpenAICompatProvider{
		apiURL:      "https://api.openai.com/v1/chat/completions",
		apiKey:      apiKey,
		model:       "gpt-4o-mini",
		projectTags: tags,
		client:      &http.Client{Timeout: 30 * time.Second},
	}
}

// NewGroqProvider creates a provider for the Groq API.
func NewGroqProvider(apiKey string, tags []string) *OpenAICompatProvider {
	return &OpenAICompatProvider{
		apiURL:      "https://api.groq.com/openai/v1/chat/completions",
		apiKey:      apiKey,
		model:       "llama-3.3-70b-versatile",
		projectTags: tags,
		client:      &http.Client{Timeout: 30 * time.Second},
	}
}

// NewOllamaProvider creates a provider for a local Ollama instance.
func NewOllamaProvider(baseURL string, tags []string) *OpenAICompatProvider {
	return &OpenAICompatProvider{
		apiURL:      baseURL + "/v1/chat/completions",
		apiKey:      "",
		model:       "llama3.2",
		projectTags: tags,
		client:      &http.Client{Timeout: 60 * time.Second},
	}
}

type openaiRequest struct {
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (p *OpenAICompatProvider) ExtractTasks(ctx context.Context, transcript string) ([]ExtractedTask, error) {
	reqBody := openaiRequest{
		Model: p.model,
		Messages: []openaiMessage{
			{Role: "system", Content: SystemPrompt(time.Now(), p.projectTags)},
			{Role: "user", Content: transcript},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", p.apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var openaiResp openaiResponse
	if err := json.Unmarshal(body, &openaiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(openaiResp.Choices) == 0 || openaiResp.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("empty response from API")
	}

	return ParseTasksResponse([]byte(openaiResp.Choices[0].Message.Content), transcript, p.projectTags), nil
}
