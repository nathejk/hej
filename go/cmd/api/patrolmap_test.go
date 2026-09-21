package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"nathejk.dk/nathejk/table/album"
	"nathejk.dk/nathejk/table/trackpoint"
)

// The public map's data endpoints (task 342).

func decodeMap(t *testing.T, body []byte) patrolMapResponse {
	t.Helper()
	var out patrolMapResponse
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decoding the map response: %v\n%s", err, body)
	}
	return out
}

// mapApp is a patrol page app with a track wired in.
func mapApp(t *testing.T) (*application, *httptest.Server) {
	t.Helper()

	app, _, srv := patrolPageApp(t)
	app.patrolTracks = trackReader(t,
		&trackPeople{members: map[string][]string{"team-42": {"p1", "p2"}}},
		&trackPoints{byPerson: map[string][]trackpoint.Point{
			"p1": walk(12.200, 10),
			"p2": walk(12.300, 10),
		}},
	)
	return app, srv
}

func TestPatrolMapServesTheTrackAndScans(t *testing.T) {
	_, srv := mapApp(t)

	resp, body := getPublic(t, srv.URL+"/api/public/patrol/42/map", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, body)
	}
	out := decodeMap(t, body)

	if len(out.Track) != 2 {
		t.Errorf("want one segment per recording member, got %d", len(out.Track))
	}
	if out.Recorders != 2 {
		t.Errorf("want 2 recorders, got %d", out.Recorders)
	}
	// Two of the fixture's three scans carry a position.
	if len(out.Scans) != 2 {
		t.Errorf("want 2 plottable scans, got %d", len(out.Scans))
	}
	for _, s := range out.Scans {
		if s.Lat == 0 || s.Lng == 0 {
			t.Errorf("a plotted scan has no position: %+v", s)
		}
		if s.Kind == "" {
			t.Errorf("a scan needs a kind so the marker can differ: %+v", s)
		}
	}
}

// **A multi-segment shape, so a gap stays a gap.** Leaflet draws an array of arrays as separate strokes; one
// flat list would draw a confident line through terrain nobody walked.
func TestPatrolMapTrackIsSegmentedNotFlattened(t *testing.T) {
	app, srv := mapApp(t)
	// One member with a long silence in the middle.
	points := append(walk(12.200, 5), func() []trackpoint.Point {
		later := walk(12.300, 5)
		for i := range later {
			later[i].TS += int64(2 * time.Hour / time.Millisecond)
		}
		return later
	}()...)
	app.patrolTracks = trackReader(t,
		&trackPeople{members: map[string][]string{"team-42": {"p1"}}},
		&trackPoints{byPerson: map[string][]trackpoint.Point{"p1": points}},
	)

	_, body := getPublic(t, srv.URL+"/api/public/patrol/42/map", nil)
	out := decodeMap(t, body)

	if len(out.Track) != 2 {
		t.Fatalf("a two-hour silence must produce two segments, got %d", len(out.Track))
	}
	// And no segment may span the gap.
	for i, seg := range out.Track {
		if len(seg) < 2 {
			t.Errorf("segment %d has %d points: a line needs two", i, len(seg))
		}
		spread := seg[len(seg)-1][1] - seg[0][1]
		if spread > 0.05 {
			t.Errorf("segment %d spans %f degrees of longitude: it bridged the gap", i, spread)
		}
	}
}

// **The serialisation half of the timestamp rule.** `patroltrack.Point` carries a time for the distance
// estimate; it must not cross the wire, because precise times on a merged track let a reader infer that two
// overlapping segments belong to different people.
func TestPatrolMapCarriesNoTimestamps(t *testing.T) {
	_, srv := mapApp(t)

	_, body := getPublic(t, srv.URL+"/api/public/patrol/42/map", nil)
	payload := string(body)

	for _, forbidden := range []string{"ts", "TS", "time", "at", "recorded"} {
		if strings.Contains(payload, `"`+forbidden+`"`) {
			t.Errorf("the map payload carries %q: a time on a merged track is one inference away from "+
				"which member walked which segment\n%s", forbidden, payload)
		}
	}

	// And structurally: a track point is exactly two numbers.
	out := decodeMap(t, body)
	for i, seg := range out.Track {
		for j, p := range seg {
			if len(p) != 2 {
				t.Errorf("segment %d point %d has %d values, want 2", i, j, len(p))
			}
		}
	}
}

// No person may appear, in any form.
func TestPatrolMapCarriesNoPerson(t *testing.T) {
	_, srv := mapApp(t)

	_, body := getPublic(t, srv.URL+"/api/public/patrol/42/map", nil)
	payload := string(body)

	for _, forbidden := range []string{"p1", "p2", "person", "member", "user"} {
		if strings.Contains(payload, forbidden) {
			t.Errorf("the map payload mentions %q\n%s", forbidden, payload)
		}
	}
}

// **The map endpoint is gated exactly as the page is, through the same code.** Two access decisions for one
// patrol is how the JSON stays open after the page closes — which would be the whole course in
// machine-readable form.
func TestPatrolMapIsClosedWhenThePageIs(t *testing.T) {
	_, srv := mapApp(t)

	for _, number := range []string{
		"43",     // exists, has not finished
		"999999", // does not exist
	} {
		resp, body := getPublic(t, srv.URL+"/api/public/patrol/"+number+"/map", nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: want 404 while the page is closed, got %d: %s", number, resp.StatusCode, body)
		}
		// And the refusal must carry nothing about the patrol.
		for _, forbidden := range []string{"Ulvene", "team-43", "2. Gruppe"} {
			if strings.Contains(string(body), forbidden) {
				t.Errorf("%s: the refusal leaks %q", number, forbidden)
			}
		}
	}
}

// With no patrol projection, the endpoint is closed rather than unavailable — the same fail-closed choice
// the page makes, for the same reason.
func TestPatrolMapFailsClosedWithoutAProjection(t *testing.T) {
	app, srv := mapApp(t)
	app.models.PublicPatrols = nil

	resp, _ := getPublic(t, srv.URL+"/api/public/patrol/42/map", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

// A patrol that recorded nothing gets an empty track rather than an error: the common case (task 082).
func TestPatrolMapEmptyTrackIsNotAnError(t *testing.T) {
	app, srv := mapApp(t)
	app.patrolTracks = trackReader(t,
		&trackPeople{members: map[string][]string{"team-42": {"p1"}}},
		&trackPoints{},
	)

	resp, body := getPublic(t, srv.URL+"/api/public/patrol/42/map", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	out := decodeMap(t, body)
	if len(out.Track) != 0 {
		t.Errorf("want no segments, got %d", len(out.Track))
	}
	// Empty arrays rather than `null`, so a client can iterate unconditionally. Asserted on the payload
	// because the decoded form cannot tell the two apart.
	if strings.Contains(string(body), "null") {
		t.Errorf("want empty arrays rather than null\n%s", body)
	}
}

// An un-positioned scan is listed on the page and absent from the map. The page explains the difference.
func TestPatrolMapOmitsUnplottableScans(t *testing.T) {
	_, srv := mapApp(t)

	_, body := getPublic(t, srv.URL+"/api/public/patrol/42/map", nil)
	out := decodeMap(t, body)

	// The fixture has a bandit catch with no position; it must not be a pin.
	for _, s := range out.Scans {
		if s.Kind == "bandit" {
			t.Errorf("the fixture's bandit catch has no position and must not be plotted: %+v", s)
		}
	}
}

func TestAlbumMapServesLocatedPhotographs(t *testing.T) {
	app, store := albumApp(t)
	// The seeded album fixture has one inside-bounds item on a published album.
	store.albums[0].items[0].BoundsVerdict = album.BoundsInside
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, body := getPublic(t, srv.URL+"/api/public/albums", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, body)
	}

	var out albumMapResponse
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decoding: %v\n%s", err, body)
	}
	// The stub's Plottable returns nothing, so this asserts the shape rather than the content — the
	// projection's own filter is tested in task 333/334 and by the SQL run against real rows.
	if !strings.Contains(string(body), `"photos"`) {
		t.Errorf("want a photos array\n%s", body)
	}
}

// **No glimt may reach the map.** A member tapping "Offentligt" agreed to share a photograph, not a
// position — and this is the endpoint that would carry one if somebody added it.
func TestAlbumMapCarriesNoGlimt(t *testing.T) {
	app, _ := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/api/public/albums", nil)
	payload := string(body)

	for _, forbidden := range []string{"glimt", "g-public", "Glimt"} {
		if strings.Contains(payload, forbidden) {
			t.Errorf("the album map source mentions %q; a glimt has no route to the map\n%s",
				forbidden, payload)
		}
	}
}

func TestAlbumMapIsUnavailableWithoutAProjection(t *testing.T) {
	app, _ := albumApp(t)
	app.models.Albums = nil
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, _ := getPublic(t, srv.URL+"/api/public/albums", nil)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("want 503, got %d", resp.StatusCode)
	}
}

// Both map endpoints must ignore the session like the rest of the surface.
func TestMapEndpointsIgnoreTheSession(t *testing.T) {
	app, srv := mapApp(t)
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	for _, path := range []string{"/api/public/patrol/42/map", "/api/public/albums"} {
		_, anonymous := getPublic(t, srv.URL+path, nil)
		_, signedIn := getPublic(t, srv.URL+path, cookies)
		if string(anonymous) != string(signedIn) {
			t.Errorf("%s differs for a signed-in member", path)
		}
	}
}

// **The track now raises the distance**, which task 341 shipped unable to do because `patroltrack.Point`
// had no timestamp. A wandering track between two scans must produce a larger figure than the straight line.
func TestTheTrackRaisesTheDistanceEstimate(t *testing.T) {
	app, _, srv := patrolPageApp(t)

	// Without a track: the straight-line floor between the fixture's two positioned scans.
	app.patrolTracks = trackReader(t,
		&trackPeople{members: map[string][]string{"team-42": {"p1"}}},
		&trackPoints{},
	)
	_, floorBody := getPublic(t, srv.URL+"/2026/patrulje/42", nil)

	// With a track that wanders well east and back between them.
	//
	// **Two-minute spacing, not thirty.** An earlier version of this test sampled every 30 minutes and the
	// figure did not move at all — because that is past `patroltrack.GapThreshold`, so every point became
	// its own one-point run and every run was dropped as undrawable. A fixture has to sample like the client
	// does, or it tests the gap rule instead of the thing it meant to.
	var wandering []trackpoint.Point
	base := nightAt(0).UnixMilli()
	for i := 0; i <= 60; i++ {
		// East then back on alternate samples: a far longer path than the direct line north.
		lng := 12.200 + 0.01*float64(i%2)
		wandering = append(wandering, trackpoint.Point{
			TS:  base + int64(i)*int64(2*time.Minute/time.Millisecond),
			Lat: 55.700 + 0.0008*float64(i),
			Lng: lng,
		})
	}
	app.patrolTracks = trackReader(t,
		&trackPeople{members: map[string][]string{"team-42": {"p1"}}},
		&trackPoints{byPerson: map[string][]trackpoint.Point{"p1": wandering}},
	)
	_, raisedBody := getPublic(t, srv.URL+"/2026/patrulje/42", nil)

	floor := distanceKmFromPage(t, string(floorBody))
	raised := distanceKmFromPage(t, string(raisedBody))

	if raised <= floor {
		t.Errorf("the track should have raised the figure: floor %d km, with track %d km", floor, raised)
	}
}

// distanceKmFromPage pulls the kilometre figure out of a rendered patrol page.
//
// Parsed to an integer rather than compared as a string, because the labels sort lexically and
// "mindst ~37 km" is *less* than "mindst ~5 km" — which is how an earlier version of this test reported a
// failure for a figure that had in fact risen from 5 km to 37.
func distanceKmFromPage(t *testing.T, page string) int {
	t.Helper()

	const marker = `class="distance">`
	i := strings.Index(page, marker)
	if i < 0 {
		t.Fatalf("no distance rendered on the page")
	}
	rest := page[i+len(marker):]
	j := strings.Index(rest, "<")
	if j < 0 {
		t.Fatalf("malformed distance markup")
	}
	label := strings.TrimSpace(rest[:j])

	digits := strings.Builder{}
	for _, r := range label {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	if digits.Len() == 0 {
		t.Fatalf("no kilometre figure in %q", label)
	}
	km, err := strconv.Atoi(digits.String())
	if err != nil {
		t.Fatalf("parsing %q: %v", label, err)
	}
	return km
}

// **Every asset the page references must actually be checked in.** The island's Leaflet and
// leaflet.markercluster builds are vendored into vue/public/vendor by scripts/vendor-leaflet.sh (a build
// input, not a build step), so the failure mode is an upgrade that forgets to re-run it: the template asks
// for a file nobody committed and the map silently stops drawing in production while every other test
// passes. This test reads the paths out of the rendered page rather than repeating them, so a new asset is
// covered the moment it is added.
//
// **Skipped where the frontend tree is not present.** The dev container for the api mounts only `go/`, so
// there is no `vue/public` to look at and a failure there would be about the mount rather than about the
// assets. A full checkout — a developer's machine, CI — has both, which is where this needs to bite.
func TestIslandAssetsAreVendored(t *testing.T) {
	publicDir := filepath.Join("..", "..", "..", "vue", "public")
	if _, err := os.Stat(publicDir); err != nil {
		t.Skipf("no frontend tree at %s (api-only checkout); nothing to verify", publicDir)
	}

	app, srv := mapApp(t)
	_ = app

	_, body := getPublic(t, srv.URL+"/2026/patrulje/42", nil)

	refs := assetRefs(string(body))
	if len(refs) == 0 {
		t.Fatal("the page referenced no island assets; the fixture no longer renders a map")
	}
	for _, ref := range refs {
		path := filepath.Join(publicDir, filepath.FromSlash(strings.TrimPrefix(ref, "/")))
		if _, err := os.Stat(path); err != nil {
			t.Errorf("the page references %s but %s is not checked in (re-run vue/scripts/vendor-leaflet.sh): %v",
				ref, path, err)
		}
	}
}

// assetRefs pulls every same-origin src= and href= out of a rendered page.
func assetRefs(page string) []string {
	var out []string
	for _, attr := range []string{`src="`, `href="`} {
		rest := page
		for {
			i := strings.Index(rest, attr)
			if i < 0 {
				break
			}
			rest = rest[i+len(attr):]
			j := strings.Index(rest, `"`)
			if j < 0 {
				break
			}
			ref := rest[:j]
			rest = rest[j:]
			// Only static files under the SPA's public root: /api/... is served by this binary and
			// /2026/... is a page, neither of which is a file on disk.
			if strings.HasPrefix(ref, "/vendor/") || ref == "/publicmap.js" {
				out = append(out, ref)
			}
		}
	}
	return out
}
