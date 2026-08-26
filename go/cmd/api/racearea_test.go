package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nathejk.dk/internal/data"
	"nathejk.dk/internal/scans"
	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/checkpoint"
)

type fakeRaceAreas struct {
	area checkpoint.RaceArea
	ok   bool
	err  error
	// years records what the handler asked for, so it cannot quietly use the wrong one.
	years *[]string
}

func (f fakeRaceAreas) RaceArea(year string) (checkpoint.RaceArea, bool, error) {
	if f.years != nil {
		*f.years = append(*f.years, year)
	}
	return f.area, f.ok, f.err
}

// raceAreaApp wires a test app with the given (possibly nil) race-area source.
func raceAreaApp(t *testing.T, areas checkpoint.Queries, year string) *application {
	t.Helper()
	app := newTestApp(t)
	app.config.eventYear = year
	app.models = data.NewModels(users.NewMockDirectory(), scans.NewMockSource(), areas)
	return app
}

func getRaceArea(t *testing.T, app *application, authed bool) (*http.Response, string) {
	t.Helper()
	srv := httptest.NewServer(app.routes())
	t.Cleanup(srv.Close)

	var cookies []*http.Cookie
	if authed {
		cookies = authedCookies(t, app, srv, "30000001", "+4530000001")
	}
	resp := getWithCookies(t, srv.URL+"/api/race-area", cookies)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, string(body)
}

// The 2026 checkpoints, so the fixture exercises a realistic hull.
var testCheckpoints = []checkpoint.Point{
	{Lat: 55.716595, Lng: 12.264819},
	{Lat: 55.852480717942505, Lng: 12.19563961029053},
	{Lat: 55.651587358831390, Lng: 12.12321996688843},
	{Lat: 55.830986, Lng: 12.057753},
}

func TestRaceArea_ReturnsThePolygon(t *testing.T) {
	area, ok := checkpoint.ComputeRaceArea(testCheckpoints, 12)
	if !ok {
		t.Fatal("fixture must produce an area")
	}

	var asked []string
	app := raceAreaApp(t, fakeRaceAreas{area: area, ok: true, years: &asked}, "2026")

	resp, body := getRaceArea(t, app, true)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, body)
	}

	var got raceAreaResponse
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Polygon) < 3 {
		t.Errorf("polygon has %d vertices, want a real polygon", len(got.Polygon))
	}
	if got.AreaKm2 <= 0 {
		t.Errorf("area_km2 = %v, want positive", got.AreaKm2)
	}
	if got.BufferKm != checkpoint.BufferKm {
		t.Errorf("buffer_km = %v, want %v — the client must know the margin is already "+
			"included so it does not add its own", got.BufferKm, checkpoint.BufferKm)
	}
	if got.PositionedCount != len(testCheckpoints) || got.TotalCount != 12 {
		t.Errorf("counts = %d/%d, want %d/12", got.PositionedCount, got.TotalCount,
			len(testCheckpoints))
	}
	// Bounds must contain every checkpoint — that is the property the tile cache relies on.
	for _, cp := range testCheckpoints {
		if cp.Lat <= got.SouthWest.Lat || cp.Lat >= got.NorthEast.Lat ||
			cp.Lng <= got.SouthWest.Lng || cp.Lng >= got.NorthEast.Lng {
			t.Errorf("checkpoint %+v not inside returned bounds", cp)
		}
	}

	// The year must come from config, not from the request or the clock.
	if len(asked) != 1 || asked[0] != "2026" {
		t.Errorf("queried years = %v, want [2026]", asked)
	}
}

// The response is a hull and must never carry the checkpoint positions it came from: the
// event area is deliberately not fully known to participants.
func TestRaceArea_DoesNotLeakCheckpointPositions(t *testing.T) {
	area, _ := checkpoint.ComputeRaceArea(testCheckpoints, 12)
	app := raceAreaApp(t, fakeRaceAreas{area: area, ok: true}, "2026")

	_, body := getRaceArea(t, app, true)

	// The full-precision coordinates as they went in. A buffered hull cannot contain them:
	// every vertex is BufferKm away from every input point.
	for _, needle := range []string{
		"55.716595", "12.264819",
		"55.852480717942505", "12.19563961029053",
		"55.65158735883139", "12.12321996688843",
		"55.830986", "12.057753",
	} {
		if strings.Contains(body, needle) {
			t.Errorf("response contains checkpoint coordinate %s\n%s", needle, body)
		}
	}
}

// No derivable area is 404, not an empty 200. The caller's next move is downloading hundreds
// of megabytes of tiles, and an empty polygon mistaken for "cache everything" would try to
// cache the whole country. This is a normal state early in the year.
func TestRaceArea_404WhenNoAreaYet(t *testing.T) {
	app := raceAreaApp(t, fakeRaceAreas{ok: false}, "2026")

	resp, body := getRaceArea(t, app, true)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404: %s", resp.StatusCode, body)
	}
	if strings.Contains(body, "polygon") {
		t.Errorf("404 must not carry a polygon: %s", body)
	}
}

// No projection at all (no database) is 503, not 404: one is a dependency the client should
// retry for, the other is a state of the event. Collapsing them would make a misconfigured
// server look like an event that has not been planned yet.
func TestRaceArea_503WithoutAProjection(t *testing.T) {
	app := raceAreaApp(t, nil, "2026")

	resp, body := getRaceArea(t, app, true)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503: %s", resp.StatusCode, body)
	}
}

func TestRaceArea_500OnQueryError(t *testing.T) {
	app := raceAreaApp(t, fakeRaceAreas{err: errors.New("connection refused")}, "2026")

	resp, _ := getRaceArea(t, app, true)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

// Unauthenticated must not reveal the region.
func TestRaceArea_RequiresASession(t *testing.T) {
	area, _ := checkpoint.ComputeRaceArea(testCheckpoints, 12)
	app := raceAreaApp(t, fakeRaceAreas{area: area, ok: true}, "2026")

	resp, body := getRaceArea(t, app, false)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
	if strings.Contains(body, "polygon") {
		t.Errorf("unauthenticated response leaked the polygon: %s", body)
	}
}
