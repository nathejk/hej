package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nathejk.dk/internal/reveal"
)

// The handout endpoint reuses the fakeMapReads from checkpoints_test.go — its Handouts method already
// returns the fixture's handouts and err. These tests exercise the response shaping, the empty/503/500
// branches, and the two guarantees that matter most: no other team is ever named, and an unknown sheet is
// listed rather than dropped.

func getHandouts(t *testing.T, app *application, authed bool) (*http.Response, string) {
	t.Helper()
	srv := httptest.NewServer(app.routes())
	t.Cleanup(srv.Close)

	var cookies []*http.Cookie
	if authed {
		cookies = authedCookies(t, app, srv, "30000001", "+4530000001")
	}
	resp := getWithCookies(t, srv.URL+"/api/patrol/handouts", cookies)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, string(body)
}

func TestHandouts_ReturnsThePatrolSheets(t *testing.T) {
	app := checkpointsApp(t, fakeMapReads{handouts: []reveal.Handout{
		{Sheet: "kort-1", Name: "Etape 1", Format: "a4", QrID: "1042", HandedOutUts: 1750000000, StillHeld: true},
	}}, "2026")

	resp, body := getHandouts(t, app, true)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, body)
	}

	var got handoutsResponse
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v: %s", err, body)
	}
	if len(got.Handouts) != 1 {
		t.Fatalf("want 1 handout, got %d: %s", len(got.Handouts), body)
	}
	h := got.Handouts[0]
	if h.Name != "Etape 1" || h.Format != "a4" || h.QrID != "1042" || !h.StillHeld {
		t.Errorf("got %+v", h)
	}
	// Unix seconds converted to a timestamp, matching scanned_at's format across the API.
	if h.HandedOut.Unix() != 1750000000 {
		t.Errorf("handed_out did not convert: %v", h.HandedOut)
	}
}

// An unknown sheet ("" id) is a QR the patrol holds whose sheet was never recorded. It must be listed —
// dropping it would hide a sheet in their hand — and named "Ukendt kort", the wording HQ's patrol page uses.
func TestHandouts_UnknownSheetIsListedAsUkendtKort(t *testing.T) {
	app := checkpointsApp(t, fakeMapReads{handouts: []reveal.Handout{
		{Sheet: "", Name: "", QrID: "9999", HandedOutUts: 1750000000, StillHeld: true},
	}}, "2026")

	_, body := getHandouts(t, app, true)

	var got handoutsResponse
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v: %s", err, body)
	}
	if len(got.Handouts) != 1 {
		t.Fatalf("an unknown sheet must still be listed, got %d: %s", len(got.Handouts), body)
	}
	if got.Handouts[0].Name != "Ukendt kort" {
		t.Errorf("want 'Ukendt kort', got %q", got.Handouts[0].Name)
	}
}

// A synthesised handout (a skitse handed over at a post) has no sticker number. The row must lay out
// without a QR rather than showing an empty one.
func TestHandouts_SynthesisedHasNoQr(t *testing.T) {
	app := checkpointsApp(t, fakeMapReads{handouts: []reveal.Handout{
		{Sheet: "kort-skitse", Name: "Skitse til etape 2", Format: "skitse", QrID: "", HandedOutUts: 1750000500, StillHeld: true, Synthesised: true},
	}}, "2026")

	_, body := getHandouts(t, app, true)

	var got handoutsResponse
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Handouts[0].QrID != "" {
		t.Errorf("a synthesised handout must have no sticker number, got %q", got.Handouts[0].QrID)
	}
}

// The response carries nothing beyond what the section needs, and — the point of this test — nothing about
// the team a reassigned sheet moved to. Asserted on the serialised keys so a field added to the Go struct
// later cannot reach a participant unnoticed.
func TestHandouts_NeverNamesAnotherTeam(t *testing.T) {
	app := checkpointsApp(t, fakeMapReads{handouts: []reveal.Handout{
		// A sheet the patrol no longer holds: HQ would show who it moved to; we must not.
		{Sheet: "kort-1", Name: "Etape 1", Format: "a4", QrID: "1042", HandedOutUts: 1750000000, StillHeld: false},
	}}, "2026")

	_, body := getHandouts(t, app, true)

	var raw struct {
		Handouts []map[string]any `json:"handouts"`
	}
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(raw.Handouts) != 1 {
		t.Fatalf("want 1 handout: %s", body)
	}

	want := map[string]bool{
		"name": true, "format": true, "qr_id": true, "handed_out": true, "still_held": true,
	}
	for key := range raw.Handouts[0] {
		if !want[key] {
			t.Errorf("unexpected field %q in the handout response", key)
		}
		// Belt and braces: no key may hint at another team, whatever it is named.
		lower := strings.ToLower(key)
		for _, banned := range []string{"team", "successor", "moved", "flyttet", "hold"} {
			if strings.Contains(lower, banned) {
				t.Errorf("field %q must not name or reference another team", key)
			}
		}
	}
	for key := range want {
		if _, ok := raw.Handouts[0][key]; !ok {
			t.Errorf("missing field %q", key)
		}
	}
}

// Empty is a normal state — a patrol before its first handout, and every personnel user without a patrol.
// 200 with `[]`, matching /api/patrol/scans and /api/checkpoints.
func TestHandouts_EmptyIsTwoHundred(t *testing.T) {
	app := checkpointsApp(t, fakeMapReads{}, "2026")

	resp, body := getHandouts(t, app, true)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, body)
	}
	if !strings.Contains(body, `"handouts":[]`) {
		t.Errorf("want an empty array, not null: %s", body)
	}
}

func TestHandouts_RequiresAuth(t *testing.T) {
	app := checkpointsApp(t, fakeMapReads{}, "2026")

	resp, _ := getHandouts(t, app, false)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

// The nil-interface trap, same as checkpoints: a nil concrete rule assigned to the interface is not a nil
// interface, so without mapReadsOrNil this would panic instead of answering 503.
func TestHandouts_NoProjectionIsUnavailable(t *testing.T) {
	app := checkpointsApp(t, mapReadsOrNil(nil), "2026")

	resp, body := getHandouts(t, app, true)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503: %s", resp.StatusCode, body)
	}
}

// A read failure is a 500, not an empty list — the client may cache this, and an empty answer would persist
// a transient database problem.
func TestHandouts_ReadFailureIsNotAnEmptyList(t *testing.T) {
	app := checkpointsApp(t, fakeMapReads{err: errors.New("database is down")}, "2026")

	resp, body := getHandouts(t, app, true)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", resp.StatusCode, body)
	}
	if strings.Contains(body, `"handouts":[]`) {
		t.Error("a failure must not be reported as an empty list")
	}
}
