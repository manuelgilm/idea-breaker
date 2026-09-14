// Package gemini implements the llm.Provider contract for the Google Gemini
// (generativelanguage) API.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"aibreak/internal/llm"
)

const defaultBaseURL = "https://generativelanguage.googleapis.com"

// Provider is a Gemini-compatible chat completions client.
type Provider struct {
	mu      sync.RWMutex
	apiKey  string
	baseURL string
	client  *http.Client
}

// Option configures a Provider.
type Option func(*Provider)

// WithBaseURL overrides the API base URL (e.g. for a fake server).
func WithBaseURL(u string) Option {
	return func(p *Provider) { p.baseURL = u }
}

// WithHTTPClient overrides the HTTP client.
func WithHTTPClient(c *http.Client) Option {
	return func(p *Provider) { p.client = c }
}

// New creates a Gemini provider.
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

// SetAPIKey replaces the API key used for subsequent requests.
func (p *Provider) SetAPIKey(key string) {
	p.mu.Lock()
	p.apiKey = key
	p.mu.Unlock()
}

type part struct {
	Text string `json:"text"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type systemInstruction struct {
	Parts []part `json:"parts"`
}

type generationConfig struct {
	Temperature      float64 `json:"temperature,omitempty"`
	MaxOutputTokens  int     `json:"maxOutputTokens,omitempty"`
	ResponseMIMEType string  `json:"responseMimeType,omitempty"`
}

type generateContentRequest struct {
	SystemInstruction *systemInstruction `json:"systemInstruction,omitempty"`
	Contents          []content          `json:"contents"`
	GenerationConfig  generationConfig   `json:"generationConfig,omitempty"`
}

type generateContentResponse struct {
	Candidates []struct {
		Content struct {
			Parts []part `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete performs a single generateContent request. The engine's "system"
// message is mapped to Gemini's systemInstruction; other messages become
// contents entries.
func (p *Provider) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	var sys *systemInstruction
	contents := make([]content, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == "system" {
			sys = &systemInstruction{Parts: []part{{Text: m.Content}}}
			continue
		}
		contents = append(contents, content{Role: m.Role, Parts: []part{{Text: m.Content}}})
	}

	cfg := generationConfig{
		Temperature:     req.Temperature,
		MaxOutputTokens: req.MaxTokens,
	}
	if req.JSONMode {
		cfg.ResponseMIMEType = "application/json"
	}

	body := generateContentRequest{
		SystemInstruction: sys,
		Contents:          contents,
		GenerationConfig:  cfg,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return llm.Response{}, fmt.Errorf("%w: %v", llm.ErrProvider, err)
	}

	url := p.baseURL + "/v1beta/models/" + req.Model + ":generateContent"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return llm.Response{}, fmt.Errorf("%w: %v", llm.ErrProvider, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	p.mu.RLock()
	httpReq.Header.Set("x-goog-api-key", p.apiKey)
	p.mu.RUnlock()

	client := p.client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(httpReq)
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
		return llm.Response{}, fmt.Errorf("%w: status %d: %s", llm.ErrProvider, resp.StatusCode, truncateBody(respBody))
	}

	var parsed generateContentResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return llm.Response{}, fmt.Errorf("%w: %v", llm.ErrProvider, err)
	}
	if parsed.Error != nil {
		return llm.Response{}, fmt.Errorf("%w: %s", llm.ErrProvider, parsed.Error.Message)
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return llm.Response{}, fmt.Errorf("%w: no candidates", llm.ErrProvider)
	}

	return llm.Response{Content: parsed.Candidates[0].Content.Parts[0].Text}, nil
}

// truncateBody returns a bounded copy of the response body for error messages.
func truncateBody(b []byte) string {
	const max = 300
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "…"
}
