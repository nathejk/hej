package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func healthcheckBody(t *testing.T, app *application) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/healthcheck", nil)
	rec := httptest.NewRecorder()
	app.healthcheckHandler(rec, req)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode healthcheck body: %v (raw: %s)", err, rec.Body.String())
	}
	return rec.Code, body
}

func dependency(t *testing.T, body map[string]any, name string) map[string]any {
	t.Helper()
	deps, ok := body["dependencies"].(map[string]any)
	if !ok {
		t.Fatalf("no dependencies object in %v", body)
	}
	d, ok := deps[name].(map[string]any)
	if !ok {
		t.Fatalf("no %q dependency in %v", name, deps)
	}
	return d
}

// No database configured is a legitimate mode today, so it must report "absent"
// and still be ready — the app runs on mocks.
func TestHealthcheckWithoutDatabaseIsReady(t *testing.T) {
	app := &application{config: config{env: "test"}}

	code, body := healthcheckBody(t, app)
	if code != http.StatusOK {
		t.Fatalf("want 200 with no database configured, got %d", code)
	}
	if got := body["status"]; got != "available" {
		t.Fatalf("want status available, got %v", got)
	}
	if got := dependency(t, body, "database")["state"]; got != "absent" {
		t.Fatalf("want database absent, got %v", got)
	}
}

// The asymmetry that matters: a configured-but-unreachable broker is reported as
// down, yet readiness stays green because reads are served from projections.
func TestHealthcheckBrokerDownStaysReady(t *testing.T) {
	app := &application{config: config{env: "test", jetstreamDSN: "nats://127.0.0.1:1"}}

	code, body := healthcheckBody(t, app)
	if code != http.StatusOK {
		t.Fatalf("a broker outage must not fail readiness; got %d", code)
	}
	broker := dependency(t, body, "broker")
	if got := broker["state"]; got != "down" {
		t.Fatalf("want broker down, got %v", got)
	}
	if broker["error"] == nil {
		t.Fatal("want an explanation on a down broker, so the outage is visible")
	}
}

func TestHealthcheckBrokerAbsentWhenNotConfigured(t *testing.T) {
	app := &application{config: config{env: "test"}}

	_, body := healthcheckBody(t, app)
	if got := dependency(t, body, "broker")["state"]; got != "absent" {
		t.Fatalf("want broker absent when no DSN is set, got %v", got)
	}
}

// Projection lag is reported as "unknown" rather than a fabricated zero while no
// projection consumes subjects. A false zero would read as "fully caught up".
func TestHealthcheckReportsUnknownProjectionLag(t *testing.T) {
	app := &application{config: config{env: "test"}}

	_, body := healthcheckBody(t, app)
	projections := dependency(t, body, "projections")
	if got := projections["lag"]; got != "unknown" {
		t.Fatalf("want lag unknown, got %v", got)
	}
	if got := projections["dead_letters"]; got != float64(0) {
		t.Fatalf("want 0 dead letters with no writer, got %v", got)
	}
	if got := projections["registered"]; got != float64(0) {
		t.Fatalf("want 0 registered projections, got %v", got)
	}
	if got := projections["reason"]; got != "no projections registered" {
		t.Fatalf("unexpected reason: %v", got)
	}
}

// "Registered but consuming nothing" must be distinguishable from "nothing
// registered". Conflating them made the healthcheck claim no projection existed while
// one was wired up and running — caught by reading the live output, not by a test.
func TestHealthcheckDistinguishesRegisteredFromConsuming(t *testing.T) {
	app := &application{
		config:   config{env: "test"},
		eventing: &eventing{registered: 1, subjects: 0},
	}

	_, body := healthcheckBody(t, app)
	projections := dependency(t, body, "projections")
	if got := projections["registered"]; got != float64(1) {
		t.Fatalf("want 1 registered projection, got %v", got)
	}
	if got := projections["reason"]; got != "projections registered but consuming no subjects yet" {
		t.Fatalf("unexpected reason: %v", got)
	}
}
