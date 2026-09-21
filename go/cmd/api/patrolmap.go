package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"

	"nathejk.dk/internal/publicgate"
	"nathejk.dk/nathejk/table/publicpatrol"
)

// The public map's data (PRD 011 §6, §8; task 342).
//
// # Two endpoints, because they answer to different gates
//
// A patrol's route and scans are gated on that patrol having finished (task 330). The album photographs are
// not gated at all — they are published when a curator publishes them. Serving both from one endpoint would
// mean one handler with two access rules, which is how the wrong one gets applied.
//
// # What may never appear here
//
// **No timestamp on a track point.** `patroltrack.Point` carries one server-side, for the distance
// estimate, and it must not cross the wire: precise times on a merged track would let a reader infer that
// two overlapping segments belong to different people, and from there how the patrol split up. `Recorders`
// answers "how many recorded?" as a count, which is the most that question should yield (PRD 011 §0b.1).
//
// **No glimt.** A member tapping "Offentligt" agreed to share a photograph, not a position — their EXIF is
// stripped and nothing replaces it (PRD 011 §6, §11 Q7). Only curated album photographs are plotted, and
// the album endpoint is the only source of markers, so a glimt has no route in.

// patrolMapResponse is the geometry for one patrol's map.
type patrolMapResponse struct {
	// Track is the merged route as a list of segments, each a list of [lat, lng] pairs.
	//
	// **A multi-segment shape, not one list of points**, because that is what keeps a gap a gap: Leaflet
	// draws an array of arrays as separate strokes, so a two-hour silence renders as a break rather than as
	// a confident line through terrain nobody walked (PRD 011 §0a).
	Track [][][2]float64 `json:"track"`

	// Scans are the plottable registrations.
	Scans []patrolMapScan `json:"scans"`

	// Recorders is how many members contributed any point. A count, never a list — see the file header.
	Recorders int `json:"recorders"`
}

// patrolMapScan is one registration as a pin.
type patrolMapScan struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
	// Label is the post's name, or a generic description when the rota could not place it.
	Label string `json:"label"`
	// Kind is "checkpoint" or "bandit", so the two get different markers.
	Kind string `json:"kind"`
}

// patrolMapHandler serves one patrol's route and scan pins.
//
// @Summary      A patrol's map geometry
// @Description  The patrol's merged route and its plottable registrations, for the map on its public page. The route is a **list of segments** rather than one list of points, so a gap in the recording renders as a break rather than as a line through ground nobody walked. Gated exactly as the page is: a patrol that has not finished answers 404, identically to a patrol that does not exist. Carries no person, and **no timestamps** — a time on a merged track would let a reader infer that two overlapping segments belong to different people. Unauthenticated; ignores the session cookie.
// @Tags         public-site
// @Produce      json
// @Param        number  path      string  true  "patrol number"
// @Success      200  {object}  patrolMapResponse
// @Failure      404  {object}  map[string]string  "unknown patrol, or its page is not open yet"
// @Failure      429  {object}  map[string]string  "read rate limit, by IP"
// @Router       /public/patrol/{number}/map [get]
func (app *application) patrolMapHandler(w http.ResponseWriter, r *http.Request) {
	if !app.allowPublicSiteRead(w, r) {
		return
	}

	number, ok := normalizePatrolNumber(httprouter.ParamsFromContext(r.Context()).ByName("number"))
	if !ok {
		app.NotFoundResponse(w, r)
		return
	}

	patrol, _, open := app.openPatrol(number)
	if !open {
		// 404 for every refusal: unknown number, closed gate, missing projection. A JSON caller gets no
		// more than the page's visitor does.
		app.NotFoundResponse(w, r)
		return
	}

	resp := patrolMapResponse{
		Track: [][][2]float64{},
		Scans: []patrolMapScan{},
	}

	if app.patrolTracks != nil {
		track, err := app.patrolTracks.Track(patrol.TeamID)
		if err != nil {
			app.ServerErrorResponse(w, r, err)
			return
		}
		resp.Recorders = track.Recorders
		for _, seg := range track.Segments {
			line := make([][2]float64, 0, len(seg.Points))
			for _, p := range seg.Points {
				// Coordinates only. The timestamp stays server-side — see the file header.
				line = append(line, [2]float64{p.Lat, p.Lng})
			}
			resp.Track = append(resp.Track, line)
		}
	}

	if app.models.Scans != nil {
		for _, s := range app.models.Scans.ByPatrol(patrol.TeamID) {
			if s.Lat == nil || s.Lng == nil {
				// Listed on the page, not plotted here. A post can register a patrol by hand, and the
				// page explains why the list is longer than the map.
				continue
			}
			resp.Scans = append(resp.Scans, patrolMapScan{
				Lat:   *s.Lat,
				Lng:   *s.Lng,
				Label: scanRowLabel(s),
				Kind:  string(s.Kind),
			})
		}
	}

	if err := app.WriteJSON(w, http.StatusOK, resp, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// albumMapResponse is every plottable album photograph.
type albumMapResponse struct {
	Photos []albumMapPhoto `json:"photos"`
}

// albumMapPhoto is one located photograph as a marker.
type albumMapPhoto struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
	// Album and Ordinal address the thumbnail, so the marker can show the picture.
	Album   string `json:"album"`
	Ordinal int    `json:"ordinal"`
	// Slug is the album's URL segment, so a marker can link to the album it came from.
	Slug string `json:"slug"`
}

// albumMapHandler serves the located album photographs.
//
// @Summary      Located album photographs
// @Description  Every curated album photograph that carries a position inside the race area, for plotting on the public maps. Only `inside` bounds verdicts are returned and only from published albums — both filters are in the projection's SQL, so a handler cannot plot an unchecked coordinate by forgetting a condition. **Public glimt are deliberately absent**: a member sharing a photograph did not agree to share a position, and their EXIF is stripped with nothing replacing it. Unauthenticated; ignores the session cookie.
// @Tags         public-site
// @Produce      json
// @Success      200  {object}  albumMapResponse
// @Failure      429  {object}  map[string]string  "read rate limit, by IP"
// @Failure      503  {object}  map[string]string  "albums are unavailable"
// @Router       /public/albums [get]
func (app *application) albumMapHandler(w http.ResponseWriter, r *http.Request) {
	if !app.allowPublicSiteRead(w, r) {
		return
	}
	if app.models.Albums == nil {
		app.ServiceUnavailableResponse(w, r, "billederne er ikke tilgængelige lige nu")
		return
	}

	items, err := app.models.Albums.Plottable(app.config.eventYear)
	if err != nil {
		app.ServerErrorResponse(w, r, err)
		return
	}

	resp := albumMapResponse{Photos: make([]albumMapPhoto, 0, len(items))}
	for _, it := range items {
		resp.Photos = append(resp.Photos, albumMapPhoto{
			Lat:     it.Lat,
			Lng:     it.Lng,
			Album:   it.AlbumID,
			Ordinal: it.Ordinal,
			Slug:    it.AlbumSlug,
		})
	}

	if err := app.WriteJSON(w, http.StatusOK, resp, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// openPatrol resolves a number to a patrol whose page may be served.
//
// Factored out so the page and its map endpoint share **one** access decision. Two decisions for one patrol
// is how the JSON stays open after the page closes — which would be the whole course in machine-readable
// form.
func (app *application) openPatrol(number string) (publicpatrol.Patrol, publicgate.Verdict, bool) {
	// Nil fails closed rather than answering 503, for the reason in publicgate.go: on the open web an
	// "exists but unavailable" confirms a patrol number is real.
	if app.models.PublicPatrols == nil {
		return publicpatrol.Patrol{}, publicgate.Verdict{}, false
	}

	patrol, found, err := app.models.PublicPatrols.ByNumber(app.config.eventYear, number)
	if err != nil {
		app.Logger.Error("reading a public patrol", "number", number, "err", err)
		return publicpatrol.Patrol{}, publicgate.Verdict{}, false
	}
	if !found {
		return publicpatrol.Patrol{}, publicgate.Verdict{}, false
	}

	// The gate is asked about the **team id**, never the number. The number is a public string a visitor
	// typed; the id is what the scans and the tracks are keyed by.
	verdict, open := app.patrolGateFor(patrol.TeamID)
	if !open {
		return publicpatrol.Patrol{}, publicgate.Verdict{}, false
	}
	return patrol, verdict, true
}
