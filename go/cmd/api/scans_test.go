package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func getWithCookies(t *testing.T, url string, cookies []*http.Cookie) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

func decodeScans(t *testing.T, resp *http.Response) scansResponse {
	t.Helper()
	defer resp.Body.Close()
	var body scansResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body
}

func TestPatrolScans_RequiresAuth(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/patrol/scans")
	if err != nil {
		t.Fatalf("GET scans: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestPatrolScans_ReturnsSeededScansNewestFirst(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	resp := getWithCookies(t, srv.URL+"/api/patrol/scans", cookies)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body := decodeScans(t, resp)

	if len(body.Scans) < 2 {
		t.Fatalf("scans = %d, want the seeded spejder patrol's registrations", len(body.Scans))
	}
	for i := 1; i < len(body.Scans); i++ {
		if body.Scans[i-1].ScannedAt.Before(body.Scans[i].ScannedAt) {
			t.Fatalf("scans not newest first at index %d", i)
		}
	}

	kinds := map[string]int{}
	unpositioned := 0
	for _, s := range body.Scans {
		kinds[s.Kind]++
		if s.ID == "" || s.Label == "" {
			t.Errorf("scan %+v: id and label must be set", s)
		}
		if s.Lat == nil || s.Lng == nil {
			unpositioned++
			continue
		}
		// Fixture coordinates must stay near the map's default view.
		if *s.Lat < 56.0 || *s.Lat > 56.3 || *s.Lng < 9.3 || *s.Lng > 9.6 {
			t.Errorf("scan %s: coordinates %v,%v outside central Jutland", s.ID, *s.Lat, *s.Lng)
		}
	}
	if kinds["checkpoint"] == 0 {
		t.Error("expected at least one checkpoint scan")
	}
	if kinds["bandit"] == 0 {
		t.Error("expected at least one bandit catch")
	}
	if unpositioned == 0 {
		t.Error("expected at least one scan without coordinates")
	}
}

func TestPatrolScans_UserWithoutPatrolGetsEmptyList(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	// The seeded postmandskab is personnel and has no patrol: 200 + empty list,
	// not 404 — the client hides the UI on an empty list.
	cookies := authedCookies(t, app, srv, "30000003", "+4530000003")
	resp := getWithCookies(t, srv.URL+"/api/patrol/scans", cookies)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if body := decodeScans(t, resp); len(body.Scans) != 0 {
		t.Fatalf("scans = %d, want 0", len(body.Scans))
	}
}
