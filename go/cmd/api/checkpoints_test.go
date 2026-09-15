package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nathejk/shared-go/types"

	"nathejk.dk/internal/data"
	"nathejk.dk/internal/reveal"
	"nathejk.dk/internal/scans"
	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/checkpoint"
)

// fakeMapReads records what it was asked about, which is the part worth pinning: the handler must take the
// patrol from the session and never from the request, or the endpoint would let one patrol read another's map.
type fakeMapReads struct {
	revealed []checkpoint.Checkpoint
	next     types.CheckgroupID
	handouts []reveal.Handout
	err      error

	askedPatrols *[]string
	askedYears   *[]string
	askedStarted *[]bool
}

func (f fakeMapReads) Revealed(year string, patrolID string, hasStarted bool) (reveal.RevealedMap, error) {
	if f.askedPatrols != nil {
		*f.askedPatrols = append(*f.askedPatrols, patrolID)
	}
	if f.askedYears != nil {
		*f.askedYears = append(*f.askedYears, year)
	}
	if f.askedStarted != nil {
		*f.askedStarted = append(*f.askedStarted, hasStarted)
	}
	if f.err != nil {
		return reveal.RevealedMap{}, f.err
	}
	return reveal.RevealedMap{Checkpoints: f.revealed, NextCheckgroup: f.next}, nil
}

func (f fakeMapReads) Handouts(string, string) ([]reveal.Handout, error) {
	return f.handouts, f.err
}

func checkpointsApp(t *testing.T, maps data.MapReads, year string) *application {
	t.Helper()
	app := newTestApp(t)
	app.config.eventYear = year
	app.models = data.NewModels(users.NewMockDirectory(), scans.NewMockSource(), nil, nil, nil,
		data.WithMapReads(maps))
	return app
}

func getCheckpoints(t *testing.T, app *application, authed bool) (*http.Response, string) {
	t.Helper()
	srv := httptest.NewServer(app.routes())
	t.Cleanup(srv.Close)

	var cookies []*http.Cookie
	if authed {
		cookies = authedCookies(t, app, srv, "30000001", "+4530000001")
	}
	resp := getWithCookies(t, srv.URL+"/api/checkpoints", cookies)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, string(body)
}

func TestCheckpoints_ReturnsTheRevealedSet(t *testing.T) {
	var patrols, years []string
	app := checkpointsApp(t, fakeMapReads{
		revealed: []checkpoint.Checkpoint{
			{ID: "cp-1", Name: "Post 1", Checkgroup: "cg-1", SortOrder: 0,
				Lat: 56.1382, Lng: 9.5521, OpenFromUts: 1750000000, OpenUntilUts: 1750003600},
		},
		askedPatrols: &patrols,
		askedYears:   &years,
	}, "2026")

	resp, body := getCheckpoints(t, app, true)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, body)
	}

	var got checkpointsResponse
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v: %s", err, body)
	}
	if len(got.Checkpoints) != 1 {
		t.Fatalf("want 1 checkpoint, got %d: %s", len(got.Checkpoints), body)
	}
	c := got.Checkpoints[0]
	if c.ID != "cp-1" || c.Name != "Post 1" || c.Checkgroup != "cg-1" {
		t.Errorf("got %+v", c)
	}
	if c.Lat != 56.1382 || c.Lng != 9.5521 {
		t.Errorf("position: %+v", c)
	}
	if c.OpenFrom != 1750000000 || c.OpenUntil != 1750003600 {
		t.Errorf("window: %+v", c)
	}
	if len(years) != 1 || years[0] != "2026" {
		t.Errorf("handler must ask for the configured year, asked %v", years)
	}
}

// The patrol comes from the session, never from the request. Pinned because the alternative — a patrol id
// in a query parameter — is the shape of bug that lets one patrol read another's map, and it would look
// entirely reasonable in review.
func TestCheckpoints_PatrolComesFromTheSession(t *testing.T) {
	var patrols []string
	app := checkpointsApp(t, fakeMapReads{askedPatrols: &patrols}, "2026")

	srv := httptest.NewServer(app.routes())
	t.Cleanup(srv.Close)
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	// A patrol id in the query string must be ignored entirely.
	resp := getWithCookies(t, srv.URL+"/api/checkpoints?patrol=team-somebody-else", cookies)
	defer resp.Body.Close()

	if len(patrols) != 1 {
		t.Fatalf("want one read, got %d", len(patrols))
	}
	if patrols[0] == "team-somebody-else" {
		t.Fatal("the handler used a patrol id from the request")
	}
}

// Empty is a normal state — a patrol before its first handout, and every personnel user without a patrol.
// 200 with `[]`, deliberately unlike /api/race-area's 404: there, "nothing" precedes a few-hundred-megabyte
// tile download and must be hard to misread. Here the client simply draws no markers.
func TestCheckpoints_EmptyIsTwoHundred(t *testing.T) {
	app := checkpointsApp(t, fakeMapReads{}, "2026")

	resp, body := getCheckpoints(t, app, true)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, `"checkpoints":[]`) {
		t.Errorf("want an empty array, not null: %s", body)
	}
}

func TestCheckpoints_RequiresAuth(t *testing.T) {
	app := checkpointsApp(t, fakeMapReads{}, "2026")

	resp, _ := getCheckpoints(t, app, false)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

// The nil-interface trap every handler here is tested against: a nil concrete projection assigned to an
// interface is not a nil interface, so without `mapReadsOrNil` this would panic instead of answering 503.
func TestCheckpoints_NoProjectionIsUnavailable(t *testing.T) {
	app := checkpointsApp(t, mapReadsOrNil(nil), "2026")

	resp, body := getCheckpoints(t, app, true)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503: %s", resp.StatusCode, body)
	}
}

// A read failure is a 500, not an empty list. The difference matters more here than usual: the client
// caches this response offline, so an empty answer would persist a transient database problem for the rest
// of the night.
func TestCheckpoints_ReadFailureIsNotAnEmptyMap(t *testing.T) {
	app := checkpointsApp(t, fakeMapReads{err: errors.New("database is down")}, "2026")

	resp, body := getCheckpoints(t, app, true)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", resp.StatusCode, body)
	}
	if strings.Contains(body, `"checkpoints":[]`) {
		t.Error("a failure must not be reported as an empty map")
	}
}

// The response carries nothing beyond what the map needs. Asserted on the serialised keys, because a field
// added to the Go struct later would otherwise reach the client unnoticed — and this is the endpoint where
// an extra field is most likely to be one about a checkpoint the patrol has not earned.
func TestCheckpoints_ResponseCarriesOnlyTheExpectedFields(t *testing.T) {
	app := checkpointsApp(t, fakeMapReads{
		revealed: []checkpoint.Checkpoint{{ID: "cp-1", Name: "Post 1", Lat: 56.1, Lng: 9.5}},
	}, "2026")

	_, body := getCheckpoints(t, app, true)

	var raw struct {
		Checkpoints []map[string]any `json:"checkpoints"`
	}
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(raw.Checkpoints) != 1 {
		t.Fatalf("want 1 checkpoint: %s", body)
	}

	want := map[string]bool{
		"id": true, "name": true, "checkgroup": true, "sort_order": true,
		"lat": true, "lng": true, "open_from": true, "open_until": true,
		"open_duration_minutes": true,
	}
	for key := range raw.Checkpoints[0] {
		if !want[key] {
			t.Errorf("unexpected field %q in the checkpoint response", key)
		}
	}
	for key := range want {
		if _, ok := raw.Checkpoints[0][key]; !ok {
			t.Errorf("missing field %q", key)
		}
	}
}

// The line the patrol is heading for. Decided in the BFF because it needs route order across checkgroups and
// whether the patrol has started — neither of which the client can honestly hold (task 275).
func TestCheckpoints_ReportsTheNextLine(t *testing.T) {
	app := checkpointsApp(t, fakeMapReads{
		revealed: []checkpoint.Checkpoint{{ID: "cp-1a", Checkgroup: "cg-1", Lat: 56.1, Lng: 9.5}},
		next:     "cg-1",
	}, "2026")

	_, body := getCheckpoints(t, app, true)

	var got checkpointsResponse
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v: %s", err, body)
	}
	if got.NextCheckgroup != "cg-1" {
		t.Errorf("next_checkgroup = %q, want cg-1: %s", got.NextCheckgroup, body)
	}
}

// At the end of the route there is no next line, and the field must be present-and-empty rather than absent:
// the client branches on it, and a missing key would read as undefined.
func TestCheckpoints_NoNextLineAtTheEnd(t *testing.T) {
	app := checkpointsApp(t, fakeMapReads{next: ""}, "2026")

	_, body := getCheckpoints(t, app, true)

	if !strings.Contains(body, `"next_checkgroup":""`) {
		t.Errorf("want an empty next_checkgroup rather than a missing one: %s", body)
	}
}

// The handler must pass the caller's started state through, since that is what retires the start line. Without
// a person projection it is false, which is the safe direction — an arrow towards the start is
// over-informative, whereas wrongly retiring the first line would hide it from a patrol still standing there.
func TestCheckpoints_PassesTheStartedStateThrough(t *testing.T) {
	var started []bool
	app := checkpointsApp(t, fakeMapReads{askedStarted: &started}, "2026")

	getCheckpoints(t, app, true)

	if len(started) != 1 {
		t.Fatalf("want one read, got %d", len(started))
	}
	if started[0] {
		t.Error("want false without a person projection")
	}
}
