package main

import (
	"net/http"

	"nathejk.dk/nathejk/table/checkpoint"
)

// raceAreasOrNil adapts the projection to the read interface, preserving nil-ness.
//
// Not redundant, and the reason is the classic Go trap: assigning a nil `*checkpoint.Table`
// to a `checkpoint.Queries` interface variable produces an interface that is **not** nil, so
// `app.models.RaceAreas == nil` in the handler would be false and the first call would
// dereference a nil pointer. The same hazard is noted on `eventing.publisherOrNil`.
//
// Returning an untyped nil keeps "no projection" checkable by the one test every handler
// does.
func raceAreasOrNil(t *checkpoint.Table) checkpoint.Queries {
	if t == nil {
		return nil
	}
	return t
}

// raceAreaResponse is the region the client caches map tiles for.
//
// Note what is absent: the checkpoints. The response is a hull, never the post positions it
// was derived from — the event area is deliberately not fully known to participants
// (PRD 002), and the client needs a region to iterate tiles over, not a list of posts. The
// `checkpoint` package's read API is shaped the same way, so this is enforced rather than
// merely observed here.
type raceAreaResponse struct {
	// Polygon is the buffered hull, in order, without a repeated closing vertex.
	Polygon []checkpoint.Point `json:"polygon"`
	// SouthWest/NorthEast bound the polygon, which is what a tile walk actually iterates.
	SouthWest checkpoint.Point `json:"south_west"`
	NorthEast checkpoint.Point `json:"north_east"`
	// AreaKm2 lets the client show a download size and refuse an implausible one.
	AreaKm2 float64 `json:"area_km2"`
	// BufferKm is the margin already included in the polygon, so the client does not add
	// its own on top.
	BufferKm float64 `json:"buffer_km"`
	// PositionedCount/TotalCount describe what the area was derived from — how many
	// checkpoints existed and how many were usable. Enough to tell "428 km² from 9 points"
	// from "428 km² from 2 points" without exposing where any of them are.
	PositionedCount int `json:"positioned_count"`
	TotalCount      int `json:"total_count"`
}

// raceAreaHandler serves the region the offline tile cache is scoped to. Runs behind
// requireAuth.
//
// Authenticated, and deliberately not part of `GET /api/config`: that endpoint's own contract
// says everything it carries is public by definition and handed to any browser that asks,
// which is exactly what the race area must not be. A participant has to sign in to use the
// app at all (PRD 005), so requiring a session costs nothing real.
//
// `404` when there is no area rather than an empty `200`. This is the one place in the API
// where "nothing" is not a benign empty list: the caller's next move is to download a few
// hundred megabytes of tiles, and an empty polygon that a client mistook for "cache
// everything" would try to cache the whole country. A missing resource is harder to
// misread. It is a normal state early in the year, before checkpoints have positions.
//
// @Summary      Race area for offline map caching
// @Description  The convex hull of this year's checkpoints plus a 3 km buffer, as a polygon with its bounding box and area. Scopes the client's offline tile cache. Individual checkpoint positions are never returned. 404 when no area can be derived yet (no checkpoints, none positioned, or an implausible result).
// @Tags         map
// @Produce      json
// @Success      200  {object}  raceAreaResponse
// @Failure      401  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      503  {object}  map[string]string
// @Router       /race-area [get]
func (app *application) raceAreaHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := contextGetSession(r); !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}

	// Nil when the app is running without a database or the projection failed to build.
	// Distinguished from "no area yet" because the two want different responses: this one is
	// a server-side problem the client should retry later, not a state of the event.
	if app.models.RaceAreas == nil {
		app.ServiceUnavailableResponse(w, r, "map data is not available")
		return
	}

	area, ok, err := app.models.RaceAreas.RaceArea(app.config.eventYear)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}
	if !ok {
		app.NotFoundResponse(w, r)
		return
	}

	resp := raceAreaResponse{
		Polygon:         area.Polygon,
		SouthWest:       area.SouthWest,
		NorthEast:       area.NorthEast,
		AreaKm2:         area.AreaKm2,
		BufferKm:        checkpoint.BufferKm,
		PositionedCount: area.PositionedCount,
		TotalCount:      area.TotalCount,
	}
	if err := app.WriteJSON(w, http.StatusOK, resp, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}
