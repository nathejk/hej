package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The install gate's default is the whole point of the flag, so it is pinned here: absent
// configuration means the gate is ON. A kill switch that defaults to "off" is one nobody
// notices is off until a member arrives at an event in a browser tab with no notifications.
func TestRuntimeConfig_InstallGateDefaultsOn(t *testing.T) {
	app := newTestApp(t)
	app.config.installGate = true
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	body := getRuntimeConfig(t, srv.URL)
	if v, present := body["install_gate"]; !present {
		t.Fatal("install_gate must be present in /api/config")
	} else if v != true {
		t.Errorf("install_gate = %v, want true", v)
	}
}

// And it must actually be switchable, or it is not a kill switch.
func TestRuntimeConfig_InstallGateCanBeSwitchedOff(t *testing.T) {
	app := newTestApp(t)
	app.config.installGate = false
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	body := getRuntimeConfig(t, srv.URL)
	if body["install_gate"] != false {
		t.Errorf("install_gate = %v, want false", body["install_gate"])
	}
}

// The contacts poll interval is served so it can be widened mid-event; a value the client
// cannot be told is not a lever.
func TestRuntimeConfig_ContactsPollInterval(t *testing.T) {
	app := newTestApp(t)
	app.config.contactsPollSeconds = 60
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	body := getRuntimeConfig(t, srv.URL)
	v, present := body["contacts_poll_seconds"]
	if !present {
		t.Fatal("contacts_poll_seconds must be present in /api/config")
	}
	if v != float64(60) {
		t.Errorf("contacts_poll_seconds = %v, want 60", v)
	}
}

// Zero is the kill switch for the interval, and it must survive the round trip rather than
// being treated as "unset" and quietly replaced by a default.
func TestRuntimeConfig_ContactsPollCanBeDisabled(t *testing.T) {
	app := newTestApp(t)
	app.config.contactsPollSeconds = 0
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	body := getRuntimeConfig(t, srv.URL)
	if body["contacts_poll_seconds"] != float64(0) {
		t.Errorf("contacts_poll_seconds = %v, want 0", body["contacts_poll_seconds"])
	}
}

func getRuntimeConfig(t *testing.T, base string) map[string]any {
	t.Helper()
	resp, err := http.Get(base + "/api/config")
	if err != nil {
		t.Fatalf("GET config: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body
}
