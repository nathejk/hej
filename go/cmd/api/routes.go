package main

import (
	"net/http"
	"os"
	"path/filepath"
)

// routes registers all HTTP handlers. API routes live under /api; everything
// else falls through to the SPA handler so the Vue app's client-side router can
// take over. New resource handlers (auth, push, …) are added here, grouped with
// related routes, as the API grows.
func (app *application) routes() http.Handler {
	mux := http.NewServeMux()

	// API routes (JSON).
	mux.HandleFunc("GET /api/healthcheck", app.healthcheckHandler)

	// Any other /api/* path that isn't matched above is a JSON 404 rather than
	// falling through to the SPA.
	mux.HandleFunc("/api/", app.notFoundAPIHandler)

	// SPA fallback: serve static assets from the web root; unknown non-API
	// paths return index.html so client-side routing works after a hard reload.
	mux.Handle("/", app.spaHandler())

	return mux
}

// spaHandler serves the built single-page app from the configured web root. In
// production this directory holds the compiled Vue bundle; in dev it's a
// placeholder (the Vite dev server serves the real SPA and proxies /api here).
func (app *application) spaHandler() http.Handler {
	root := app.config.webRoot
	fileServer := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested := filepath.Join(root, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(requested); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(root, "index.html"))
	})
}
