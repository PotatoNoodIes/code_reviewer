// Package ollama is a minimal client for a self-hosted Ollama server's
// /api/chat endpoint, with support for constraining output to JSON.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// FormatMode controls how JSON output is requested from Ollama.
type FormatMode string

const (
	// FormatSchema passes a full JSON-Schema object as "format", which
	// newer Ollama versions (0.5+) use to constrain decoding exactly.
	FormatSchema FormatMode = "schema"
	// FormatJSON passes the plain string "json" as "format", the older
	// Ollama mechanism that only guarantees syntactically valid JSON, not
	// a specific shape.
	FormatJSON FormatMode = "json"
	// FormatNone omits the format field entirely.
	FormatNone FormatMode = "none"
)

// ChatMessage is one turn in a /api/chat conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string          `json:"model"`
	Messages []ChatMessage   `json:"messages"`
	Stream   bool            `json:"stream"`
	Format   json.RawMessage `json:"format,omitempty"`
	Options  map[string]any  `json:"options,omitempty"`
}

type chatResponse struct {
	Message ChatMessage `json:"message"`
	Done    bool        `json:"done"`
}

// Client talks to a single Ollama server/model pair.
type Client struct {
	BaseURL    string
	Model      string
	FormatMode FormatMode
	HTTPClient *http.Client
}

// NewClient builds a Client. timeout bounds a single Chat call.
func NewClient(baseURL, model string, formatMode FormatMode, timeout time.Duration) *Client {
	return &Client{
		BaseURL:    baseURL,
		Model:      model,
		FormatMode: formatMode,
		HTTPClient: &http.Client{Timeout: timeout},
	}
}

// Chat sends a system+user prompt pair and returns the assistant's raw
// message content. schema is the JSON-Schema document to use when
// FormatMode is FormatSchema; it is ignored for other modes.
func (c *Client) Chat(ctx context.Context, systemPrompt, userPrompt, schema string) (string, error) {
	req := chatRequest{
		Model: c.Model,
		Messages: []ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Stream: false,
	}

	switch c.FormatMode {
	case FormatSchema:
		req.Format = json.RawMessage(schema)
	case FormatJSON:
		req.Format = json.RawMessage(`"json"`)
	case FormatNone:
		// leave Format nil
	default:
		return "", fmt.Errorf("ollama: unknown format mode %q", c.FormatMode)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("ollama: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ollama: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("ollama: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ollama: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama: server returned %d: %s", resp.StatusCode, truncate(string(respBody), 500))
	}

	var cr chatResponse
	if err := json.Unmarshal(respBody, &cr); err != nil {
		return "", fmt.Errorf("ollama: decode response: %w", err)
	}

	return cr.Message.Content, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}
