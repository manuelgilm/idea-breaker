package engine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPCallerSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("expected auth header, got %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("expected content-type application/json, got %q", got)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}

		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != defaultModel {
			t.Errorf("expected model %q, got %q", defaultModel, req.Model)
		}
		if len(req.Messages) != 2 || req.Messages[0].Role != "system" || req.Messages[1].Role != "user" {
			t.Errorf("unexpected messages: %+v", req.Messages)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chatResponse{
			Choices: []struct {
				Message chatMessage `json:"message"`
			}{{Message: chatMessage{Role: "assistant", Content: "a thoughtful review"}}},
		})
	}))
	defer srv.Close()

	caller := NewHTTPCaller("sk-test", srv.URL, "")
	got, err := caller.Break(context.Background(), Persona{Name: "pessimist", SystemPrompt: "be negative"}, "my idea")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "a thoughtful review" {
		t.Errorf("expected %q, got %q", "a thoughtful review", got)
	}
}

func TestHTTPCallerNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("invalid api key"))
	}))
	defer srv.Close()

	caller := NewHTTPCaller("sk-test", srv.URL, "")
	_, err := caller.Break(context.Background(), Persona{Name: "pessimist", SystemPrompt: "be negative"}, "my idea")
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
}

func TestHTTPCallerDefaults(t *testing.T) {
	caller := NewHTTPCaller("sk-test", "", "")
	if caller.BaseURL != defaultBaseURL {
		t.Errorf("expected default base url %q, got %q", defaultBaseURL, caller.BaseURL)
	}
	if caller.Model != defaultModel {
		t.Errorf("expected default model %q, got %q", defaultModel, caller.Model)
	}
}
