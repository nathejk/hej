package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"nathejk.dk/internal/data"
	"nathejk.dk/internal/scans"
	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/checkpoint"
	"nathejk.dk/nathejk/table/person"
)

// The endpoint's contract in one line: absence means "you may not hold this", `unavailable` means
// "unchanged, ask again", and the two must never be confused — a client that reads a transient
// failure as a permission decision stops asking and nothing ever tells it otherwise.

func syncApp(t *testing.T, opts ...func(*application)) *application {
	t.Helper()
	app := newTestApp(t)
	app.config.eventYear = "2026"
	app.config.syncIntervalSeconds = 60
	app.config.syncDebounceSeconds = 5
	app.models = data.NewModels(users.NewMockDirectory(), scans.NewMockSource(),
		fakeRaceAreas{area: areaFixture(), ok: true},
		&stubPeople{p: person.Person{PersonID: "mock-spejder-1"}, found: true}, nil,
		data.WithMapReads(fakeMapReads{}))
	for _, opt := range opts {
		opt(app)
	}
	return app
}

// getSync signs in as the given mock phone and runs the check.
func getSync(t *testing.T, app *application, phone string, ifNoneMatch string) (*http.Response, syncResponse) {
	t.Helper()
	srv := httptest.NewServer(app.routes())
	t.Cleanup(srv.Close)

	var cookies []*http.Cookie
	if phone != "" {
		cookies = authedCookies(t, app, srv, phone[3:], phone)
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/sync", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/sync: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var out syncResponse
	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatalf("decode: %v (body %q)", err, string(body))
		}
	}
	return resp, out
}

// A spejder with a patrol: every dataset except the directory, which they do not get a pane for.
func TestSync_SpejderHoldsEverythingButContacts(t *testing.T) {
	app := syncApp(t)
	resp, out := getSync(t, app, "+4530000001", "")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	for _, want := range []string{"profile", "scans", "handouts", "checkpoints", "race_area"} {
		if out.Versions[want] == "" {
			t.Errorf("missing version for %q: %v", want, out.Versions)
		}
	}
	// Absent, not empty. The key's absence is what tells the device never to ask for a directory —
	// better than the arrangement it replaces, where the client guessed and ate a 403 per foreground.
	if _, present := out.Versions["contacts"]; present {
		t.Error("a spejder must not receive a contacts version")
	}
	if len(out.Unavailable) != 0 {
		t.Errorf("nothing should be unavailable: %v", out.Unavailable)
	}
}

// Personnel: the directory, but nothing patrol-scoped. Their scan and handout endpoints already return
// empty, so a version for those would be a version for something they can never hold.
func TestSync_PersonnelHasNoPatrolDatasets(t *testing.T) {
	app := syncApp(t)
	_, out := getSync(t, app, "+4530000003", "")

	if out.Versions["contacts"] == "" {
		t.Error("crew must receive a contacts version")
	}
	if out.Versions["profile"] == "" {
		t.Error("everyone has a profile")
	}
	if out.Versions["race_area"] == "" {
		t.Error("everyone in the event shares the race area")
	}
	for _, absent := range []string{"scans", "handouts", "checkpoints"} {
		if _, present := out.Versions[absent]; present {
			t.Errorf("a user with no patrol must not receive a %q version", absent)
		}
	}
}

// The failure isolation that lets this be one request instead of six: one broken projection costs its
// own dataset a cycle and costs the other five nothing.
func TestSync_FailingDerivationIsUnavailableNotFatal(t *testing.T) {
	app := syncApp(t, func(a *application) {
		a.models = data.NewModels(users.NewMockDirectory(), scans.NewMockSource(),
			fakeRaceAreas{err: errRaceAreaRead},
			&stubPeople{found: true}, nil,
			data.WithMapReads(fakeMapReads{}))
	})

	resp, out := getSync(t, app, "+4530000001", "")

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 — one failing dataset must not fail the check", resp.StatusCode)
	}
	if len(out.Unavailable) != 1 || out.Unavailable[0] != "race_area" {
		t.Errorf("unavailable = %v, want [race_area]", out.Unavailable)
	}
	// Crucially it is *not* absent: absence would tell the client it may not hold a race area, and it
	// would stop asking forever.
	if _, present := out.Versions["race_area"]; present {
		t.Error("an unavailable dataset must not also carry a version")
	}
	for _, want := range []string{"profile", "scans", "handouts", "checkpoints"} {
		if out.Versions[want] == "" {
			t.Errorf("the other datasets must still be answered; %q missing", want)
		}
	}
}

// The versions must be the same ones the individual derivations produce, or the client would refetch
// against a version that never matches what the payload endpoints imply.
func TestSync_VersionsMatchTheDerivations(t *testing.T) {
	app := syncApp(t)
	_, out := getSync(t, app, "+4530000001", "")

	viewer, found := app.models.Users.Get("mock-spejder-1")
	if !found {
		t.Fatal("fixture user must resolve")
	}

	wantProfile, _ := app.profileVersionFor(viewer)
	if out.Versions["profile"] != wantProfile {
		t.Errorf("profile version = %q, want %q", out.Versions["profile"], wantProfile)
	}
	wantArea, _ := app.raceAreaVersionFor(viewer)
	if out.Versions["race_area"] != wantArea {
		t.Errorf("race_area version = %q, want %q", out.Versions["race_area"], wantArea)
	}
	wantScans, _ := app.scansVersionFor(viewer)
	if out.Versions["scans"] != wantScans {
		t.Errorf("scans version = %q, want %q", out.Versions["scans"], wantScans)
	}
}

func TestSync_RequiresAuth(t *testing.T) {
	app := syncApp(t)
	resp, _ := getSync(t, app, "", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestSync_ServesTheIntervalAndDebounce(t *testing.T) {
	app := syncApp(t, func(a *application) {
		a.config.syncIntervalSeconds = 300
		a.config.syncDebounceSeconds = 7
	})
	_, out := getSync(t, app, "+4530000001", "")

	if out.IntervalSeconds != 300 {
		t.Errorf("interval_seconds = %d, want 300", out.IntervalSeconds)
	}
	if out.DebounceSeconds != 7 {
		t.Errorf("debounce_seconds = %d, want 7", out.DebounceSeconds)
	}
}

// Zero is the operator's kill switch for the interval. It must still be *served* as zero rather than
// silently replaced by a default — the client is what turns it into "no timer, but still check on
// foreground", and it cannot do that if the server helpfully corrects the value.
func TestSync_ZeroIntervalIsServedAsZero(t *testing.T) {
	app := syncApp(t, func(a *application) { a.config.syncIntervalSeconds = 0 })
	_, out := getSync(t, app, "+4530000001", "")

	if out.IntervalSeconds != 0 {
		t.Errorf("interval_seconds = %d, want 0 preserved", out.IntervalSeconds)
	}
	// And it is not a disabled *check*: the versions are still there to be compared.
	if out.Versions["profile"] == "" {
		t.Error("a disabled interval must not disable the check itself")
	}
}

func TestSync_ETagAllowsA304(t *testing.T) {
	app := syncApp(t)
	resp, _ := getSync(t, app, "+4530000001", "")
	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Fatal("ETag must be set, for the browser's own conditional requests")
	}

	again, _ := getSync(t, app, "+4530000001", etag)
	if again.StatusCode != http.StatusNotModified {
		t.Errorf("status = %d, want 304 for a matching ETag", again.StatusCode)
	}
}

// Go randomises map iteration order. An ETag derived from it would change on every request, which is
// worse than having none: it defeats the browser cache *and* claims the data changed.
func TestSyncETag_IsStableAcrossMapOrder(t *testing.T) {
	versions := map[string]string{"contacts": "a", "profile": "b", "scans": "c", "handouts": "d", "checkpoints": "e", "race_area": "f"}
	first := syncETag(versions, nil)
	for range 20 {
		if again := syncETag(versions, nil); again != first {
			t.Fatalf("etag must not depend on map order: %q vs %q", first, again)
		}
	}

	changed := map[string]string{"contacts": "a", "profile": "b", "scans": "CHANGED", "handouts": "d", "checkpoints": "e", "race_area": "f"}
	if syncETag(changed, nil) == first {
		t.Error("a changed version must change the etag")
	}

	// Recovering from a failure has to be visible to a conditional request, or a 304 would mask it.
	if syncETag(versions, []string{"race_area"}) == first {
		t.Error("the unavailable set must be part of the etag")
	}
}

// The endpoint must stay a composition of cheap derivations. This is the property that makes it
// affordable on every foreground from every device, and the one a future change is most likely to
// break — by reaching for the payload builders because they are right there.
func TestSync_DoesNotBuildPayloads(t *testing.T) {
	stub := &stubPeople{p: person.Person{PersonID: "mock-spejder-1"}, found: true}
	app := syncApp(t, func(a *application) {
		a.models = data.NewModels(users.NewMockDirectory(), scans.NewMockSource(),
			fakeRaceAreas{area: checkpoint.RaceArea{}, ok: true}, stub, nil,
			data.WithMapReads(fakeMapReads{}))
	})

	// A spejder gets no contacts key, so the directory listing must not be touched at all.
	getSync(t, app, "+4530000001", "")
	if len(stub.listedRoles) != 0 {
		t.Errorf("the manifest query must not run for a caller with no contacts key: %v", stub.listedRoles)
	}
}
