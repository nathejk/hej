package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	bff "nathejk.dk/cmd/api/app"
	"nathejk.dk/internal/commands"
	"nathejk.dk/internal/data"
	"nathejk.dk/internal/pin"
	"nathejk.dk/internal/push"
	"nathejk.dk/internal/ratelimit"
	"nathejk.dk/internal/scans"
	"nathejk.dk/internal/session"
	"nathejk.dk/internal/sms"
	"nathejk.dk/internal/users"
)

func newTestApp(t *testing.T) *application {
	t.Helper()

	webRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(webRoot, "index.html"), []byte("<!doctype html><title>spa</title>"), 0o600); err != nil {
		t.Fatalf("writing test index.html: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &application{
		JsonApi:  bff.JsonApi{Logger: logger},
		config:   config{env: "testing", webRoot: webRoot},
		models:   data.NewModels(users.NewMockDirectory(), scans.NewMockSource()),
		commands: commands.New(),

		pins:              pin.NewStore(),
		sms:               sms.LogSender{Logger: logger},
		sessions:          session.NewManager([]byte("test-secret"), time.Hour, false),
		requestPinLimiter: ratelimit.New(100, time.Minute),
		pushStore:         push.NewMemoryStore(),
	}
}

func TestHealthcheck(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/healthcheck")
	if err != nil {
		t.Fatalf("GET /api/healthcheck: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body struct {
		Status     string `json:"status"`
		SystemInfo struct {
			Environment string `json:"environment"`
		} `json:"system_info"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	if body.Status != "available" {
		t.Errorf("status = %q, want %q", body.Status, "available")
	}
	if body.SystemInfo.Environment != "testing" {
		t.Errorf("environment = %q, want %q", body.SystemInfo.Environment, "testing")
	}
}

func TestUnknownAPIReturnsJSON404(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/does-not-exist")
	if err != nil {
		t.Fatalf("GET unknown api: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}
}

func TestSPAFallbackServesIndex(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/some/client/route")
	if err != nil {
		t.Fatalf("GET spa route: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	data, _ := io.ReadAll(resp.Body)
	if len(data) == 0 {
		t.Error("expected index.html contents, got empty body")
	}
}
