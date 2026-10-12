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

type fakeRouter struct {
	names  []string
	active string
	models map[string]string
}

func (r *fakeRouter) Names() []string             { return r.names }
func (r *fakeRouter) Active() string              { return r.active }
func (r *fakeRouter) SetActive(name string)       { r.active = name }
func (r *fakeRouter) Model(name string) string    { return r.models[name] }
func (r *fakeRouter) SetModel(name, model string) { r.models[name] = model }

func newTestServer(t *testing.T, opts ...Option) (*Server, *service.Service) {
	t.Helper()
	store, err := sqlite.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	svc := service.New(store, engine.New(fixedProvider{}))
	return New(svc, opts...), svc
}

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	srv, _ := newTestServer(t)
	return srv.Handler()
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
		"name": "Investor", "system_prompt": "p",
	})
	assert.Equal(t, http.StatusCreated, rec.Code)
	created := decode[domain.Persona](t, rec)
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, 1, created.Version)
	assert.Equal(t, 1.0, created.Weight, "omitted weight defaults to 1.0")
}

func TestUpdatePersona(t *testing.T) {
	h := newTestHandler(t)
	rec := doJSON(t, h, http.MethodPost, "/v1/personas", map[string]any{
		"name": "Investor", "system_prompt": "p",
	})
	require.Equal(t, http.StatusCreated, rec.Code)
	created := decode[domain.Persona](t, rec)

	rec = doJSON(t, h, http.MethodPatch, "/v1/personas/"+created.ID, map[string]any{
		"name": "Investor v2",
	})
	assert.Equal(t, http.StatusOK, rec.Code)
	p := decode[domain.Persona](t, rec)
	assert.Equal(t, "Investor v2", p.Name)
	assert.Equal(t, 2, p.Version, "version auto-increments")
	assert.Equal(t, "p", p.SystemPrompt, "unprovided prompt preserved")

	rec = doJSON(t, h, http.MethodPatch, "/v1/personas/"+created.ID, map[string]any{
		"name": "",
	})
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = doJSON(t, h, http.MethodPatch, "/v1/personas/"+created.ID, map[string]any{})
	assert.Equal(t, http.StatusBadRequest, rec.Code, "empty patch rejected")

	rec = doJSON(t, h, http.MethodPatch, "/v1/personas/"+created.ID, map[string]any{
		"weight": 0,
	})
	assert.Equal(t, http.StatusOK, rec.Code)
	p = decode[domain.Persona](t, rec)
	assert.Equal(t, 1.0, p.Weight, "zero weight normalized to 1.0")

	rec = doJSON(t, h, http.MethodPatch, "/v1/personas/skeptic", map[string]any{
		"name": "Nope",
	})
	assert.Equal(t, http.StatusConflict, rec.Code)

	rec = doJSON(t, h, http.MethodPatch, "/v1/personas/nope", map[string]any{
		"name": "Nope",
	})
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHealthz(t *testing.T) {
	h := newTestHandler(t)
	rec := doJSON(t, h, http.MethodGet, "/healthz", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := decode[map[string]string](t, rec)
	assert.Equal(t, "ok", body["status"])
}

func TestIdeaCards(t *testing.T) {
	h := newTestHandler(t)
	rec := doJSON(t, h, http.MethodPost, "/v1/ideas", map[string]any{"title": "X"})
	idea := decode[domain.Idea](t, rec)

	rec = doJSON(t, h, http.MethodGet, "/v1/ideas/cards", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	cards := decode[[]IdeaCard](t, rec)
	require.Len(t, cards, 1)
	assert.Equal(t, idea.ID, cards[0].ID)
	assert.Nil(t, cards[0].Score, "no run yet")

	doJSON(t, h, http.MethodPost, "/v1/ideas/"+idea.ID+"/evaluate", nil)

	rec = doJSON(t, h, http.MethodGet, "/v1/ideas/cards", nil)
	cards = decode[[]IdeaCard](t, rec)
	require.Len(t, cards, 1)
	require.NotNil(t, cards[0].Score)
	assert.Equal(t, 60.0, *cards[0].Score)
}

func TestProviders(t *testing.T) {
	router := &fakeRouter{
		names:  []string{"openai", "gemini"},
		active: "openai",
		models: map[string]string{"openai": "gpt-4o-mini", "gemini": "gemini-3.8-flash"},
	}
	srv, _ := newTestServer(t, WithProviderRouter(router), WithFallbackKeys(map[string]string{"openai": "sk-x"}))
	h := srv.Handler()

	rec := doJSON(t, h, http.MethodGet, "/v1/providers", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	infos := decode[[]ProviderInfo](t, rec)
	require.Len(t, infos, 2)
	assert.Equal(t, "openai", infos[0].Name)
	assert.True(t, infos[0].Active)
	assert.True(t, infos[0].HasKey)
	assert.Equal(t, "gemini-3.8-flash", infos[1].Model)
	assert.False(t, infos[1].HasKey)
}

func TestUpdateProvider(t *testing.T) {
	router := &fakeRouter{
		names:  []string{"openai", "gemini"},
		active: "openai",
		models: map[string]string{"openai": "gpt-4o-mini", "gemini": "gemini-3.8-flash"},
	}
	srv, svc := newTestServer(t, WithProviderRouter(router))
	h := srv.Handler()

	rec := doJSON(t, h, http.MethodPut, "/v1/providers/openai", map[string]any{"model": "gpt-4o"})
	require.Equal(t, http.StatusOK, rec.Code)
	info := decode[ProviderInfo](t, rec)
	assert.Equal(t, "gpt-4o", info.Model)
	assert.Equal(t, "gpt-4o", router.models["openai"])

	rec = doJSON(t, h, http.MethodPut, "/v1/providers/gemini", map[string]any{"active": true})
	require.Equal(t, http.StatusOK, rec.Code)
	_ = decode[ProviderInfo](t, rec)
	assert.Equal(t, "gemini", router.active)

	rec = doJSON(t, h, http.MethodGet, "/v1/providers", nil)
	infos := decode[[]ProviderInfo](t, rec)
	require.Len(t, infos, 2)
	var gemini ProviderInfo
	for _, i := range infos {
		if i.Name == "gemini" {
			gemini = i
		}
	}
	assert.True(t, gemini.Active, "gemini now active")

	active, err := svc.GetSetting(context.Background(), "active_provider")
	require.NoError(t, err)
	assert.Equal(t, "gemini", active, "active provider persisted")
}

func TestUpdateUnknownProvider(t *testing.T) {
	router := &fakeRouter{names: []string{"openai"}, active: "openai", models: map[string]string{}}
	srv, _ := newTestServer(t, WithProviderRouter(router))
	h := srv.Handler()

	rec := doJSON(t, h, http.MethodPut, "/v1/providers/nope", map[string]any{"model": "x"})
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestEvaluateStream(t *testing.T) {
	h := newTestHandler(t)
	rec := doJSON(t, h, http.MethodPost, "/v1/ideas", map[string]any{"title": "X"})
	idea := decode[domain.Idea](t, rec)

	rec = doJSON(t, h, http.MethodPost, "/v1/ideas/"+idea.ID+"/evaluate/stream", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	assert.Equal(t, "no", rec.Header().Get("X-Accel-Buffering"))

	body := rec.Body.String()
	assert.Equal(t, 3, strings.Count(body, "event: persona"))
	assert.Contains(t, body, "event: done")
	assert.Contains(t, body, `"total":60`)
}

func TestEvaluateStreamUnknownIdea(t *testing.T) {
	h := newTestHandler(t)
	rec := doJSON(t, h, http.MethodPost, "/v1/ideas/nope/evaluate/stream", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.NotEqual(t, "text/event-stream", rec.Header().Get("Content-Type"))
}

func TestEvaluateStreamAllFail(t *testing.T) {
	store, err := sqlite.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	svc := service.New(store, engine.New(alwaysFailProvider{}))
	h := New(svc).Handler()

	rec := doJSON(t, h, http.MethodPost, "/v1/ideas", map[string]any{"title": "X"})
	idea := decode[domain.Idea](t, rec)

	rec = doJSON(t, h, http.MethodPost, "/v1/ideas/"+idea.ID+"/evaluate/stream", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "event: error")
	assert.NotContains(t, body, "event: done")
}

type alwaysFailProvider struct{}

func (alwaysFailProvider) Complete(ctx context.Context, req llm.Request) (llm.Response, error) {
	return llm.Response{}, llm.ErrProvider
}
