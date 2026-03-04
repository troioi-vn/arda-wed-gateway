package suggestions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

const defaultLLMTranscriptDir = "logs"

type OpenRouterClient struct {
	baseURL       string
	model         string
	apiKey        string
	httpClient    *http.Client
	transcriptDir string
	transcriptSeq uint64
}

func NewOpenRouterClient(baseURL, model, apiKey string, timeout time.Duration) *OpenRouterClient {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	if strings.TrimSpace(model) == "" {
		model = "openai/gpt-4o-mini"
	}
	if timeout <= 0 {
		timeout = 25 * time.Second
	}

	return &OpenRouterClient{
		baseURL:       strings.TrimRight(baseURL, "/"),
		model:         model,
		apiKey:        apiKey,
		transcriptDir: defaultLLMTranscriptDir,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *OpenRouterClient) RequestSuggestion(ctx context.Context, prompt string) (string, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return "", fmt.Errorf("openrouter api key is not configured")
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		content, err := c.requestOnce(ctx, prompt)
		if err == nil {
			return content, nil
		}
		lastErr = err
		if !isRetryableTimeout(err) || attempt == 1 {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	return "", lastErr
}

func (c *OpenRouterClient) requestOnce(ctx context.Context, prompt string) (content string, err error) {
	startedAt := time.Now().UTC()
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
	var rawResponse []byte
	defer func() {
		c.writeTranscript(startedAt, requestPayload, rawResponse, content, err)
	}()

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
	rawResponse = append([]byte(nil), payload...)

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

	content = strings.TrimSpace(decoded.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("openrouter response content is empty")
	}
	return content, nil
}

func (c *OpenRouterClient) writeTranscript(startedAt time.Time, requestPayload map[string]any, rawResponse []byte, content string, requestErr error) {
	dir := c.transcriptDir
	if strings.TrimSpace(dir) == "" {
		dir = defaultLLMTranscriptDir
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}

	requestJSON, err := json.MarshalIndent(requestPayload, "", "  ")
	if err != nil {
		requestJSON = []byte(`{"error":"failed to encode request payload"}`)
	}

	seq := atomic.AddUint64(&c.transcriptSeq, 1)
	filename := fmt.Sprintf(
		"%s-%06d.log",
		startedAt.Format("20060102T150405.000000000Z"),
		seq,
	)
	path := filepath.Join(dir, filename)

	var b strings.Builder
	b.WriteString("started_at_utc: ")
	b.WriteString(startedAt.Format(time.RFC3339Nano))
	b.WriteByte('\n')
	b.WriteString("model: ")
	b.WriteString(c.model)
	b.WriteByte('\n')
	b.WriteString("request_json:\n")
	b.Write(requestJSON)
	b.WriteString("\n\nresponse_raw:\n")
	if len(rawResponse) == 0 {
		b.WriteString("(empty)\n")
	} else {
		b.Write(rawResponse)
		b.WriteByte('\n')
	}
	b.WriteString("\nresponse_content:\n")
	if strings.TrimSpace(content) == "" {
		b.WriteString("(empty)\n")
	} else {
		b.WriteString(content)
		b.WriteByte('\n')
	}
	b.WriteString("\nerror:\n")
	if requestErr != nil {
		b.WriteString(requestErr.Error())
		b.WriteByte('\n')
	} else {
		b.WriteString("(none)\n")
	}

	_ = os.WriteFile(path, []byte(b.String()), 0o644)
}

func isRetryableTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
