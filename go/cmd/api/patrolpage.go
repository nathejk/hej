package main

import (
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"

	"nathejk.dk/internal/distance"
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
// @Router       /offentligt/patrulje/{number} [get]
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
	app.renderPublicPage(w, "patrol", data)
}

// patrolPage assembles everything the page shows, or reports that it must not be served.
//
// # One place decides whether to serve
//
// Every refusal — unknown number, closed gate, missing projection, failed read — returns `false` and the
// caller renders the same not-yet page. That is the property PRD 011 §6 requires: the differences between
// those cases are exactly what would leak which numbers are real and which patrols have finished.
func (app *application) patrolPage(number string) (publicPatrolPageData, bool) {
	// Nil fails closed rather than answering 503, for the reason in publicgate.go: on the open web an
	// "exists but unavailable" confirms a patrol number is real.
	if app.models.PublicPatrols == nil {
		return publicPatrolPageData{}, false
	}

	patrol, found, err := app.models.PublicPatrols.ByNumber(app.config.eventYear, number)
	if err != nil {
		app.Logger.Error("reading a public patrol", "number", number, "err", err)
		return publicPatrolPageData{}, false
	}
	if !found {
		return publicPatrolPageData{}, false
	}

	// The gate is asked about the **team id**, never the number. The number is a public string a visitor
	// typed; the id is what the scans and the tracks are keyed by.
	verdict, open := app.patrolGateFor(patrol.TeamID)
	if !open {
		return publicPatrolPageData{}, false
	}

	data := publicPatrolPageData{
		publicPageData: publicPageData{
			Year:  app.config.eventYear,
			Title: patrolPageTitle(patrol),
		},
		Patrol:     patrol,
		KorpsLabel: patrol.KorpsLabel(),
		FinishedAt: verdict.FinishedAt,
		// The diploma follows the finish, not the gate: a backstop-opened patrol never crossed the line,
		// so there is nothing to certify (PRD 011 §6, task 346).
		HasDiploma: verdict.Finished(),
	}
	if verdict.FinishedAt != nil {
		data.FinishedLabel = publicDanishDateTime(*verdict.FinishedAt)
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
// # Why this loses the timestamps, and why that is a problem worth naming
//
// `patroltrack.Point` carries only a latitude and a longitude — deliberately, because the map draws a line
// and a field the map does not use is a field that ends up in a payload nobody audited. But
// `distance.Compute` matches track points into scan legs **by time**, so without timestamps it cannot.
//
// So today the track raises nothing, and the distance is the pure scan-leg floor. That is correct but
// weaker than PRD 011 §6 intends, and the fix belongs with the map endpoint (task 342), which will need a
// timestamped shape anyway. Recorded here rather than silently returning nil, so the next reader knows the
// figure is a floor by *omission* rather than by design.
func trackPointsForDistance(patroltrack.Track) []distance.Point { return nil }

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
