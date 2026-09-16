package main

import (
	"net/http"
	"time"
)

// handoutResponse is one map sheet the caller's patrol has been handed.
//
// # What is deliberately absent
//
// The successor team. When a sheet is reassigned, HQ's organizer view shows "Flyttet til {team}"; this app
// shows only that the sheet is no longer held (still_held=false) and names nobody. The reveal layer never
// selects those columns (task 251), so there is nothing to strip here — but the response shape is pinned by
// a test so a later edit cannot reintroduce a field that names another team to a participant.
//
// There is no sheet id either: a participant acts on the sticker number (which they can read aloud down a
// phone) and the sheet's name, not on our internal kort id.
type handoutResponse struct {
	// Name is the sheet's name, or "Ukendt kort" when the sheet behind the QR was never recorded.
	//
	// The Danish fallback is decided here, not in the client, for the same reason the scans endpoint labels
	// an unplaceable scan "Registrering" server-side: it is the wording HQ's own patrol page uses, so a
	// patrol and an organizer on the phone read the same words. "" means *unknown sheet*, not *no sheet* —
	// the patrol is holding something — so the row is listed with a name rather than dropped.
	Name string `json:"name"`

	// Format is a4 | a3 | skitse | andet, empty when the sheet is unknown.
	Format string `json:"format"`

	// QrID is the printed sticker number, or "" for a synthesised handout (a skitse has no code at all).
	QrID string `json:"qr_id"`

	// HandedOut is when the patrol got it. Upstream stores unix seconds; converted here to a timestamp so
	// the whole API speaks one time format (see scanned_at on /patrol/scans) rather than leaking the raw
	// integer to the client.
	HandedOut time.Time `json:"handed_out"`

	// StillHeld is false once the sheet has been reassigned to another team. Always true for a synthesised
	// handout: there is no binding to lose.
	StillHeld bool `json:"still_held"`
}

type handoutsResponse struct {
	Handouts []handoutResponse `json:"handouts"`
}

// listPatrolHandoutsHandler returns the map sheets the signed-in user's patrol has been handed, oldest
// first. Runs behind requireAuth.
//
// # 200 with an empty list, never 404
//
// A patrol before its first handout, and every personnel user without a patrol, gets an empty list —
// matching /api/patrol/scans and /api/checkpoints. Empty is a normal state for the first hour of the race,
// and the client simply shows no "Kort udleveret" section.
//
// # Patrol-scoped by construction
//
// Like the checkpoints read, the handout read takes only the patrol id (data.MapReads.Handouts), so this
// route cannot be asked for another team's sheets. The set filter — patrol sheets are those in a set marked
// `teamType == "patrulje"`, across all such sets, never matched on the set's name — lives in the projection
// (kort.PatrolSheets), so a mid-season rename cannot change what a patrol sees.
//
// @Summary      Patrol map sheets
// @Description  The map sheets the signed-in user's patrol has been handed, oldest first: QR-bound sheets from the handout record, plus sheets handed over at a post (synthesised, no sticker number). Each carries the sheet name (or "Ukendt kort" when the sheet behind a QR was never recorded), format, the sticker number when there is one, when it was handed out, and whether the patrol still holds it. A reassigned sheet reports still_held=false and never names the team it moved to — this app does not tell one participant about another. Users without a patrol get an empty list. Empty is a normal state, not an error.
// @Tags         patrol
// @Produce      json
// @Success      200  {object}  handoutsResponse
// @Failure      401  {object}  map[string]string
// @Failure      503  {object}  map[string]string
// @Router       /patrol/handouts [get]
func (app *application) listPatrolHandoutsHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}

	// Nil when the app runs without a database or the projections failed to build. A server-side problem
	// the client should retry, not a state of the event — and the client must not cache it as "empty".
	if app.models.Maps == nil {
		app.ServiceUnavailableResponse(w, r, "map data is not available")
		return
	}

	// An unknown user id resolves to the zero User, whose empty PatrolID yields no handouts — the same
	// benign empty response as a personnel user.
	user, _ := app.models.Users.Get(s.UserID)

	handouts, err := app.models.Maps.Handouts(app.config.eventYear, user.PatrolID)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}

	// Non-nil slice so the JSON is [] rather than null; the client iterates unconditionally.
	out := make([]handoutResponse, 0, len(handouts))
	for _, h := range handouts {
		name := h.Name
		// Empty sheet id is the "unknown sheet" case (task 256/257): the patrol holds a QR whose sheet was
		// never recorded. Name it rather than showing a blank row.
		if h.Sheet == "" {
			name = "Ukendt kort"
		}
		out = append(out, handoutResponse{
			Name:      name,
			Format:    string(h.Format),
			QrID:      h.QrID,
			HandedOut: time.Unix(h.HandedOutUts, 0).UTC(),
			StillHeld: h.StillHeld,
		})
	}

	if err := app.WriteJSON(w, http.StatusOK, handoutsResponse{Handouts: out}, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}
