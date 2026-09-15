package main

import (
	"net/http"

	"nathejk.dk/internal/data"
	"nathejk.dk/internal/reveal"
)

// mapReadsOrNil adapts the reveal rule to the read interface, preserving nil-ness.
//
// Same classic Go trap as `raceAreasOrNil` and `eventing.publisherOrNil`: assigning a nil `*reveal.Rule`
// to a `data.MapReads` interface variable produces an interface that is **not** nil, so
// `app.models.Maps == nil` in the handler would be false and the first call would dereference a nil
// pointer. Returning an untyped nil keeps "no map data" checkable by the one test every handler here does.
func mapReadsOrNil(r *reveal.Rule) data.MapReads {
	if r == nil {
		return nil
	}
	return r
}

// checkpointResponse is one checkpoint the caller's patrol has earned sight of.
//
// # What is absent
//
// Everything about checkpoints the patrol has *not* earned. That is not enforced here — it is enforced by
// the read this handler is allowed to make, which takes the patrol id and cannot be asked a broader
// question (see data.MapReads and PRD 016 §6). A handler that could ask for all checkpoints would be one
// bug away from serving them, so it cannot.
//
// The window travels as three fields rather than a computed "open now": the client renders it, and a
// server-side boolean would be stale by the time it arrived.
type checkpointResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	// Checkgroup and SortOrder give route order together with the group's order. The client needs both
	// halves to point an arrow at the *next* post rather than the nearest one.
	Checkgroup string `json:"checkgroup"`
	SortOrder  int    `json:"sort_order"`

	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`

	// The open window as stored. Zero means "not set", which the client renders as no time rather than as
	// 1970 — a real window on this event is never near the epoch.
	OpenFrom     int64 `json:"open_from"`
	OpenUntil    int64 `json:"open_until"`
	OpenDuration int   `json:"open_duration_minutes"`
}

type checkpointsResponse struct {
	Checkpoints []checkpointResponse `json:"checkpoints"`
}

// listCheckpointsHandler returns the checkpoints the signed-in user's patrol may see. Runs behind
// requireAuth.
//
// # 200 with an empty list, never 404
//
// A patrol that has not been handed a sheet yet, and every personnel user without a patrol, gets an empty
// list. This is deliberately unlike `/api/race-area`, which answers 404 when it has nothing: there, the
// caller's next move is to download a few hundred megabytes of tiles, so an empty polygon mistaken for
// "cache everything" would try to cache the country. Here "nothing revealed" is a benign, normal state for
// the first hour of the race, and the client simply draws no markers.
//
// # Why the reveal rule is not consulted for un-positioned checkpoints
//
// It filters them out already. A revealed checkpoint with no position has nothing to draw and nothing to
// point an arrow at, so it is absent rather than returned with nulls — which is the honest rendering of
// "the organizers have not sited it yet", and one fewer nullable case for the client.
//
// @Summary      Revealed checkpoints
// @Description  The checkpoints the signed-in user's patrol has earned sight of: those drawn on the map sheets it has been handed, plus those in checkgroups it has already reached. Positions of checkpoints the patrol has not been shown are never returned — the read is patrol-scoped in the BFF, not filtered in the client. Users without a patrol get an empty list. Empty is a normal state, not an error.
// @Tags         map
// @Produce      json
// @Success      200  {object}  checkpointsResponse
// @Failure      401  {object}  map[string]string
// @Failure      503  {object}  map[string]string
// @Router       /checkpoints [get]
func (app *application) listCheckpointsHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}

	// Nil when the app is running without a database or the projections failed to build. Distinguished
	// from "nothing revealed" because the two want different answers: this one is a server-side problem
	// the client should retry, not a state of the event — and the client caches the empty answer offline,
	// so reporting it as empty would persist the outage past its end.
	if app.models.Maps == nil {
		app.ServiceUnavailableResponse(w, r, "map data is not available")
		return
	}

	// An unknown user id resolves to the zero User, whose empty PatrolID reveals nothing — the same benign
	// empty response as a personnel user.
	user, _ := app.models.Users.Get(s.UserID)

	revealed, err := app.models.Maps.Revealed(app.config.eventYear, user.PatrolID)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}

	// Non-nil slice so the JSON is [] rather than null; the client iterates unconditionally.
	out := make([]checkpointResponse, 0, len(revealed))
	for _, c := range revealed {
		out = append(out, checkpointResponse{
			ID:           string(c.ID),
			Name:         c.Name,
			Checkgroup:   string(c.Checkgroup),
			SortOrder:    c.SortOrder,
			Lat:          c.Lat,
			Lng:          c.Lng,
			OpenFrom:     c.OpenFromUts,
			OpenUntil:    c.OpenUntilUts,
			OpenDuration: c.OpenDuration,
		})
	}

	if err := app.WriteJSON(w, http.StatusOK, checkpointsResponse{Checkpoints: out}, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}
