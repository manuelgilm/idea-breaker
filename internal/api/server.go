// Package api implements the aibreak HTTP API.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"aibreak/internal/engine"
	"aibreak/internal/llm"
	"aibreak/internal/service"
)

// Server exposes the service over HTTP.
type Server struct {
	svc          *service.Service
	router       ProviderRouter
	fallbackKeys map[string]string
}

// ProviderRouter selects the active LLM provider and updates its model at
// runtime. The concrete llm.Switchable satisfies it. It may be nil for
// deployments that do not expose provider settings.
type ProviderRouter interface {
	SetActive(name string)
	Active() string
	SetModel(name, model string)
	Names() []string
	Model(name string) string
}

// Option configures a Server.
type Option func(*Server)

// WithProviderRouter enables the provider endpoints against a switchable
// provider router.
func WithProviderRouter(r ProviderRouter) Option {
	return func(s *Server) { s.router = r }
}

// WithFallbackKeys sets the per-provider config/env API keys used to report
// whether a provider has a usable key (the web service has no keyring).
func WithFallbackKeys(m map[string]string) Option {
	return func(s *Server) { s.fallbackKeys = m }
}

// New creates an API server.
func New(svc *service.Service, opts ...Option) *Server {
	s := &Server{svc: svc, fallbackKeys: map[string]string{}}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Handler returns the configured HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /v1/ideas", s.createIdea)
	mux.HandleFunc("GET /v1/ideas", s.listIdeas)
	mux.HandleFunc("GET /v1/ideas/cards", s.listIdeaCards)
	mux.HandleFunc("GET /v1/ideas/{id}", s.getIdea)
	mux.HandleFunc("PATCH /v1/ideas/{id}", s.updateIdea)
	mux.HandleFunc("DELETE /v1/ideas/{id}", s.deleteIdea)

	mux.HandleFunc("POST /v1/ideas/{id}/evaluate", s.evaluate)
	mux.HandleFunc("POST /v1/ideas/{id}/evaluate/stream", s.evaluateStream)
	mux.HandleFunc("GET /v1/ideas/{id}/evaluations", s.listRuns)

	mux.HandleFunc("POST /v1/ideas/{id}/feedback", s.addFeedback)
	mux.HandleFunc("GET /v1/ideas/{id}/feedback", s.listFeedback)
	mux.HandleFunc("DELETE /v1/ideas/{id}/feedback/{fid}", s.deleteFeedback)

	mux.HandleFunc("POST /v1/ideas/{id}/resources", s.addResource)
	mux.HandleFunc("GET /v1/ideas/{id}/resources", s.listResources)
	mux.HandleFunc("DELETE /v1/ideas/{id}/resources/{rid}", s.deleteResource)

	mux.HandleFunc("GET /v1/personas", s.listPersonas)
	mux.HandleFunc("POST /v1/personas", s.createPersona)
	mux.HandleFunc("PATCH /v1/personas/{id}", s.updatePersona)
	mux.HandleFunc("DELETE /v1/personas/{id}", s.deletePersona)

	mux.HandleFunc("GET /v1/providers", s.listProviders)
	mux.HandleFunc("PUT /v1/providers/{name}", s.updateProvider)

	mux.HandleFunc("GET /healthz", s.healthz)

	return mux
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	status, code := errorCode(err)
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": err.Error()},
	})
}

func errorCode(err error) (int, string) {
	switch {
	case errors.Is(err, service.ErrValidation),
		errors.Is(err, engine.ErrEmptyPersonas),
		errors.Is(err, engine.ErrZeroWeight):
		return http.StatusBadRequest, "validation_error"
	case errors.Is(err, service.ErrNotFound):
		return http.StatusNotFound, "not_found"
	case errors.Is(err, service.ErrConflict):
		return http.StatusConflict, "conflict"
	case errors.Is(err, llm.ErrRateLimited):
		return http.StatusTooManyRequests, "rate_limited"
	case errors.Is(err, llm.ErrProvider),
		errors.Is(err, engine.ErrNoResults):
		return http.StatusBadGateway, "llm_error"
	default:
		return http.StatusInternalServerError, "internal"
	}
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("%w: %v", service.ErrValidation, err)
	}
	return nil
}
