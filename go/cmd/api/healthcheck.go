package main

import (
	"context"
	"net/http"
	"time"

	"nathejk.dk/internal/vcs"
)

// dependencyStatus is one dependency's state in the healthcheck payload.
type dependencyStatus struct {
	// State is one of "up", "down" or "absent". "absent" means not configured,
	// which is a legitimate mode here rather than a fault — see healthcheckHandler.
	State string `json:"state"`
	Err   string `json:"error,omitempty"`
}

// healthcheckHandler reports liveness plus the state of each dependency.
//
// The readiness rule is asymmetric on purpose, and it is the whole point of this
// handler (PRD 008 §6):
//
//   - **Database down → not ready.** Once PRD 006's directory lands, login reads
//     from a projection, so no database means no logins. Reporting ready would tell
//     an orchestrator to keep sending traffic to a process that cannot serve it.
//   - **Broker down → still ready.** Reads come from SQL projections, so the app
//     keeps working; only writes and fresh projections stop. During an event,
//     degraded and serving beats correct and dead. But it must still be *visible*,
//     because the silent version of this failure is stale data nobody notices.
//
// A non-zero dead-letter count is reported for the same reason (task 060): it means
// a projection is quietly incomplete, which is invisible from the outside otherwise.
//
// OpenAPI annotations use swaggo/swag (registered as a `go tool`); regenerate
// the spec with:
//
//	go tool swag init -g cmd/api/main.go -o cmd/api/docs
//
// @Summary      Health check
// @Description  Reports API liveness, the running environment, and the state of the database, event broker and projections. Returns 503 when a dependency the API cannot serve without is unavailable; a missing broker is reported but does not fail readiness.
// @Tags         system
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "Ready"
// @Failure      503  {object}  map[string]interface{}  "Not ready (database unavailable)"
// @Router       /healthcheck [get]
func (app *application) healthcheckHandler(w http.ResponseWriter, r *http.Request) {
	db := app.databaseStatus(r.Context())
	broker := app.brokerStatus()

	ready := db.State != "down"
	status := "available"
	code := http.StatusOK
	if !ready {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}

	dependencies := map[string]any{
		"database": db,
		"broker":   broker,
	}

	// Projection lag is reported as the dead-letter count plus how much is wired up.
	// Real lag (stream sequence vs applied sequence) needs a projection that actually
	// consumes subjects; reporting a fabricated zero would read as "fully caught up",
	// which is the most misleading answer available.
	registered, subjects := app.eventing.projectionStats()
	projections := map[string]any{
		"registered": registered,
		"subjects":   subjects,
		"lag":        "unknown",
	}
	switch {
	case registered == 0:
		projections["reason"] = "no projections registered"
	case subjects == 0:
		// This is the current state and it is deliberate, not broken: the person
		// projection is registered but its Consumes() is still empty (tasks 072-075
		// add the subjects). Saying so beats implying nothing is wired.
		projections["reason"] = "projections registered but consuming no subjects yet"
	default:
		projections["reason"] = "lag measurement not implemented"
	}
	if n, err := app.eventing.deadletterCount(); err != nil {
		projections["dead_letters"] = "unknown"
		projections["dead_letters_error"] = err.Error()
	} else {
		projections["dead_letters"] = n
		if n > 0 {
			// Surfaced, not hidden: a non-zero count means at least one event was
			// dropped from a read model and that projection is now incomplete.
			projections["warning"] = "statements were dead-lettered; a projection is incomplete"
		}
	}
	dependencies["projections"] = projections

	data := map[string]any{
		"status":       status,
		"dependencies": dependencies,
		"system_info": map[string]string{
			"environment": app.config.env,
			"version":     vcs.Version(),
		},
	}
	if err := app.WriteJSON(w, code, data, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// databaseStatus pings the pool with a short timeout. The timeout matters: a
// healthcheck that blocks on an unresponsive database is itself a failure mode, and
// probes are usually the thing that notices first.
func (app *application) databaseStatus(ctx context.Context) dependencyStatus {
	if app.db == nil {
		return dependencyStatus{State: "absent"}
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := app.db.PingContext(ctx); err != nil {
		return dependencyStatus{State: "down", Err: err.Error()}
	}
	return dependencyStatus{State: "up"}
}

func (app *application) brokerStatus() dependencyStatus {
	switch {
	case app.config.jetstreamDSN == "":
		return dependencyStatus{State: "absent"}
	case app.eventing.connected():
		return dependencyStatus{State: "up"}
	default:
		// Configured but not connected: the background connector is still retrying.
		return dependencyStatus{State: "down", Err: "not connected; retrying in the background"}
	}
}

// notFoundAPIHandler returns a JSON 404 for unmatched /api/* routes.
func (app *application) notFoundAPIHandler(w http.ResponseWriter, r *http.Request) {
	app.NotFoundResponse(w, r)
}
