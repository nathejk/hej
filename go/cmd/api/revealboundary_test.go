package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"nathejk.dk/internal/data"
	"nathejk.dk/internal/mapfixture"
	"nathejk.dk/internal/scans"
	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/checkpoint"
)

// This file is one test, and it is the test PRD 016 exists to be safe about.
//
// # What it guards
//
// The event area is deliberately not fully known to participants (PRD 002). PRD 016 opens a hole in that
// on purpose — a patrol may see the posts drawn on the sheets it holds — and the risk is that the hole
// widens by accident: a column added to a projection, a struct field added to a response, a join copied
// across from an organizer tool. Each of those is a small, reasonable-looking change, and none of them
// would fail any other test.
//
// # Why it runs the real rule rather than a fake
//
// A fake reveal rule would test the handler's plumbing and nothing about the guarantee. So this wires the
// **actual** `reveal.Rule` over fake projections holding a fixture world, and drives the **actual** HTTP
// handlers. The only thing faked is the data.
//
// # Why it greps the JSON rather than inspecting structs
//
// A field added to a Go struct in six months would still serialise, and a struct assertion written today
// would not know to look for it. Searching the response body for a coordinate that must never leave is
// blunt, and it keeps working against code nobody has thought about yet.

// The fixture world lives in `internal/mapfixture`, not here — it is shared with the dev-simulation
// fallback (task 270), so the states this test guards are the same ones a developer can look at on a
// device. cp-secret, in cg-secret, is revealed by nothing; its coordinates are the strings this test hunts.
const (
	secretLat = mapfixture.SecretLat
	secretLng = mapfixture.SecretLng
)

// revealApp wires the real reveal rule over the shared fixture world.
func revealApp(t *testing.T, year string) *application {
	t.Helper()

	// The race area is derived from **every** checkpoint, including the secret one — by design, because the
	// hull plus a 3 km buffer is what makes it safe to publish. Included here so the test covers that
	// claim rather than assuming it. The unsited post is skipped: 0,0 would drag the hull into the Atlantic.
	points := make([]checkpoint.Point, 0, len(mapfixture.Checkpoints()))
	for _, c := range mapfixture.Checkpoints() {
		if c.Lat == 0 && c.Lng == 0 {
			continue
		}
		points = append(points, checkpoint.Point{Lat: c.Lat, Lng: c.Lng})
	}
	area, ok := checkpoint.ComputeRaceArea(points, len(points))
	if !ok {
		t.Fatal("fixture must produce a race area")
	}

	app := newTestApp(t)
	app.config.eventYear = year
	app.models = data.NewModels(
		users.NewMockDirectory(),
		scans.NewMockSource(),
		fakeRaceAreas{area: area, ok: true},
		nil, nil,
		data.WithMapReads(mapfixture.NewRule()),
	)
	return app
}

// The endpoints this PRD touches, and the ones a leak would most plausibly reach.
func revealSurfaces() []string {
	return []string{
		"/api/checkpoints",
		"/api/patrol/scans",
		"/api/race-area",
		"/api/me/profile",
		"/api/contacts/manifest",
	}
}

func TestUnrevealedCheckpointNeverLeavesTheBFF(t *testing.T) {
	app := revealApp(t, "2026")

	srv := httptest.NewServer(app.routes())
	t.Cleanup(srv.Close)
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	// Every rendering of the secret coordinate a JSON encoder might plausibly produce. Checking the bare
	// float is not enough: a truncated or reformatted value is just as much of a leak, and a future change
	// to how coordinates are serialised must not quietly slip past this test.
	forbidden := []string{
		strconv.FormatFloat(secretLat, 'f', -1, 64),
		strconv.FormatFloat(secretLng, 'f', -1, 64),
		fmt.Sprintf("%v", secretLat),
		fmt.Sprintf("%v", secretLng),
		"56.051", // a truncated latitude is still a position
		"9.203",
		"cp-secret", // and the id alone tells a patrol a post exists
		"hemmelig",
	}

	for _, path := range revealSurfaces() {
		resp := getWithCookies(t, srv.URL+path, cookies)
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("%s: read body: %v", path, err)
		}
		// A 403 or 404 is a perfectly good answer for a surface this user cannot see; what matters is that
		// nothing leaked in whatever the answer was.
		if resp.StatusCode >= http.StatusInternalServerError {
			t.Fatalf("%s: status %d: %s", path, resp.StatusCode, body)
		}

		for _, needle := range forbidden {
			if strings.Contains(string(body), needle) {
				t.Errorf("%s leaked %q from an un-revealed checkpoint\nbody: %s", path, needle, body)
			}
		}
	}
}

// The positive half. Without it the test above could be satisfied by an endpoint that returns nothing at
// all, which would "pass" while the feature was broken — the failure mode that makes secrecy tests
// worthless.
func TestRevealedCheckpointsDoReachThePatrol(t *testing.T) {
	app := revealApp(t, "2026")

	srv := httptest.NewServer(app.routes())
	t.Cleanup(srv.Close)
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp := getWithCookies(t, srv.URL+"/api/checkpoints", cookies)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	for _, want := range []string{"cp-1", "Post 1"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("want the revealed checkpoint %q in the response\nbody: %s", want, body)
		}
	}
}

// The race area is derived from every checkpoint including the secret one, and is published anyway — because
// the convex hull plus a 3 km buffer is not a position. Asserted here so that claim is tested rather than
// trusted: if the buffer were ever dropped, or the hull replaced by a point list, the test above would
// start failing on /api/race-area and this comment would explain why.
func TestRaceAreaIsAHullNotAPointList(t *testing.T) {
	app := revealApp(t, "2026")

	srv := httptest.NewServer(app.routes())
	t.Cleanup(srv.Close)
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp := getWithCookies(t, srv.URL+"/api/race-area", cookies)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", resp.StatusCode, body)
	}
	// It must carry a polygon (so it is useful) and no checkpoint id (so it is not a list of posts).
	if !strings.Contains(string(body), `"polygon"`) {
		t.Errorf("want a polygon: %s", body)
	}
	for _, forbidden := range []string{"cp-1", "cp-secret", "checkpointId"} {
		if strings.Contains(string(body), forbidden) {
			t.Errorf("the race area must not name checkpoints, found %q: %s", forbidden, body)
		}
	}
}

// The repo's hard rule, asserted on the map surfaces: a guardian's phone number reaches exactly **one**
// place in this app — a user confirming or approving *their own* — and never a map, a directory, a contact
// list or a cached manifest. The person it belongs to is not a user of this app and never agreed to be in
// it.
//
// `/api/me/profile` is that one place, so it is excluded here by name rather than by omission. Writing this
// test the obvious way — every surface — failed on it, which is the rule working: the exception is real,
// it is documented in `.rules` and PRD 003/005, and it is the caller's own guardian being shown to the
// person who has to confirm it. Excluding it silently would have left the next reader unable to tell a
// sanctioned exception from a gap in the test.
func TestNoGuardianPhoneOnMapSurfaces(t *testing.T) {
	app := revealApp(t, "2026")

	srv := httptest.NewServer(app.routes())
	t.Cleanup(srv.Close)
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	for _, path := range revealSurfaces() {
		// The one sanctioned surface: the owner confirming their own guardian's number.
		if path == "/api/me/profile" {
			continue
		}

		resp := getWithCookies(t, srv.URL+path, cookies)
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("%s: read body: %v", path, err)
		}
		for _, needle := range []string{"phoneParent", "phone_parent"} {
			if strings.Contains(string(body), needle) {
				t.Errorf("%s carries %q: guardian numbers must be projected out in the BFF\nbody: %s",
					path, needle, body)
			}
		}
	}
}

// And the exception itself, pinned — so that if the profile endpoint ever stops carrying it, that is a
// deliberate change to PRD 003/005 rather than something this test file quietly permitted.
func TestProfileIsTheOnlyGuardianPhoneSurface(t *testing.T) {
	app := revealApp(t, "2026")

	srv := httptest.NewServer(app.routes())
	t.Cleanup(srv.Close)
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp := getWithCookies(t, srv.URL+"/api/me/profile", cookies)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), "phone_parent") {
		t.Errorf("the profile is where a user confirms their own guardian's number (PRD 003/005); "+
			"if that changed, update this test and the rule together\nbody: %s", body)
	}
}
