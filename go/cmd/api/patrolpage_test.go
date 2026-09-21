package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"nathejk.dk/internal/publicgate"
	"nathejk.dk/internal/scans"
	"nathejk.dk/nathejk/table/checkgroup"
	"nathejk.dk/nathejk/table/publicpatrol"
	"nathejk.dk/nathejk/table/scan"
	"nathejk.dk/nathejk/table/trackpoint"
)

// Patruljens egen side (task 341). The gate is finally consulted here, so about half of this file is about
// what a *closed* page must not reveal.

// patrolStore answers ByNumber from a fixed set.
//
// Mutex-guarded because task 347's burst test drives it from hundreds of goroutines, where an unguarded
// slice append is both a race and a lost record.
type patrolStore struct {
	patrols map[string]publicpatrol.Patrol
	err     error

	mu    sync.Mutex
	asked []string
}

func (s *patrolStore) ByNumber(_ string, number string) (publicpatrol.Patrol, bool, error) {
	s.mu.Lock()
	s.asked = append(s.asked, number)
	s.mu.Unlock()
	if s.err != nil {
		return publicpatrol.Patrol{}, false, s.err
	}
	p, ok := s.patrols[number]
	return p, ok, nil
}

// askedFor reads the recorded numbers safely.
func (s *patrolStore) askedFor() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.asked...)
}

// pageScans is a scans.Source over a fixed list.
type pageScans struct{ byPatrol map[string][]scans.Scan }

func (s pageScans) ByPatrol(patrolID string) []scans.Scan { return s.byPatrol[patrolID] }

func coord(v float64) *float64 { return &v }

var raceNight = time.Date(2026, 9, 19, 21, 0, 0, 0, time.UTC)

func nightAt(min int) time.Time { return raceNight.Add(time.Duration(min) * time.Minute) }

// patrolPageApp wires a patrol that finished, with scans and a gate that opens on the last checkgroup.
func patrolPageApp(t *testing.T) (*application, *patrolStore, *httptest.Server) {
	t.Helper()

	app, _, _ := publicApp(t)
	app.config.eventYear = "2026"

	store := &patrolStore{patrols: map[string]publicpatrol.Patrol{
		"42": {TeamID: "team-42", Number: "42", Name: "Ørnene",
			GroupName: "1. Søllerød Gruppe", Korps: "dds"},
		// A patrol that has not finished: its gate stays closed.
		"43": {TeamID: "team-43", Number: "43", Name: "Ulvene", GroupName: "2. Gruppe", Korps: "kfum"},
		// A patrol with no group and an unspecified korps: the header must omit the whole line.
		"44": {TeamID: "team-44", Number: "44", Name: "Bjørnene", Korps: "andet"},
	}}
	app.models.PublicPatrols = store

	app.models.Scans = pageScans{byPatrol: map[string][]scans.Scan{
		"team-42": {
			// Newest first, as the source returns them.
			{ID: "s-3", Kind: scans.KindCheckpoint, Label: "Mål", CheckpointID: "cp-9",
				Lat: coord(55.7500), Lng: coord(12.2000), ScannedAt: nightAt(600)},
			// A bandit catch with no position.
			{ID: "s-2", Kind: scans.KindBandit, ScannedAt: nightAt(300)},
			{ID: "s-1", Kind: scans.KindCheckpoint, Label: "Post 4A", CheckpointID: "cp-1",
				Lat: coord(55.7000), Lng: coord(12.2000), ScannedAt: nightAt(0)},
		},
	}}

	// The gate: team-42 scanned at the last checkgroup, team-43 did not.
	app.publicGate = publicgate.New(
		gateCheckgroups{groups: []checkgroup.Checkgroup{
			{ID: "cg-1", SortOrder: 10}, {ID: "cg-mål", SortOrder: 20},
		}},
		gateScans{byTeam: map[string][]scan.Scan{
			"team-42": {{QrID: "q1", Uts: nightAt(600).Unix(), CheckgroupID: "cg-mål"}},
			"team-43": {{QrID: "q2", Uts: nightAt(120).Unix(), CheckgroupID: "cg-1"}},
		}},
		gateClosing{},
	)

	srv := httptest.NewServer(app.routes())
	t.Cleanup(srv.Close)
	return app, store, srv
}

func TestPatrolPageRendersTheHeader(t *testing.T) {
	_, _, srv := patrolPageApp(t)

	resp, body := getPublic(t, srv.URL+"/2026/patrulje/42", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	page := string(body)

	for _, want := range []string{
		"Patrulje 42", "Ørnene",
		"1. Søllerød Gruppe",
		// The korps as its label, never the slug.
		"Det Danske Spejderkorps",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the header is missing %q\n%s", want, page)
		}
	}
	if strings.Contains(page, ">dds<") || strings.Contains(page, " dds ") {
		t.Error("the korps slug leaked into the page instead of its label")
	}
}

// `andet` and an absent group must omit the line rather than printing "Andet", which tells a visitor
// nothing and looks like a bug.
func TestPatrolPageOmitsAnUnspecifiedKorpsAndGroup(t *testing.T) {
	app, _, srv := patrolPageApp(t)
	// Open 44's gate by finishing it.
	app.publicGate = publicgate.New(
		gateCheckgroups{groups: []checkgroup.Checkgroup{{ID: "cg-mål", SortOrder: 20}}},
		gateScans{byTeam: map[string][]scan.Scan{
			"team-44": {{QrID: "q", Uts: nightAt(600).Unix(), CheckgroupID: "cg-mål"}},
		}},
		gateClosing{},
	)

	_, body := getPublic(t, srv.URL+"/2026/patrulje/44", nil)
	page := string(body)

	if !strings.Contains(page, "Bjørnene") {
		t.Fatalf("want the patrol rendered\n%s", page)
	}
	for _, forbidden := range []string{"Andet", "andet", `class="group"`} {
		if strings.Contains(page, forbidden) {
			t.Errorf("an unspecified korps and no group must omit the line, found %q", forbidden)
		}
	}
}

// **The finish time comes from the gate**, so the page and the diploma cannot disagree about whether the
// patrol finished (PRD 011 §0b.3).
func TestPatrolPageShowsTheFinishTimeAndTheDiplomaSlot(t *testing.T) {
	_, _, srv := patrolPageApp(t)

	_, body := getPublic(t, srv.URL+"/2026/patrulje/42", nil)
	page := string(body)

	if !strings.Contains(page, "I mål") {
		t.Errorf("want the finish time\n%s", page)
	}
	// 21:00 plus ten hours is the *next* morning — which is what a night race does, and worth pinning so a
	// timezone or date-rollover bug shows up here rather than on a patrol's page.
	if !strings.Contains(page, "20. september 2026 kl. 07:00") {
		t.Errorf("want the Danish-formatted finish time, rolled into the next morning\n%s", page)
	}
	if !strings.Contains(page, `class="diploma"`) {
		t.Error("a patrol that finished should have a diploma slot")
	}
}

// **A backstop-opened page has no diploma slot at all** — absent, not empty. An empty frame where a
// diploma should be is a page pointing at what is missing (task 346).
func TestABackstopOpenedPageHasNoDiplomaSlot(t *testing.T) {
	app, _, srv := patrolPageApp(t)
	// The race is over, and 43 never reached the finish.
	app.publicGate = publicgate.New(
		gateCheckgroups{groups: []checkgroup.Checkgroup{
			{ID: "cg-1", SortOrder: 10}, {ID: "cg-mål", SortOrder: 20},
		}},
		gateScans{byTeam: map[string][]scan.Scan{
			"team-43": {{QrID: "q", Uts: nightAt(120).Unix(), CheckgroupID: "cg-1"}},
		}},
		gateClosing{uts: time.Now().Add(-time.Hour).Unix(), ok: true},
	)

	resp, body := getPublic(t, srv.URL+"/2026/patrulje/43", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the backstop should have opened the page, got %d", resp.StatusCode)
	}
	page := string(body)

	if !strings.Contains(page, "Ulvene") {
		t.Fatalf("want the patrol rendered\n%s", page)
	}
	if strings.Contains(page, `class="diploma"`) {
		t.Error("a patrol that did not finish must have no diploma slot")
	}
	if strings.Contains(page, "I mål") {
		t.Error("a patrol that did not finish must not claim a finish time")
	}
	// And nothing on the page may say they gave up.
	for _, forbidden := range []string{"udgået", "opgav", "gennemførte ikke", "retired"} {
		if strings.Contains(strings.ToLower(page), forbidden) {
			t.Errorf("the page must not announce that a patrol did not finish, found %q", forbidden)
		}
	}
}

// **The closed gate.** A patrol that has not finished, while the race runs, must be indistinguishable from
// one that does not exist.
func TestAPatrolThatHasNotFinishedIsIndistinguishableFromAnUnknownOne(t *testing.T) {
	_, _, srv := patrolPageApp(t)

	var first string
	for i, number := range []string{"43", "999999"} {
		resp, body := getPublic(t, srv.URL+"/2026/patrulje/"+number, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: want 200 with the not-yet page, got %d", number, resp.StatusCode)
		}
		if !strings.Contains(string(body), "ikke klar endnu") {
			t.Errorf("%s: want the not-yet page", number)
		}
		if i == 0 {
			first = string(body)
		} else if string(body) != first {
			t.Errorf("%s: the closed answer must be byte-identical to an unknown patrol", number)
		}
	}
}

// And a closed page must leak nothing about the patrol behind it — not its name, not its group.
//
// Note the sharp edge: the substring search covers the *whole* document, stylesheet included, so a hex
// colour containing "43" fails this test (one did — task 343). That is a false positive, and it is still
// worth keeping the check this blunt: the alternative is parsing the body, and a leak that hides in an
// attribute is exactly the kind this test is for.
func TestAClosedPageLeaksNothingAboutARealPatrol(t *testing.T) {
	_, _, srv := patrolPageApp(t)

	_, body := getPublic(t, srv.URL+"/2026/patrulje/43", nil)
	page := string(body)

	for _, forbidden := range []string{"Ulvene", "2. Gruppe", "KFUM", "43", "team-43"} {
		if strings.Contains(page, forbidden) {
			t.Errorf("the closed page leaks %q", forbidden)
		}
	}
}

// Registrations are listed in **race order, oldest first**: a page read afterwards is a story, and a story
// is read forwards. The app's own list is newest-first, so this is a deliberate difference.
func TestPatrolPageListsRegistrationsInRaceOrder(t *testing.T) {
	_, _, srv := patrolPageApp(t)

	_, body := getPublic(t, srv.URL+"/2026/patrulje/42", nil)
	page := string(body)

	first := strings.Index(page, "Post 4A")
	bandit := strings.Index(page, "Fanget af en bandit")
	last := strings.Index(page, "Mål")
	if first < 0 || bandit < 0 || last < 0 {
		t.Fatalf("want all three registrations listed\n%s", page)
	}
	if !(first < bandit && bandit < last) {
		t.Errorf("registrations are out of race order: %d, %d, %d", first, bandit, last)
	}
}

// An unattributed scan has no label, because the checkpoint is recovered from the scanner's rota. The row
// must say what kind of thing happened rather than inventing a post name.
func TestAnUnattributedScanIsStillListed(t *testing.T) {
	app, _, srv := patrolPageApp(t)
	app.models.Scans = pageScans{byPatrol: map[string][]scans.Scan{
		"team-42": {
			{ID: "s-2", Kind: scans.KindCheckpoint, ScannedAt: nightAt(120)},
			{ID: "s-1", Kind: scans.KindBandit, ScannedAt: nightAt(60)},
		},
	}}

	_, body := getPublic(t, srv.URL+"/2026/patrulje/42", nil)
	page := string(body)

	if !strings.Contains(page, "En post") {
		t.Errorf("an unattributed checkpoint scan should read as a post\n%s", page)
	}
	if !strings.Contains(page, "Fanget af en bandit") {
		t.Errorf("a bandit catch should say so\n%s", page)
	}
	for _, forbidden := range []string{"Post ?", "cp-", "ukendt"} {
		if strings.Contains(page, forbidden) {
			t.Errorf("an unattributed scan must not invent a post name, found %q", forbidden)
		}
	}
}

// A registration with no position is listed and marked, because the list is not the map's fallback — it is
// a requirement (PRD 011 §6).
func TestUnplottableRegistrationsAreListedAndExplained(t *testing.T) {
	_, _, srv := patrolPageApp(t)

	_, body := getPublic(t, srv.URL+"/2026/patrulje/42", nil)
	page := string(body)

	if !strings.Contains(page, "ikke på kortet") {
		t.Errorf("want the un-plottable registration marked\n%s", page)
	}
	if !strings.Contains(page, "registrere en patrulje i hånden") {
		t.Errorf("want the explanation of why a registration has no position\n%s", page)
	}
}

func TestPatrolPageShowsTheDistance(t *testing.T) {
	_, _, srv := patrolPageApp(t)

	_, body := getPublic(t, srv.URL+"/2026/patrulje/42", nil)
	page := string(body)

	// Two positioned scans ~5.5 km apart over ten hours: a walk, and a figure.
	if !strings.Contains(page, "mindst ~5 km") {
		t.Errorf("want the distance floor\n%s", page)
	}
	// Never a decimal, and never a comparison.
	for _, forbidden := range []string{"5,5", "5.5", "længst", "rekord"} {
		if strings.Contains(page, forbidden) {
			t.Errorf("the distance must be a plain floor, found %q", forbidden)
		}
	}
}

// **The caption over the map.** Reworded by the maintainer on 2026-09-21 from a paragraph explaining that the
// route only covers the minutes the app was open, to one sentence: *"Her vises alle de registreringer vi har om
// patruljen."*
//
// The honesty PRD 011 §0a requires — that the drawing must not imply we know more than we do — is now carried
// by **the drawing** instead of by prose: solid where there is a recording, dotted between two registrations
// with nothing behind them (task 354). That is a better division of labour on a page a twelve-year-old reads,
// and it is worth knowing that the sentence alone no longer makes the point.
func TestPatrolPageSaysWhatTheMapShows(t *testing.T) {
	app, _, srv := patrolPageApp(t)
	// A track with something in it, so the map section renders.
	app.patrolTracks = trackReader(t,
		&trackPeople{members: map[string][]string{"team-42": {"p1"}}},
		&trackPoints{byPerson: map[string][]trackpoint.Point{"p1": walk(12.200, 10)}},
	)

	_, body := getPublic(t, srv.URL+"/2026/patrulje/42", nil)
	page := string(body)

	if !strings.Contains(page, "Her vises alle de registreringer vi har om patruljen") {
		t.Errorf("the map needs its caption\n%s", page)
	}
	// And it must not claim the route is the whole walk — the thing the old paragraph was there to prevent.
	for _, forbidden := range []string{"hele vejen I gik", "jeres rute", "ruten I gik"} {
		if strings.Contains(page, forbidden) {
			t.Errorf("the caption claims more than the data supports: %q", forbidden)
		}
	}
}

// **An empty track is the common case** (task 082 measured 2% coverage), and the page must say so rather
// than showing an unexplained empty map.
func TestAnAbsentTrackIsExplainedRatherThanShownEmpty(t *testing.T) {
	app, _, srv := patrolPageApp(t)
	app.patrolTracks = trackReader(t,
		&trackPeople{members: map[string][]string{"team-42": {"p1"}}},
		&trackPoints{},
	)

	_, body := getPublic(t, srv.URL+"/2026/patrulje/42", nil)
	page := string(body)

	if !strings.Contains(page, "ingen rute at vise") {
		t.Errorf("want the absent-track explanation\n%s", page)
	}
	if !strings.Contains(page, "helt normalt") {
		t.Errorf("the page must say an absent route is normal, not a fault\n%s", page)
	}
	if strings.Contains(page, `id="patrolmap"`) {
		t.Error("no map container should be rendered when there is no route")
	}
}

// The page is complete without JavaScript: the map is the only enhancement, and everything else is server
// rendered.
//
// A patrol with no track has nothing to enhance, so its page carries no script at all — which is the case
// this test uses, since it is also the common one (task 082 measured 2% coverage).
func TestPatrolPageNeedsNoScript(t *testing.T) {
	_, _, srv := patrolPageApp(t)

	_, body := getPublic(t, srv.URL+"/2026/patrulje/42", nil)
	page := strings.ToLower(string(body))

	for _, forbidden := range []string{"<script", "onclick=", "onload="} {
		if strings.Contains(page, forbidden) {
			t.Errorf("a patrol page with no route must carry no script, found %q", forbidden)
		}
	}
	// And the substance must be there without it.
	for _, want := range []string{"<h1>", "<ol class=\"scans\">", "undervejs"} {
		if !strings.Contains(page, want) {
			t.Errorf("the page is missing %q without script", want)
		}
	}
}

// **The script on a patrol page may only ever be the map island.** Three files, same-origin, deferred — and
// nothing inline, because an inline handler is how an enhancement becomes a requirement.
func TestPatrolPageScriptIsOnlyTheMapIsland(t *testing.T) {
	app, _, srv := patrolPageApp(t)
	app.patrolTracks = trackReader(t,
		&trackPeople{members: map[string][]string{"team-42": {"p1"}}},
		&trackPoints{byPerson: map[string][]trackpoint.Point{"p1": walk(12.200, 10)}},
	)

	_, body := getPublic(t, srv.URL+"/2026/patrulje/42", nil)
	page := string(body)

	for _, want := range []string{
		`<script src="/vendor/leaflet.js" defer></script>`,
		`<script src="/vendor/leaflet.markercluster.js" defer></script>`,
		`<script src="/publicmap.js" defer></script>`,
		`<link rel="stylesheet" href="/vendor/leaflet.css">`,
		`<link rel="stylesheet" href="/vendor/MarkerCluster.css">`,
		`<link rel="stylesheet" href="/vendor/MarkerCluster.Default.css">`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the map island is missing %s", want)
		}
	}

	// Exactly three scripts, all with a src: no inline script, ever.
	if got := strings.Count(page, "<script"); got != 3 {
		t.Errorf("want exactly 3 script tags, got %d", got)
	}
	if strings.Count(page, "<script src=") != 3 {
		t.Error("every script must be an external, deferred file — an inline script cannot be deferred and " +
			"is how an enhancement becomes a requirement")
	}
	for _, forbidden := range []string{"onclick=", "onload=", "javascript:", "//unpkg", "//cdn"} {
		if strings.Contains(page, forbidden) {
			t.Errorf("the page contains %q; the island is same-origin and event-handler-free", forbidden)
		}
	}

	// The container must carry the number the island needs, and nothing else.
	if !strings.Contains(page, `data-patrol="42"`) {
		t.Errorf("the map container must name the patrol for the island to fetch\n%s", page)
	}
}

// And the substance survives the enhancement: a page *with* a map still carries the scan list and the
// caveat, so a visitor who never runs the script loses nothing but the picture.
func TestTheMapDoesNotReplaceTheScanList(t *testing.T) {
	app, _, srv := patrolPageApp(t)
	app.patrolTracks = trackReader(t,
		&trackPeople{members: map[string][]string{"team-42": {"p1"}}},
		&trackPoints{byPerson: map[string][]trackpoint.Point{"p1": walk(12.200, 10)}},
	)

	_, body := getPublic(t, srv.URL+"/2026/patrulje/42", nil)
	page := string(body)

	for _, want := range []string{
		`<ol class="scans">`,
		"Post 4A",
		"Her vises alle de registreringer vi har om patruljen",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page with a map is missing %q; the list is not the map's fallback, it is a "+
				"requirement", want)
		}
	}
}

// The gate must be asked about the **team id**, never the number a visitor typed.
func TestTheGateIsAskedAboutTheTeamIDNotTheNumber(t *testing.T) {
	app, _, srv := patrolPageApp(t)

	asked := map[string]bool{}
	app.publicGate = publicgate.New(
		gateCheckgroups{groups: []checkgroup.Checkgroup{{ID: "cg-mål", SortOrder: 20}}},
		recordingScans{asked: asked, byTeam: map[string][]scan.Scan{
			"team-42": {{QrID: "q", Uts: nightAt(600).Unix(), CheckgroupID: "cg-mål"}},
		}},
		gateClosing{},
	)

	getPublic(t, srv.URL+"/2026/patrulje/42", nil)

	if !asked["team-42"] {
		t.Errorf("the gate should have been asked about the team id, was asked about %v", asked)
	}
	if asked["42"] {
		t.Error("the gate was asked about the public number instead of the team id")
	}
}

// A failing patrol read must answer not-yet rather than an error page: an error is distinguishable, and a
// distinguishable answer is a probe.
func TestAFailingPatrolReadAnswersNotYet(t *testing.T) {
	_, store, srv := patrolPageApp(t)
	store.err = errPatrolReadFailed

	resp, body := getPublic(t, srv.URL+"/2026/patrulje/42", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 with the not-yet page, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "ikke klar endnu") {
		t.Error("want the not-yet page")
	}
}

// With no patrol projection at all, every page is closed — failing closed rather than answering 503, since
// an "exists but unavailable" confirms a number is real.
func TestNoPatrolProjectionClosesEveryPage(t *testing.T) {
	app, _, srv := patrolPageApp(t)
	app.models.PublicPatrols = nil

	resp, body := getPublic(t, srv.URL+"/2026/patrulje/42", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 with the not-yet page, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "ikke klar endnu") {
		t.Error("want the not-yet page")
	}
}

// Leading zeros and stray whitespace must reach the same page, so a number read off a sign works however
// it is typed.
func TestPatrolPageNormalisesTheNumber(t *testing.T) {
	_, store, srv := patrolPageApp(t)

	resp, _ := getPublic(t, srv.URL+"/2026/patrulje/042", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	for _, asked := range store.askedFor() {
		if asked != "42" {
			t.Errorf("the projection was asked for %q; the number should be normalised to 42", asked)
		}
	}
}

// The session must be ignored here as everywhere else on this surface.
func TestPatrolPageIgnoresTheSession(t *testing.T) {
	app, _, srv := patrolPageApp(t)
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	for _, number := range []string{"42", "43"} {
		_, anonymous := getPublic(t, srv.URL+"/2026/patrulje/"+number, nil)
		_, signedIn := getPublic(t, srv.URL+"/2026/patrulje/"+number, cookies)
		if string(anonymous) != string(signedIn) {
			t.Errorf("patrol %s differs for a signed-in member", number)
		}
	}
}

// recordingScans records which teams the gate asked about.
type recordingScans struct {
	byTeam map[string][]scan.Scan
	asked  map[string]bool
}

func (s recordingScans) ByTeam(_, teamID string) ([]scan.Scan, error) {
	s.asked[teamID] = true
	return s.byTeam[teamID], nil
}

// errPatrolReadFailed stands in for a database problem in the patrol read.
var errPatrolReadFailed = errors.New("database is down")

// The page for a patrol that did not finish (PRD 011 §11 Q4, task 346).
//
// Plenty of Nathejk patrols do not finish, and they still walked most of a night. The gate's backstop gives
// them a page when the last checkpoint closes, and the whole design question is what that page *says*: the
// answer is that it says nothing about it. These tests hold that line, because it is the kind of thing a
// well-meaning copy change breaks.

// backstopOpenedApp opens every page via the backstop: the race is over and nobody reached the finish.
func backstopOpenedApp(t *testing.T) (*application, *httptest.Server) {
	t.Helper()

	app, _, srv := patrolPageApp(t)
	// A track, because "complete" below includes the map: a patrol that did not finish still recorded a
	// route, and the page must show it.
	app.patrolTracks = trackReader(t,
		&trackPeople{members: map[string][]string{"team-42": {"p1"}}},
		&trackPoints{byPerson: map[string][]trackpoint.Point{"p1": walk(12.200, 10)}},
	)
	app.publicGate = publicgate.New(
		gateCheckgroups{groups: []checkgroup.Checkgroup{
			{ID: "cg-1", SortOrder: 10}, {ID: "cg-mål", SortOrder: 20},
		}},
		// 43 was scanned at an intermediate post only. 42's own scans are in the fixture but its gate
		// verdict now comes from the backstop, not from a finish.
		gateScans{byTeam: map[string][]scan.Scan{
			"team-43": {{QrID: "q", Uts: nightAt(120).Unix(), CheckgroupID: "cg-1"}},
		}},
		gateClosing{uts: time.Now().Add(-time.Hour).Unix(), ok: true},
	)
	return app, srv
}

// **A backstop-opened page is a first-class page, not a degraded one.** Everything the page is for — the
// header, the distance, the registrations, the map — is there; only the diploma is not.
func TestABackstopOpenedPageIsComplete(t *testing.T) {
	_, srv := backstopOpenedApp(t)

	resp, body := getPublic(t, srv.URL+"/2026/patrulje/42", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	page := string(body)

	for _, want := range []string{
		"Ørnene",             // the header
		"1. Søllerød Gruppe", // gruppe and korps
		"mindst",             // the distance, as a floor
		"Post 4A",            // the registrations
		`id="patrolmap"`,     // the map container
		"Undervejs",          // the section that carries the list
	} {
		if !strings.Contains(page, want) {
			t.Errorf("a backstop-opened page is missing %q — it must be a whole page, not a stub", want)
		}
	}
	// And the one thing it does not have.
	if strings.Contains(page, `class="diploma"`) {
		t.Error("a patrol that did not finish must have no diploma slot")
	}
}

// **Nothing on the page draws attention to what is missing.** A patrol that walked seven hours and got
// driven home does not need a page explaining that to their family: the absence of a diploma is information
// enough for anyone looking for it, and silence is kinder than a sentence about retiring.
func TestABackstopOpenedPageSaysNothingAboutNotFinishing(t *testing.T) {
	_, srv := backstopOpenedApp(t)

	_, body := getPublic(t, srv.URL+"/2026/patrulje/42", nil)
	page := strings.ToLower(string(body))

	for _, forbidden := range []string{
		"udgået", "udgik", "opgav", "opgivet", "gennemførte ikke", "nåede ikke",
		"ikke i mål", "intet diplom", "uden diplom", "afbrudt", "retired",
	} {
		if strings.Contains(page, forbidden) {
			t.Errorf("the page says %q; a patrol that did not finish is not announced", forbidden)
		}
	}
	// Nor may it claim a finish it does not have.
	if strings.Contains(page, "i mål") {
		t.Error("a backstop-opened page must not claim a finish time")
	}
}

// **The copy must not tell a family that not finishing means no page.** Both the frontpage hint and the
// not-yet page used to say a patrol's page appears "når patruljen er i mål" — true of the first trigger and
// misleading about the second, which is precisely the reading that excludes the patrols this task is about.
func TestTheCopyNamesBothWaysAPageOpens(t *testing.T) {
	_, _, srv := patrolPageApp(t)

	for _, path := range []string{"/2026", "/2026/patrulje/43"} {
		_, body := getPublic(t, srv.URL+path, nil)
		page := string(body)

		if !strings.Contains(page, "i mål") {
			t.Errorf("%s: the copy should still say a page appears when a patrol finishes", path)
		}
		// The second trigger has to be named too, or the first reads as a condition.
		if !strings.Contains(page, "løbet er slut") {
			t.Errorf("%s: the copy does not say every patrol gets a page when the race ends; a patrol "+
				"that was driven home would read this as \"not for us\"\n%s", path, page)
		}
	}
}
