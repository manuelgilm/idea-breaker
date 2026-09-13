package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"aibreak/internal/llm"
)

func TestComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hello"}}]}`))
	}))
	defer srv.Close()

	p := New("key", WithBaseURL(srv.URL))
	resp, err := p.Complete(context.Background(), llm.Request{
		Model:     "m",
		Messages:  []llm.Message{{Role: "user", Content: "hi"}},
		JSONMode:  true,
		MaxTokens: 10,
	})
	require.NoError(t, err)
	assert.Equal(t, "hello", resp.Content)
}

func TestCompleteRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	p := New("key", WithBaseURL(srv.URL))
	_, err := p.Complete(context.Background(), llm.Request{Model: "m"})
	assert.ErrorIs(t, err, llm.ErrRateLimited)
}

func TestCompleteAuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := New("key", WithBaseURL(srv.URL))
	_, err := p.Complete(context.Background(), llm.Request{Model: "m"})
	assert.ErrorIs(t, err, llm.ErrAuth)
}

// TestSetAPIKeyConcurrent exercises SetAPIKey while Complete is in flight; run
// under -race it guards the apiKey field access.
func TestSetAPIKeyConcurrent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	p := New("initial", WithBaseURL(srv.URL))

	const n = 32
	done := make(chan struct{}, n)
	for i := 0; i < n; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			_, _ = p.Complete(context.Background(), llm.Request{Model: "m"})
		}()
	}
	for i := 0; i < n; i++ {
		p.SetAPIKey("updated")
	}
	for i := 0; i < n; i++ {
		<-done
	}
}
