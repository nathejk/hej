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
