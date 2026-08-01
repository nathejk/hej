package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPushPublicKey(t *testing.T) {
	app := newTestApp(t)
	app.config.vapidPublicKey = "test-public-key"
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/push/public-key")
	if err != nil {
		t.Fatalf("GET public-key: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var body struct {
		PublicKey string `json:"public_key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.PublicKey != "test-public-key" {
		t.Fatalf("public_key = %q, want test-public-key", body.PublicKey)
	}
}
