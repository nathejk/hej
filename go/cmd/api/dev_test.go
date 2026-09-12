package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// newDevTestApp is newTestApp switched into the one environment where the dev routes
// exist, with a PIN store that can be read back (see pin.NewDevStoreWithPlaintextRecall).
// It mirrors what run() builds for ENV=development.
func newDevTestApp(t *testing.T) *application {
	t.Helper()

	app := newTestApp(t)
	app.config.env = envDevelopment
	app.pins = pinStoreFor(app.config)
	return app
}

// The safety property of PRD 014 §8: outside development the route is absent, not present
// and refusing. newTestApp runs with env "testing", which is a non-development env like any
// other, so this is the production shape.
func TestDevPin_RouteAbsentOutsideDevelopment(t *testing.T) {
	app := newTestApp(t)
	if app.config.env == envDevelopment {
		t.Fatalf("the test app is in development, so this test would prove nothing")
	}
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	// A PIN is outstanding, so a registered handler would have something to return —
	// a 404 here cannot be "no PIN issued".
	if _, err := app.pins.Issue("+4530000001"); err != nil {
		t.Fatalf("issue: %v", err)
	}

	resp, err := http.Get(srv.URL + "/api/dev/pin?phone=30000001")
	if err != nil {
		t.Fatalf("GET /api/dev/pin: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 404/405: the dev PIN route must not exist outside %s", resp.StatusCode, envDevelopment)
	}
	if strings.Contains(string(body), "pin") {
		t.Errorf("a non-development response mentions a pin:\n%s", body)
	}
}

func TestDevPin_ReturnsTheIssuedPin(t *testing.T) {
	app := newDevTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	code, err := app.pins.Issue("+4530000001")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	// Unnormalised on purpose: the endpoint must key the same way /auth/request-pin does,
	// or the developer is handed a PIN for a different number (or none at all).
	resp, err := http.Get(srv.URL + "/api/dev/pin?phone=30000001")
	if err != nil {
		t.Fatalf("GET /api/dev/pin: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var payload struct {
		Pin string `json:"pin"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload.Pin != code {
		t.Errorf("pin = %q, want the issued %q", payload.Pin, code)
	}
}

// The PIN must still verify afterwards: reading it is a diagnostic, not a login attempt.
func TestDevPin_ReadDoesNotConsumeThePin(t *testing.T) {
	app := newDevTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	code, err := app.pins.Issue("+4530000001")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	resp, err := http.Get(srv.URL + "/api/dev/pin?phone=30000001")
	if err != nil {
		t.Fatalf("GET /api/dev/pin: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if err := app.pins.Verify("+4530000001", code); err != nil {
		t.Errorf("verify after a dev read: %v, want the PIN to still be usable", err)
	}
}

// No PIN outstanding is a bare 404. Identical to an unknown number, so the endpoint cannot
// be used to discover which numbers exist in a dev database full of real ones.
func TestDevPin_404WhenNoPinIssued(t *testing.T) {
	app := newDevTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	known, err := http.Get(srv.URL + "/api/dev/pin?phone=30000001")
	if err != nil {
		t.Fatalf("GET known: %v", err)
	}
	knownBody, _ := io.ReadAll(known.Body)
	known.Body.Close()

	unknown, err := http.Get(srv.URL + "/api/dev/pin?phone=29999999")
	if err != nil {
		t.Fatalf("GET unknown: %v", err)
	}
	unknownBody, _ := io.ReadAll(unknown.Body)
	unknown.Body.Close()

	if known.StatusCode != http.StatusNotFound {
		t.Errorf("status for a known number with no PIN = %d, want 404", known.StatusCode)
	}
	if unknown.StatusCode != http.StatusNotFound {
		t.Errorf("status for an unknown number = %d, want 404", unknown.StatusCode)
	}
	if string(knownBody) != string(unknownBody) {
		t.Errorf("the two 404s differ, which makes this a phone-number oracle:\n%s\n%s", knownBody, unknownBody)
	}
}

// The response carries the PIN and nothing else. The dev database holds real personal data
// about minors, and this endpoint is unauthenticated.
//
// Two layers, as in guardiantripwire_test.go: the body scan catches a populated field, the
// type reflection catches a field somebody adds but leaves empty.
func TestDevPin_ResponseCarriesNothingButThePin(t *testing.T) {
	app := newDevTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	code, err := app.pins.Issue("+4530000001")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	resp, err := http.Get(srv.URL + "/api/dev/pin?phone=30000001")
	if err != nil {
		t.Fatalf("GET /api/dev/pin: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (a refused request tests nothing)", resp.StatusCode)
	}

	// Exactly one key, and it is the PIN.
	var generic map[string]any
	if err := json.Unmarshal(body, &generic); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(generic) != 1 {
		t.Errorf("body has %d keys, want exactly 1:\n%s", len(generic), body)
	}
	if generic["pin"] != code {
		t.Errorf("pin = %v, want %q", generic["pin"], code)
	}

	// The `.rules` invariant, plus the member fields that have no business here.
	lower := strings.ToLower(string(body))
	for _, forbidden := range append([]string{"name", "role", "team", "section", "user_id", "userid", "phone"}, forbiddenKeys...) {
		if strings.Contains(lower, forbidden) {
			t.Errorf("the dev PIN body carries the forbidden key %q:\n%s", forbidden, body)
		}
	}

	rt := reflect.TypeOf(devPinResponse{})
	assertNoForbiddenFields(t, rt, rt.Name())
	if rt.NumField() != 1 {
		t.Errorf("devPinResponse has %d fields, want 1 — this response must never grow member data", rt.NumField())
	}
}

// A malformed number is a client error, not a 404: it says nothing about who exists.
func TestDevPin_MalformedPhoneIs400(t *testing.T) {
	app := newDevTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/dev/pin")
	if err != nil {
		t.Fatalf("GET /api/dev/pin: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

// The recall store is the dev store only. A production-shaped store must not retain the
// plaintext even if the accessor is somehow called.
func TestPinStoreFor_OnlyDevelopmentCanRecallPlaintext(t *testing.T) {
	for _, env := range []string{"production", "staging", "testing"} {
		store := pinStoreFor(config{env: env})
		if _, err := store.Issue("+4530000001"); err != nil {
			t.Fatalf("issue (%s): %v", env, err)
		}
		if code, ok := store.IssuedPlaintextForDev("+4530000001"); ok {
			t.Errorf("env %q recalled the plaintext PIN %q; only development may", env, code)
		}
	}
}
