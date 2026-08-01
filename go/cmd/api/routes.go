package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/julienschmidt/httprouter"
)

// routes registers all HTTP handlers on an httprouter. API routes live under
// /api; everything else falls through to the SPA handler so the Vue app's
// client-side router can take over. New resource handlers (auth, push, …) are
// added here, grouped with related routes, as the API grows.
func (app *application) routes() http.Handler {
	router := httprouter.New()

	// Unknown routes: /api/* → JSON 404; anything else → SPA fallback so a hard
	// reload on a client-side route still returns index.html.
	router.NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			app.notFoundAPIHandler(w, r)
			return
		}
		app.spaHandler().ServeHTTP(w, r)
	})
	router.MethodNotAllowed = http.HandlerFunc(app.MethodNotAllowedResponse)

	// API routes (JSON).
	router.HandlerFunc(http.MethodGet, "/api/healthcheck", app.healthcheckHandler)
	router.HandlerFunc(http.MethodPost, "/api/auth/request-pin", app.requestPinHandler)
	router.HandlerFunc(http.MethodPost, "/api/auth/verify", app.verifyPinHandler)
	router.HandlerFunc(http.MethodPost, "/api/auth/logout", app.logoutHandler)
	router.HandlerFunc(http.MethodGet, "/api/me", app.requireAuth(app.meHandler))
	router.HandlerFunc(http.MethodGet, "/api/push/public-key", app.pushPublicKeyHandler)
	router.HandlerFunc(http.MethodPost, "/api/push/subscription", app.requireAuth(app.createPushSubscriptionHandler))

	return router
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
