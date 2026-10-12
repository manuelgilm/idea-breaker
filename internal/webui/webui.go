// Package webui serves the built single-page frontend from the aibreakd
// binary. It embeds the compiled SPA (internal/webui/dist, produced by
// `make web-build`) and serves it at "/", returning the entrypoint for any
// path that is not an API route or a static asset. It contains no domain
// logic; the API is mounted alongside it.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// Handler mounts the API under /v1/ and /healthz and serves the embedded SPA
// for everything else, with an index.html fallback for client-side navigation.
func Handler(api http.Handler) http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("webui: embedded dist missing: " + err.Error())
	}
	mux := http.NewServeMux()
	mux.Handle("/v1/", api)
	mux.Handle("/healthz", api)
	mux.Handle("/", spa(sub))
	return mux
}

// spa serves files from sub and falls back to index.html for navigable,
// extensionless paths. A path that looks like an asset (its last segment has a
// file extension) but does not exist returns 404, so a missing script or
// stylesheet is never mis-served as HTML.
func spa(sub fs.FS) http.Handler {
	files := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" || isFile(sub, p) {
			files.ServeHTTP(w, r)
			return
		}
		if looksLikeAsset(p) {
			http.NotFound(w, r)
			return
		}
		serveIndex(files, w, r)
	})
}

// looksLikeAsset reports whether the last path segment has a file extension
// (e.g. "main.js", "img/logo.svg").
func looksLikeAsset(p string) bool {
	base := p
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		base = p[i+1:]
	}
	return strings.Contains(base, ".")
}

func serveIndex(files http.Handler, w http.ResponseWriter, r *http.Request) {
	r2 := r.Clone(r.Context())
	r2.URL.Path = "/"
	files.ServeHTTP(w, r2)
}

func isFile(sub fs.FS, name string) bool {
	f, err := sub.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()
	info, err := f.Stat()
	return err == nil && !info.IsDir()
}
