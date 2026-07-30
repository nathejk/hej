package main

import "net/http"

// healthcheckHandler reports basic liveness and environment info. Used by
// container/orchestration health probes.
//
// NOTE: per repo rules all product endpoints must carry OpenAPI annotations.
// The annotation tool/convention is not yet chosen for this greenfield repo
// (tracked as a follow-up); healthcheck is infrastructure, and the product
// endpoints (auth, push) will be annotated once the convention is set.
func (app *application) healthcheckHandler(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"status": "available",
		"system_info": map[string]string{
			"environment": app.config.env,
		},
	}
	if err := app.WriteJSON(w, http.StatusOK, data, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// notFoundAPIHandler returns a JSON 404 for unmatched /api/* routes.
func (app *application) notFoundAPIHandler(w http.ResponseWriter, r *http.Request) {
	app.NotFoundResponse(w, r)
}
