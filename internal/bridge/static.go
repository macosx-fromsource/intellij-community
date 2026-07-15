package bridge

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:web/dist
var webFS embed.FS

// registerStatic serves the embedded single-page application. Requests that do
// not match an embedded asset fall back to index.html so that client-side
// routing works. Requests under /api, /openapi, /docs and /schemas are left for
// Huma to handle and are never intercepted here.
func registerStatic(mux *http.ServeMux) {
	dist, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		// The embedded directory is compiled in, so this can only fail due to a
		// programming error; there is nothing sensible to serve in that case.
		panic(err)
	}

	fileServer := http.FileServer(http.FS(dist))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if isReservedAPIPath(r.URL.Path) {
			http.NotFound(w, r)

			return
		}

		trimmed := strings.TrimPrefix(r.URL.Path, "/")
		if trimmed == "" {
			trimmed = "index.html"
		}

		if _, statErr := fs.Stat(dist, trimmed); statErr != nil {
			// Unknown asset: serve index.html for SPA client-side routing.
			r.URL.Path = "/"
		}

		fileServer.ServeHTTP(w, r)
	})
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
