package main

import (
	"net/http"
	"time"
)

type scanResponse struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Label string `json:"label"`
	// Nullable: a manually registered scan has no position, so the client can
	// list it but not plot it.
	Lat       *float64  `json:"lat"`
	Lng       *float64  `json:"lng"`
	ScannedAt time.Time `json:"scanned_at"`
}

type scansResponse struct {
	Scans []scanResponse `json:"scans"`
}

// listPatrolScansHandler returns the signed-in user's patrol's event
// registrations (checkpoint scans + bandit catches), newest first. Runs behind
// requireAuth.
//
// A user without a patrol (the personnel roles) gets 200 with an empty list
// rather than 404: having no patrol is a normal state, and the client hides the
// registrations UI on an empty list.
//
// @Summary      Patrol registrations
// @Description  Returns the signed-in user's patrol's checkpoint scans and bandit catches, newest first. Users without a patrol get an empty list. lat/lng are null when the registration has no position.
// @Tags         patrol
// @Produce      json
// @Success      200  {object}  scansResponse
// @Failure      401  {object}  map[string]string
// @Router       /patrol/scans [get]
func (app *application) listPatrolScansHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}

	// An unknown user id resolves to the zero User, whose empty PatrolID yields
	// no scans — the same benign empty response as a personnel user.
	user, _ := app.models.Users.Get(s.UserID)

	found := app.models.Scans.ByPatrol(user.PatrolID)

	// Non-nil slice so the JSON is [] rather than null; the client iterates
	// unconditionally.
	out := make([]scanResponse, 0, len(found))
	for _, scan := range found {
		out = append(out, scanResponse{
			ID:        scan.ID,
			Kind:      string(scan.Kind),
			Label:     scan.Label,
			Lat:       scan.Lat,
			Lng:       scan.Lng,
			ScannedAt: scan.ScannedAt.UTC(),
		})
	}

	if err := app.WriteJSON(w, http.StatusOK, scansResponse{Scans: out}, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}
