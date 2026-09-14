// Package llm defines the provider contract used by the engine.
package llm

import (
	"context"
	"errors"
)

// Message is a single chat message.
type Message struct {
	Role    string // "system" | "user"
	Content string
}

// Request is a chat completion request.
type Request struct {
	Model       string
	Messages    []Message
	Temperature float64
	MaxTokens   int
	JSONMode    bool
}

// Response is a chat completion response.
type Response struct {
	Content string
}

// Provider performs chat completions. Implementations surface the typed errors
// defined below so callers can classify failures.
type Provider interface {
	Complete(ctx context.Context, req Request) (Response, error)
}

// KeyedProvider is a Provider whose API key can be swapped at runtime (used by
// the desktop settings screen to update credentials in place).
type KeyedProvider interface {
	Provider
	SetAPIKey(key string)
}

var (
	// ErrRateLimited indicates an upstream rate limit (HTTP 429).
	ErrRateLimited = errors.New("llm: rate limited")
	// ErrAuth indicates an authentication/authorization failure (HTTP 401/403).
	ErrAuth = errors.New("llm: auth failure")
	// ErrProvider indicates a network or 5xx upstream failure.
	ErrProvider = errors.New("llm: provider failure")
)

// IsRetryable reports whether an error should be retried: rate limits,
// provider/transport failures, and timeouts are retryable; auth and invalid
// requests are not.
func IsRetryable(err error) bool {
	return errors.Is(err, ErrRateLimited) ||
		errors.Is(err, ErrProvider) ||
		errors.Is(err, context.DeadlineExceeded)
}
