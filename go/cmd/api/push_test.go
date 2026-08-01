package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// authedCookies logs in as the given recognized phone (issuing a PIN directly)
// and returns the resulting session cookies.
func authedCookies(t *testing.T, app *application, srv *httptest.Server, phone, normalized string) []*http.Cookie {
	t.Helper()
	code, err := app.pins.Issue(normalized)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	resp := postJSON(t, srv.URL+"/api/auth/verify", `{"phone":"`+phone+`","pin":"`+code+`"}`)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("verify status = %d, want 200", resp.StatusCode)
	}
	return resp.Cookies()
}

func postJSONWithCookies(t *testing.T, url, body string, cookies []*http.Cookie) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

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

func TestPushSubscription_RequiresAuth(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp := postJSON(t, srv.URL+"/api/push/subscription", `{"endpoint":"https://x","keys":{"p256dh":"a","auth":"b"}}`)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestPushSubscription_StoresIdempotently(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	body := `{"endpoint":"https://push.example/abc","keys":{"p256dh":"key","auth":"auth"}}`

	for i := 0; i < 2; i++ {
		resp := postJSONWithCookies(t, srv.URL+"/api/push/subscription", body, cookies)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("attempt %d status = %d, want 201", i, resp.StatusCode)
		}
	}

	if n := len(app.pushStore.All()); n != 1 {
		t.Fatalf("stored subscriptions = %d, want 1 (idempotent)", n)
	}
}

func TestPushSubscription_MissingFieldsIs400(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	resp := postJSONWithCookies(t, srv.URL+"/api/push/subscription", `{"endpoint":"https://x"}`, cookies)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}
