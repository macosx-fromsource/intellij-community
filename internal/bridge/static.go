package bridge

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:web/dist
var webFS embed.FS

// localAuthMarker is injected into index.html in local serve mode so the SPA
// knows requests are already authenticated by the caller's kubeconfig and can
// hide the bearer-token entry.
const localAuthMarker = `<script>window.__BRIDGE_AUTH__="local"</script>`

// registerStatic serves the embedded single-page application. Requests that do
// not match an embedded asset fall back to index.html so that client-side
// routing works. Requests under /api, /openapi, /docs and /schemas are left for
// Huma to handle and are never intercepted here. When localMode is true the
// served index.html carries the local-auth marker.
func registerStatic(mux *http.ServeMux, localMode bool) {
	dist, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		// The embedded directory is compiled in, so this can only fail due to a
		// programming error; there is nothing sensible to serve in that case.
		panic(err)
	}

	fileServer := http.FileServer(http.FS(dist))
	indexHTML := indexDocument(dist, localMode)

	serveIndex := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if isReservedAPIPath(r.URL.Path) {
			http.NotFound(w, r)

			return
		}

		trimmed := strings.TrimPrefix(r.URL.Path, "/")
		if trimmed == "" {
			serveIndex(w)

			return
		}

		if _, statErr := fs.Stat(dist, trimmed); statErr != nil {
			// Unknown asset: serve index.html for SPA client-side routing.
			serveIndex(w)

			return
		}

		fileServer.ServeHTTP(w, r)
	})
}

// indexDocument reads the embedded index.html, injecting the local-auth marker
// before </head> when localMode is set. It returns a small placeholder when the
// SPA has not been built (web/dist has no index.html), matching the previous
// best-effort behavior.
func indexDocument(dist fs.FS, localMode bool) []byte {
	data, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		return []byte("<!doctype html><title>GitLab Operator Bridge</title>SPA not built")
	}

	if !localMode {
		return data
	}

	marker := []byte(localAuthMarker)

	if idx := bytes.Index(data, []byte("</head>")); idx >= 0 {
		out := make([]byte, 0, len(data)+len(marker))
		out = append(out, data[:idx]...)
		out = append(out, marker...)
		out = append(out, data[idx:]...)

		return out
	}

	return append(marker, data...)
}

// isReservedAPIPath reports whether the path belongs to the Huma-served API and
// must not be handled by the static file server.
func isReservedAPIPath(path string) bool {
	for _, prefix := range []string{"/api/", "/openapi", "/docs", "/schemas"} {
		if path == prefix || strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}
