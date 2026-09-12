package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"aibreak/internal/domain"
	"aibreak/internal/engine"
	"aibreak/internal/llm"
	"aibreak/internal/registry/sqlite"
	"aibreak/internal/service"
)

type fixedProvider struct{}

func (fixedProvider) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	if strings.Contains(req.Messages[0].Content, "SYNTHESIZER") {
		return llm.Response{Content: `{"verdict":"promising","summary":"Good upside, some risk."}`}, nil
	}
	return llm.Response{Content: `{"score":3,"rationale":"ok"}`}, nil
}

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	store, err := sqlite.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	svc := service.New(store, engine.New(fixedProvider{}))
	return New(svc).Handler()
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&v))
	return v
}

func TestCreateIdea(t *testing.T) {
	h := newTestHandler(t)
	rec := doJSON(t, h, http.MethodPost, "/v1/ideas", map[string]any{"title": "X"})
	assert.Equal(t, http.StatusCreated, rec.Code)

	idea := decode[domain.Idea](t, rec)
	assert.NotEmpty(t, idea.ID)
	assert.Equal(t, "X", idea.Title)
}

func TestGetUnknownIdea(t *testing.T) {
	h := newTestHandler(t)
	rec := doJSON(t, h, http.MethodGet, "/v1/ideas/nope", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&env))
	assert.Equal(t, "not_found", env.Error.Code)
}

func TestCreateIdeaDuplicateWarningHeader(t *testing.T) {
	h := newTestHandler(t)
	doJSON(t, h, http.MethodPost, "/v1/ideas", map[string]any{"title": "Note App"})

	rec := doJSON(t, h, http.MethodPost, "/v1/ideas", map[string]any{"title": "note app"})
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("X-Warning"))
}

func TestUpdateIdea(t *testing.T) {
	h := newTestHandler(t)
	rec := doJSON(t, h, http.MethodPost, "/v1/ideas", map[string]any{"title": "X", "body": "b"})
	idea := decode[domain.Idea](t, rec)

	rec = doJSON(t, h, http.MethodPatch, "/v1/ideas/"+idea.ID, map[string]any{"title": "Y"})
	assert.Equal(t, http.StatusOK, rec.Code)
	updated := decode[domain.Idea](t, rec)
	assert.Equal(t, "Y", updated.Title)
	assert.Equal(t, "b", updated.Body, "body preserved")
}

func TestEvaluateAndHistory(t *testing.T) {
	h := newTestHandler(t)
	rec := doJSON(t, h, http.MethodPost, "/v1/ideas", map[string]any{"title": "X"})
	idea := decode[domain.Idea](t, rec)

	rec = doJSON(t, h, http.MethodPost, "/v1/ideas/"+idea.ID+"/evaluate", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	score := decode[domain.FeasibilityScore](t, rec)
	assert.Equal(t, 60.0, score.Total)
	assert.Equal(t, 3, score.Requested)
	assert.Equal(t, 3, score.Responded)
	assert.Equal(t, 0.0, score.Spread, "uniform mocked scores give spread 0")

	rec = doJSON(t, h, http.MethodGet, "/v1/ideas/"+idea.ID+"/evaluations", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	runs := decode[[]domain.FeasibilityScore](t, rec)
	require.Len(t, runs, 1)
	assert.Equal(t, score.RunID, runs[0].RunID)
}

func TestEvaluateSummary(t *testing.T) {
	h := newTestHandler(t)
	rec := doJSON(t, h, http.MethodPost, "/v1/ideas", map[string]any{"title": "X"})
	idea := decode[domain.Idea](t, rec)

	rec = doJSON(t, h, http.MethodPost, "/v1/ideas/"+idea.ID+"/evaluate", map[string]any{"summary": true})
	assert.Equal(t, http.StatusOK, rec.Code)
	score := decode[domain.FeasibilityScore](t, rec)
	assert.Equal(t, "promising", score.Verdict)
	assert.Equal(t, "Good upside, some risk.", score.Summary)

	rec = doJSON(t, h, http.MethodGet, "/v1/ideas/"+idea.ID+"/evaluations", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	runs := decode[[]domain.FeasibilityScore](t, rec)
	require.Len(t, runs, 1)
	assert.Equal(t, "promising", runs[0].Verdict)
	assert.Equal(t, "Good upside, some risk.", runs[0].Summary)
}

func TestFeedback(t *testing.T) {
	h := newTestHandler(t)
	rec := doJSON(t, h, http.MethodPost, "/v1/ideas", map[string]any{"title": "X"})
	idea := decode[domain.Idea](t, rec)

	rec = doJSON(t, h, http.MethodPost, "/v1/ideas/"+idea.ID+"/feedback", map[string]any{"author": "a", "score": 3})
	assert.Equal(t, http.StatusCreated, rec.Code)
	f := decode[domain.Feedback](t, rec)
	assert.Equal(t, "a", f.Author)
}

func TestResources(t *testing.T) {
	h := newTestHandler(t)
	rec := doJSON(t, h, http.MethodPost, "/v1/ideas", map[string]any{"title": "X"})
	idea := decode[domain.Idea](t, rec)

	rec = doJSON(t, h, http.MethodPost, "/v1/ideas/"+idea.ID+"/resources", map[string]any{"url": "https://x.com", "kind": "repo"})
	assert.Equal(t, http.StatusCreated, rec.Code)
	r := decode[domain.Resource](t, rec)
	assert.Equal(t, "https://x.com", r.URL)
}

func TestInvalidBody(t *testing.T) {
	h := newTestHandler(t)
	rec := doJSON(t, h, http.MethodPost, "/v1/ideas", map[string]any{"bogus": 1})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPersonas(t *testing.T) {
	h := newTestHandler(t)
	rec := doJSON(t, h, http.MethodGet, "/v1/personas", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	personas := decode[[]domain.Persona](t, rec)
	assert.Len(t, personas, 3)

	rec = doJSON(t, h, http.MethodPost, "/v1/personas", map[string]any{
		"id": "investor", "name": "Investor", "system_prompt": "p",
	})
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestUpdatePersona(t *testing.T) {
	h := newTestHandler(t)
	rec := doJSON(t, h, http.MethodPost, "/v1/personas", map[string]any{
		"id": "investor", "name": "Investor", "system_prompt": "p",
	})
	require.Equal(t, http.StatusCreated, rec.Code)

	rec = doJSON(t, h, http.MethodPatch, "/v1/personas/investor", map[string]any{
		"name": "Investor v2", "version": "2.0.0",
	})
	assert.Equal(t, http.StatusOK, rec.Code)
	p := decode[domain.Persona](t, rec)
	assert.Equal(t, "Investor v2", p.Name)
	assert.Equal(t, "2.0.0", p.Version)
	assert.Equal(t, "p", p.SystemPrompt, "unprovided prompt preserved")

	rec = doJSON(t, h, http.MethodPatch, "/v1/personas/investor", map[string]any{
		"name": "Nope",
	})
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = doJSON(t, h, http.MethodPatch, "/v1/personas/skeptic", map[string]any{
		"name": "Nope", "version": "9.0.0",
	})
	assert.Equal(t, http.StatusConflict, rec.Code)

	rec = doJSON(t, h, http.MethodPatch, "/v1/personas/nope", map[string]any{
		"name": "Nope", "version": "1.0.0",
	})
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
