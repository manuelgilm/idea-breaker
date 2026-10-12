package webui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func stubAPI() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Stub-Path", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	})
}

func TestServesSPAAtRoot(t *testing.T) {
	h := Handler(stubAPI())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `<div id="app">`)
}

func TestSPAFallbackForUnknownPath(t *testing.T) {
	h := Handler(stubAPI())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ideas/123", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `<div id="app">`)
}

func TestMissingAssetReturns404(t *testing.T) {
	h := Handler(stubAPI())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing.js", nil))

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAPIPassthrough(t *testing.T) {
	h := Handler(stubAPI())

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/personas", nil))
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "/v1/personas", rec.Header().Get("X-Stub-Path"))

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "/healthz", rec.Header().Get("X-Stub-Path"))
}
