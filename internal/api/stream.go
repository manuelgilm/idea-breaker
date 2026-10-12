package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"aibreak/internal/domain"
)

// personaResult is the per-persona payload streamed over SSE.
type personaResult struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Score     int    `json:"score"`
	Rationale string `json:"rationale"`
	Error     string `json:"error,omitempty"`
}

// evaluateStream runs an evaluation and streams progress as Server-Sent
// Events: one "persona" event per completed persona, then either a "done"
// event carrying the FeasibilityScore or an "error" event carrying the error
// envelope. Validation that fails before the run starts returns a normal
// non-streaming error response.
func (s *Server) evaluateStream(w http.ResponseWriter, r *http.Request) {
	var req evaluateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	ideaID := r.PathValue("id")
	if _, err := s.svc.GetIdea(r.Context(), ideaID); err != nil {
		writeError(w, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, fmt.Errorf("streaming unsupported"))
		return
	}
	names, err := s.personaNames(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	score, err := s.svc.EvaluateWithProgress(r.Context(), ideaID, req.Personas, req.Summary, func(ev domain.Evaluation) {
		writeSSE(w, "persona", personaResult{
			Name:      names[ev.PersonaID],
			Status:    ev.Status,
			Score:     ev.Score,
			Rationale: ev.Rationale,
			Error:     ev.Error,
		})
		flusher.Flush()
	})
	if err != nil {
		writeSSE(w, "error", errorEnvelope(err))
		flusher.Flush()
		return
	}
	writeSSE(w, "done", score)
	flusher.Flush()
}

func (s *Server) personaNames(ctx context.Context) (map[string]string, error) {
	personas, err := s.svc.ListPersonas(ctx)
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(personas))
	for _, p := range personas {
		names[p.ID] = p.Name
	}
	return names, nil
}

func writeSSE(w io.Writer, event string, data any) {
	b, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
}

// errorEnvelope mirrors the JSON error body used by non-streaming responses.
func errorEnvelope(err error) map[string]any {
	_, code := errorCode(err)
	return map[string]any{
		"error": map[string]string{"code": code, "message": err.Error()},
	}
}
