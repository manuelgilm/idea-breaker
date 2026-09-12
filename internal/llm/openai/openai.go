// Package openai implements the llm.Provider contract for the OpenAI-compatible
// chat completions API.
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"aibreak/internal/llm"
)

const defaultBaseURL = "https://api.openai.com/v1"

// Provider is an OpenAI-compatible chat completions client.
type Provider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// Option configures a Provider.
type Option func(*Provider)

// WithBaseURL overrides the API base URL (e.g. for a fake server or a
// local/Ollama-compatible endpoint).
func WithBaseURL(u string) Option {
	return func(p *Provider) { p.baseURL = u }
}

// WithHTTPClient overrides the HTTP client.
func WithHTTPClient(c *http.Client) Option {
	return func(p *Provider) { p.client = c }
}

// New creates an OpenAI-compatible provider.
func New(apiKey string, opts ...Option) *Provider {
	p := &Provider{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

type completionRequest struct {
	Model          string               `json:"model"`
	Messages       []completionMessage  `json:"messages"`
	Temperature    float64              `json:"temperature"`
	MaxTokens      int                  `json:"max_tokens"`
	ResponseFormat *completionResponseF `json:"response_format,omitempty"`
}

type completionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type completionResponseF struct {
	Type string `json:"type"`
}

type completionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete performs a single chat completion.
func (p *Provider) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	msgs := make([]completionMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, completionMessage{Role: m.Role, Content: m.Content})
	}

	body := completionRequest{
		Model:       req.Model,
		Messages:    msgs,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}
	if req.JSONMode {
		body.ResponseFormat = &completionResponseF{Type: "json_object"}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return llm.Response{}, fmt.Errorf("%w: %v", llm.ErrProvider, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return llm.Response{}, fmt.Errorf("%w: %v", llm.ErrProvider, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return llm.Response{}, fmt.Errorf("%w: %v", llm.ErrProvider, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return llm.Response{}, fmt.Errorf("%w: %v", llm.ErrProvider, err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// fall through to parse
	case http.StatusTooManyRequests:
		return llm.Response{}, llm.ErrRateLimited
	case http.StatusUnauthorized, http.StatusForbidden:
		return llm.Response{}, llm.ErrAuth
	default:
		return llm.Response{}, fmt.Errorf("%w: status %d", llm.ErrProvider, resp.StatusCode)
	}

	var parsed completionResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return llm.Response{}, fmt.Errorf("%w: %v", llm.ErrProvider, err)
	}
	if parsed.Error != nil {
		return llm.Response{}, fmt.Errorf("%w: %s", llm.ErrProvider, parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return llm.Response{}, fmt.Errorf("%w: no choices", llm.ErrProvider)
	}

	return llm.Response{Content: parsed.Choices[0].Message.Content}, nil
}
