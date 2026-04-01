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

// ClaudeProvider implements Provider using the Anthropic Messages API.
type ClaudeProvider struct {
	apiKey      string
	model       string
	projectTags []string
	client      *http.Client
}

// NewClaudeProvider creates a new Anthropic Claude provider.
func NewClaudeProvider(apiKey string, tags []string) *ClaudeProvider {
	return &ClaudeProvider{
		apiKey:      apiKey,
		model:       "claude-sonnet-4-20250514",
		projectTags: tags,
		client:      &http.Client{Timeout: 30 * time.Second},
	}
}

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system"`
	Messages  []claudeMessage `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func (p *ClaudeProvider) ExtractTasks(ctx context.Context, transcript string) ([]ExtractedTask, error) {
	reqBody := claudeRequest{
		Model:     p.model,
		MaxTokens: 1024,
		System:    SystemPrompt(time.Now(), p.projectTags),
		Messages: []claudeMessage{
			{Role: "user", Content: transcript},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

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
		return nil, fmt.Errorf("anthropic API error (status %d): %s", resp.StatusCode, string(body))
	}

	var claudeResp claudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(claudeResp.Content) == 0 || claudeResp.Content[0].Text == "" {
		return nil, fmt.Errorf("empty response from Anthropic API")
	}

	return ParseTasksResponse([]byte(claudeResp.Content[0].Text), transcript, p.projectTags), nil
}
