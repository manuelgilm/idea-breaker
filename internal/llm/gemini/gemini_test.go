package gemini

import (
	"context"
	"encoding/json"
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
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"hello"}]}}]}`))
	}))
	defer srv.Close()

	p := New("key", WithBaseURL(srv.URL))
	resp, err := p.Complete(context.Background(), llm.Request{
		Model:    "gemini-2.5-flash",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "hello", resp.Content)
}

func TestCompleteMapsSystemAndJSON(t *testing.T) {
	var got generateContentRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`))
	}))
	defer srv.Close()

	p := New("key", WithBaseURL(srv.URL))
	_, err := p.Complete(context.Background(), llm.Request{
		Model: "gemini-2.5-flash",
		Messages: []llm.Message{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "hi"},
		},
		Temperature: 0,
		MaxTokens:   512,
		JSONMode:    true,
	})
	require.NoError(t, err)

	require.NotNil(t, got.SystemInstruction, "system message maps to systemInstruction")
	assert.Equal(t, "sys", got.SystemInstruction.Parts[0].Text)
	require.Len(t, got.Contents, 1, "only the user message becomes contents")
	assert.Equal(t, "user", got.Contents[0].Role)
	assert.Equal(t, "hi", got.Contents[0].Parts[0].Text)
	assert.Equal(t, "application/json", got.GenerationConfig.ResponseMIMEType)
	assert.Equal(t, 512, got.GenerationConfig.MaxOutputTokens)
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
