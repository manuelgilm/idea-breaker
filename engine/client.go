package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	defaultBaseURL = "https://api.openai.com"
	defaultModel   = "gpt-4o-mini"
)

// Caller makes a single LLM chat call from a system prompt and a user message.
// It is intentionally free of any auth, persona, or domain concern: credentials
// are bound to the concrete caller at construction time, and both the persona
// fan-out (BreakAll) and the synthesizer (Synthesize) build their own messages.
type Caller interface {
	Chat(ctx context.Context, system, user string) (string, error)
}

// HTTPCaller makes real LLM HTTP calls to an OpenAI-compatible endpoint.
type HTTPCaller struct {
	APIKey  string
	BaseURL string
	Model   string
	client  *http.Client
}

// NewHTTPCaller returns a caller with the given settings, applying the
// OpenAI defaults for any empty BaseURL/Model.
func NewHTTPCaller(apiKey, baseURL, model string) HTTPCaller {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if model == "" {
		model = defaultModel
	}
	return HTTPCaller{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
		client:  http.DefaultClient,
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// Chat sends the system prompt and user message to the provider and returns
// the model's text content.
func (c HTTPCaller) Chat(ctx context.Context, system, user string) (string, error) {
	body := chatRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := c.client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("provider returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var decoded chatResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return "", fmt.Errorf("provider returned no choices")
	}

	return decoded.Choices[0].Message.Content, nil
}
