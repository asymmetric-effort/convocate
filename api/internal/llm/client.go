package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

var (
	endpoint string
	apiKey   string
	model    string
)

func Init() {
	endpoint = os.Getenv("LLM_ENDPOINT")
	if endpoint == "" {
		endpoint = "https://api.anthropic.com/v1/messages"
	}
	apiKey = os.Getenv("LLM_API_KEY")
	model = os.Getenv("LLM_MODEL")
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CompletionRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type CompletionResponse struct {
	Content []ContentBlock `json:"content"`
}

// Endpoint returns the current LLM endpoint.
func Endpoint() string { return endpoint }

// SetEndpoint overrides the LLM endpoint (for testing).
func SetEndpoint(s string) { endpoint = s }

// SetAPIKey overrides the LLM API key (for testing).
func SetAPIKey(s string) { apiKey = s }

func Complete(systemPrompt, userPrompt string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("LLM_API_KEY not configured")
	}

	reqBody := CompletionRequest{
		Model: model,
		Messages: []Message{
			{Role: "user", Content: userPrompt},
		},
		MaxTokens: 4096,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM returned %d: %s", resp.StatusCode, string(respBody))
	}

	var completion CompletionResponse
	if err := json.Unmarshal(respBody, &completion); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if len(completion.Content) == 0 {
		return "", fmt.Errorf("empty response from LLM")
	}

	return completion.Content[0].Text, nil
}
