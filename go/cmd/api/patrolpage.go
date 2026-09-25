package main

import (
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"

	"nathejk.dk/internal/distance"
	"nathejk.dk/internal/eventtime"
	"nathejk.dk/internal/patroltrack"
	"nathejk.dk/internal/scans"
	"nathejk.dk/nathejk/table/publicpatrol"
)

// Patruljens egen side (PRD 011 §6, §7; task 341).
//
// # This is where the gate is finally consulted
//
// Task 332 registered this route answering the not-yet page for everybody, and deliberately did *not* call
// the gate — a call that looked like a check and then rendered the same page whatever it said would have
// been worse than no call, because the next reader would have believed the route was gated. The gate and
// the open branch arrive together, here, with the tests that prove the closed path stays closed.
//
// # The page is complete without JavaScript, and the map is not part of it
//
// Header, distance, diploma slot and the scan list are all server-rendered. The map is task 342's island
// and is a *progressive enhancement*: where it cannot run, its place is taken by the scan list — which is
// on the page regardless, because a patrol whose scans have no coordinates at all must still get a usable
// page (PRD 011 §6).
//
// # What the page may not claim
//
// Two honesty requirements, both from §0a's measurements:
//
//   - The track records **where a phone had the app open**, not where the patrol walked. Task 082 measured
//     2% coverage. So the page says so, in a sentence a parent will read — without it, a gap reads as "they
//     stood still here".
//   - The distance is a **floor built from the posts**, not a measured route (task 339). The wording carries
//     no route and no precision.

// publicPatrolPageData is the patrol page.
type publicPatrolPageData struct {
	publicPageData

	Patrol publicpatrol.Patrol

	// KorpsLabel is the korps as a human reads it, or "" to omit the segment. Resolved here rather than in
	// the template so `andet` and an unknown slug collapse to the same silence.
	KorpsLabel string

	// FinishedAt is when the patrol crossed the line, or nil for a page the backstop opened.
	FinishedAt *time.Time

	// FinishedLabel is FinishedAt rendered, or "".
	//
	// Precomputed rather than formatted in the template, because `html/template` cannot call a
	// `time.Time` formatter with a `*time.Time` — and the alternative (a formatter taking `any` and
	// switching) moves nil-handling into a template function, which is the one place a nil dereference
	// surfaces as a half-rendered page rather than an error.
	FinishedLabel string

	// HasDiploma reports whether the diploma slot is rendered at all.
	//
	// **Absent, not empty**, for a patrol with no finish (PRD 011 §6, task 346): an empty frame where a
	// diploma should be is a page pointing at what is missing.
	HasDiploma bool

	// Distance is the estimate's label, or "" when there is no figure worth showing.
	Distance string
	// DistanceIncomplete reports that too many scans had no position for the figure to be whole.
	DistanceIncomplete bool

	// Scans are the patrol's registrations in race order.
	Scans []publicScanRow

	// UnplottableScans is how many registrations carry no position, so the page can explain why the map
	// shows fewer pins than the list shows rows.
	UnplottableScans int

	// Track is the merged route, for the map island to fetch. Present here only so the page can state what
	// it covers; the geometry itself goes over the JSON endpoint (task 342).
	TrackSegments int
	TrackPoints   int
	TrackAbsent   bool

	// Reported acknowledges a report that was just filed (task 343).
	//
	// Set from a query parameter after the form's redirect, not from any stored state: the page must not
	// know whether *somebody else* reported it. "Others have complained about your patrol's page" is not
	// a thing a public page should tell a visitor, and a count would invite exactly that rendering.
	//
	// Moved to `publicPageData` in task 423, when the form moved into the shared footer. Kept named here so
	// `data.Reported` reads the same at the call site.
}

// publicScanRow is one registration as the page lists it.
//
// **Race order, oldest first.** The app's list is newest-first, because a participant during the race wants
// the last thing that happened; a page read afterwards is a story, and a story is read forwards. Stated
// because PRD 011 §6 asks for the choice to be made explicitly rather than inherited.
type publicScanRow struct {
	// Label is the post's name, or a plain description when the rota could not place the scan.
	Label string
	// Kind is "checkpoint" or "bandit".
	Kind string
	// At is when it happened.
	At time.Time
	// Plottable reports whether this registration has a position, so the list can mark the ones the map
	// cannot show.
	Plottable bool
}

// publicPatrolPageHandler serves a patrol's own page.
//
// @Summary      A patrol's public page (HTML)
// @Description  The patrol's own page: name, gruppe and korps, an estimated distance built from the posts it was scanned at, its diploma where it finished, and the registrations it collected in race order. Appears only once that patrol has finished, or once the last checkpoint has closed. Before that it answers a "not yet" page that is **identical to the answer for a patrol number that does not exist** — deliberately, so the URL space cannot be used to discover which numbers are real or to watch the field finish in real time. The map is a progressive enhancement served separately; this page is complete without it. Unauthenticated; ignores the session cookie entirely. Not indexed.
// @Tags         public-site
// @Produce      html
// @Param        number  path      string  true  "patrol number"
// @Success      200  {string}  string  "the page, or the not-yet page"
// @Failure      429  {object}  map[string]string  "read rate limit, by IP"
// @Router       /{year}/patrulje/{number} [get]
func (app *application) publicPatrolPageHandler(w http.ResponseWriter, r *http.Request) {
	if !app.allowPublicSiteRead(w, r) {
		return
	}

	number, ok := normalizePatrolNumber(httprouter.ParamsFromContext(r.Context()).ByName("number"))
	if !ok {
		// A number that could not be a number answers not-yet like everything else. A distinguishable
		// "bad request" here would tell a prober that the route parses its input, which is one bit more
		// than nothing.
		app.renderPatrolNotYet(w)
		return
	}

	data, ok := app.patrolPage(number)
	if !ok {
		app.renderPatrolNotYet(w)
		return
	}
	// The acknowledgement after the takedown form's redirect (task 343). A query parameter, so a reload
	// keeps the message and nothing is stored about who reported what.
	data.Reported = r.URL.Query().Get("anmeldt") == "1"
	// Where that form posts (task 423). The footer renders the form now, and it renders one only for a page
	// that names where it goes — so this line is what makes the patrol page the only page with one.
	data.ReportPath = app.publicRoot() + "/patrulje/" + number + "/anmeld"
	app.renderPublicPage(w, "patrol", data)
}

// patrolPage assembles everything the page shows, or reports that it must not be served.
//
// # One place decides whether to serve
//
// The decision is `openPatrol` (patrolmap.go), shared with the map endpoint. Every refusal — unknown
// number, closed gate, missing projection, failed read — comes back as `false` and the caller renders the
// same not-yet page. That is the property PRD 011 §6 requires: the differences between those cases are
// exactly what would leak which numbers are real and which patrols have finished.
func (app *application) patrolPage(number string) (publicPatrolPageData, bool) {
	patrol, verdict, open := app.openPatrol(number)
	if !open {
		return publicPatrolPageData{}, false
	}

	data := publicPatrolPageData{
		publicPageData: publicPageData{
			Year:  app.config.eventYear,
			Title: patrolPageTitle(patrol),
			Root:  app.publicRoot(),
		},
		Patrol:     patrol,
		KorpsLabel: patrol.KorpsLabel(),
		FinishedAt: verdict.FinishedAt,
		// The diploma follows the **page**, not the finish (task 360). Both wordings exist — *har gennemført* and
		// *deltog i* — and `diplom` has printed the second since 2024, so a patrol that walked the night without
		// reaching the finish still gets a certificate that says what it did.
		//
		// This reverses task 346, which made the slot conditional on finishing so that a diploma could never say
		// "gennemført" to a patrol that did not. That risk is real and is handled where it belongs — in the wording,
		// which the renderer picks from FinishedAt — rather than by withholding the document.
		HasDiploma: true,
	}
	if verdict.FinishedAt != nil {
		data.FinishedLabel = eventtime.Danish(*verdict.FinishedAt)
	}

	registrations := app.patrolRegistrations(patrol.TeamID)
	data.Scans = registrations.rows
	data.UnplottableScans = registrations.unplottable

	app.addPatrolDistance(&data, patrol.TeamID, registrations.forDistance)
	app.addPatrolTrackSummary(&data, patrol.TeamID)

	return data, true
}

// patrolRegistrations reads the patrol's scans once, for both the list and the distance.
//
// Once, because two reads would be two chances for the list and the figure to disagree — a page showing
// nine posts and a distance built from eight is a page nobody can explain.
type patrolRegistrations struct {
	rows        []publicScanRow
	unplottable int
	forDistance []distance.Scan
	// pins are the plottable registrations as the map draws them.
	//
	// Built here rather than by the map handler reading the projection again: one read means the list, the
	// pins and the dotted legs cannot disagree about which registrations a patrol has — which they would,
	// visibly, on the same screen, if a scan arrived between two queries.
	pins []patrolMapScan
}

func (app *application) patrolRegistrations(teamID string) patrolRegistrations {
	out := patrolRegistrations{rows: []publicScanRow{}}
	if app.models.Scans == nil || teamID == "" {
		return out
	}

	// Newest-first from the source; the page reads forwards. Reversed rather than re-sorted, because the
	// source's order is already total and re-sorting would be a second opinion about it.
	registrations := app.models.Scans.ByPatrol(teamID)
	for i := len(registrations) - 1; i >= 0; i-- {
		s := registrations[i]
		plottable := s.Lat != nil && s.Lng != nil
		if !plottable {
			out.unplottable++
		}
		out.rows = append(out.rows, publicScanRow{
			Label:     scanRowLabel(s),
			Kind:      string(s.Kind),
			At:        s.ScannedAt,
			Plottable: plottable,
		})
		out.forDistance = append(out.forDistance, distance.Scan{
			Lat: s.Lat, Lng: s.Lng, At: s.ScannedAt,
		})
		if plottable {
			out.pins = append(out.pins, patrolMapScan{
				Lat:   *s.Lat,
				Lng:   *s.Lng,
				Label: scanRowLabel(s),
				Kind:  string(s.Kind),
			})
		}
	}
	return out
}

// scanRowLabel is what the list calls a registration.
//
// The source's label when it has one. When it does not — an unattributed scan, which is a normal outcome
// because the checkpoint is recovered from the scanner's rota — the row says *what kind of thing happened*
// rather than inventing a post name. "En post" is honest; "Post ?" reads as a bug.
func scanRowLabel(s scans.Scan) string {
	if s.Label != "" {
		return s.Label
	}
	if s.Kind == scans.KindBandit {
		return "Fanget af en bandit"
	}
	return "En post"
}

// addPatrolDistance computes the estimate, raised by whatever the track covers.
func (app *application) addPatrolDistance(data *publicPatrolPageData, teamID string, scanList []distance.Scan) {
	var points []distance.Point
	if app.patrolTracks != nil {
		// A failure here costs the *raising*, not the figure: the scan-leg floor stands on its own, and a
		// page with a slightly lower floor is better than no distance at all.
		if track, err := app.patrolTracks.Track(teamID); err == nil {
			points = trackPointsForDistance(track)
		} else {
			app.Logger.Error("reading a track for the distance estimate", "team", teamID, "err", err)
		}
	}

	est := distance.Compute(scanList, points)
	data.Distance = distance.Label(est)
	data.DistanceIncomplete = est.Incomplete
}

// trackPointsForDistance flattens the merged segments for the distance calculation.
//
// Every point of every segment, because `distance.Compute` matches them into scan legs by time and does not
// care which segment they came from — the segmentation exists for *drawing*, where a gap must stay a gap.
//
// Task 341 shipped this returning nil, because `patroltrack.Point` had no timestamp and `Compute` matches
// by time; task 342 added the timestamp for exactly this. The distance is now a floor raised by whatever
// the track covers, which is what PRD 011 §6 asks for.
func trackPointsForDistance(track patroltrack.Track) []distance.Point {
	var out []distance.Point
	for _, seg := range track.Segments {
		for _, p := range seg.Points {
			out = append(out, distance.Point{
				Lat: p.Lat,
				Lng: p.Lng,
				At:  time.UnixMilli(p.TS).UTC(),
			})
		}
	}
	return out
}

// trackSegmentsForDistance is the same conversion, kept **segment by segment**.
//
// For `distance.UncoveredLegs`, which joins the ends of the lines the map draws and therefore has to see where
// one line stops and the next begins. Flattening for that was the bug behind the second round of task 354: the
// merge segments per recorder, so a flat time-ordered list interleaves two phones and hides every boundary.
func trackSegmentsForDistance(track patroltrack.Track) [][]distance.Point {
	var out [][]distance.Point
	for _, seg := range track.Segments {
		line := make([]distance.Point, 0, len(seg.Points))
		for _, p := range seg.Points {
			line = append(line, distance.Point{
				Lat: p.Lat,
				Lng: p.Lng,
				At:  time.UnixMilli(p.TS).UTC(),
			})
		}
		out = append(out, line)
	}
	return out
}

// addPatrolTrackSummary records what the track covers, so the page can say so.
func (app *application) addPatrolTrackSummary(data *publicPatrolPageData, teamID string) {
	if app.patrolTracks == nil {
		// Cannot tell, which is different from "nothing recorded" — so the page says neither and simply
		// omits the sentence rather than claiming the patrol recorded nothing.
		return
	}
	track, err := app.patrolTracks.Track(teamID)
	if err != nil {
		app.Logger.Error("reading a patrol track", "team", teamID, "err", err)
		return
	}
	data.TrackSegments = len(track.Segments)
	data.TrackPoints = track.Points
	// **An empty track is the common case**, not a failure: task 082 measured 2% coverage, and plenty of
	// members never grant location. The page says so plainly instead of showing an unexplained empty map.
	data.TrackAbsent = track.IsEmpty()
}

// patrolPageTitle is the browser title and the page's own heading source.
func patrolPageTitle(p publicpatrol.Patrol) string {
	if p.Name != "" {
		return "Patrulje " + p.Number + " · " + p.Name
	}
	return "Patrulje " + p.Number
}
