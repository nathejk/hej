package main

import (
	"net/http"
	"time"
)

type scanResponse struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Label string `json:"label"`
	// The post this scan happened at, or "" when the personnel rota could not place it.
	//
	// Additive (PRD 016). The client uses it to show a post the patrol has reached as visited, so an
	// unattributed scan leaves that post looking unvisited — an under-statement rather than a wrong one.
	CheckpointID string `json:"checkpoint_id"`
	// Nullable: a manually registered scan has no position, so the client can
	// list it but not plot it.
	Lat       *float64  `json:"lat"`
	Lng       *float64  `json:"lng"`
	ScannedAt time.Time `json:"scanned_at"`
	// On-time verdict, computed server-side (PRD 016 §11.5). Both null together when there is nothing
	// to judge against: a bandit catch, an unattributed scan, a post with no window, or a `relative`
	// window whose anchoring scan has not happened yet. A wrong "for sent" is worse than a missing one,
	// so absence is the honest default.
	//
	// delta_seconds is signed: 0 exactly on time, negative when the scan was early (before the window
	// opened), positive when late (after it closed). The client renders the magnitude as "12 min for
	// sent"; the sign lets it tell early from late.
	OnTime       *bool `json:"on_time"`
	DeltaSeconds *int  `json:"delta_seconds"`
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
// @Description  Returns the signed-in user's patrol's checkpoint scans and bandit catches, newest first. Users without a patrol get an empty list. lat/lng are null when the registration has no position. checkpoint_id names the post the scan happened at, or is empty when the personnel rota could not place it. on_time and delta_seconds carry the server-computed on-time verdict; both are null together when there is no window to judge against (bandit catch, unattributed scan, post with no window, or a relative window whose anchoring scan has not happened). delta_seconds is signed: negative early, positive late.
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
		resp := scanResponse{
			ID:           scan.ID,
			Kind:         string(scan.Kind),
			Label:        scan.Label,
			CheckpointID: scan.CheckpointID,
			Lat:          scan.Lat,
			Lng:          scan.Lng,
			ScannedAt:    scan.ScannedAt.UTC(),
		}
		// A missing verdict leaves both fields null — the client draws no badge. Copying into locals
		// rather than pointing into the loop variable's Verdict, which would alias across iterations.
		if v := scan.Verdict; v != nil {
			onTime := v.OnTime
			delta := v.DeltaSeconds
			resp.OnTime = &onTime
			resp.DeltaSeconds = &delta
		}
		out = append(out, resp)
	}

	if err := app.WriteJSON(w, http.StatusOK, scansResponse{Scans: out}, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}
