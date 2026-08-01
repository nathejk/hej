package main

import (
	"net/http"

	"nathejk.dk/internal/vcs"
)

// healthcheckHandler reports basic liveness and environment info. Used by
// container/orchestration health probes.
//
// OpenAPI annotations use swaggo/swag (registered as a `go tool`); regenerate
// the spec with:
//
//	go tool swag init -g cmd/api/main.go -o cmd/api/docs
//
// @Summary      Health check
// @Description  Reports API liveness and the running environment.
// @Tags         system
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /healthcheck [get]
func (app *application) healthcheckHandler(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"status": "available",
		"system_info": map[string]string{
			"environment": app.config.env,
			"version":     vcs.Version(),
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
