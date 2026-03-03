package suggestions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OpenRouterClient struct {
	baseURL    string
	model      string
	apiKey     string
	httpClient *http.Client
}

func NewOpenRouterClient(baseURL, model, apiKey string, timeout time.Duration) *OpenRouterClient {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	if strings.TrimSpace(model) == "" {
		model = "openai/gpt-4o-mini"
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	return &OpenRouterClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *OpenRouterClient) RequestSuggestion(ctx context.Context, prompt string) (string, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return "", fmt.Errorf("openrouter api key is not configured")
	}

	requestPayload := map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are an Arda MUD assistant. Return strict JSON only with keys: commands, reason, expected_outcome.",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0.2,
	}

	body, err := json.Marshal(requestPayload)
	if err != nil {
		return "", fmt.Errorf("marshal openrouter request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create openrouter request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("openrouter request failed: %w", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read openrouter response: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("openrouter status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return "", fmt.Errorf("decode openrouter response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return "", fmt.Errorf("openrouter response has no choices")
	}

	content := strings.TrimSpace(decoded.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("openrouter response content is empty")
	}
	return content, nil
}
