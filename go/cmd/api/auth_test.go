package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"nathejk.dk/internal/ratelimit"
)

// recordingSender captures sent messages so tests can assert whether a PIN was
// actually dispatched.
type recordingSender struct {
	mu   sync.Mutex
	sent []sentMessage
}

type sentMessage struct {
	to      string
	message string
}

func (s *recordingSender) Send(_ context.Context, to, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, sentMessage{to: to, message: message})
	return nil
}

func (s *recordingSender) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sent)
}

func postJSON(t *testing.T, url, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func bodyMessage(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return payload.Message
}

func TestRequestPin_RecognizedSendsPIN(t *testing.T) {
	app := newTestApp(t)
	rec := &recordingSender{}
	app.sms = rec
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	// "30000001" normalizes to "+4530000001" which the mock recognizes.
	resp := postJSON(t, srv.URL+"/api/auth/request-pin", `{"phone":"30000001"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if msg := bodyMessage(t, resp); msg != antiEnumerationMessage {
		t.Fatalf("message = %q, want anti-enumeration message", msg)
	}
	if rec.count() != 1 {
		t.Fatalf("sent count = %d, want 1", rec.count())
	}
	if rec.sent[0].to != "+4530000001" {
		t.Errorf("sent to = %q, want +4530000001", rec.sent[0].to)
	}
}

func TestRequestPin_UnrecognizedIsIndistinguishable(t *testing.T) {
	app := newTestApp(t)
	rec := &recordingSender{}
	app.sms = rec
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp := postJSON(t, srv.URL+"/api/auth/request-pin", `{"phone":"+4599999999"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if msg := bodyMessage(t, resp); msg != antiEnumerationMessage {
		t.Fatalf("message = %q, want anti-enumeration message", msg)
	}
	if rec.count() != 0 {
		t.Fatalf("sent count = %d, want 0 (no PIN for unknown number)", rec.count())
	}
}

func TestRequestPin_InvalidPhoneIs400(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp := postJSON(t, srv.URL+"/api/auth/request-pin", `{"phone":"abc"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestRequestPin_RateLimited(t *testing.T) {
	app := newTestApp(t)
	app.requestPinLimiter = ratelimit.New(1, time.Minute)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	first := postJSON(t, srv.URL+"/api/auth/request-pin", `{"phone":"30000001"}`)
	io.Copy(io.Discard, first.Body)
	first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first status = %d, want 200", first.StatusCode)
	}

	second := postJSON(t, srv.URL+"/api/auth/request-pin", `{"phone":"30000001"}`)
	io.Copy(io.Discard, second.Body)
	second.Body.Close()
	if second.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429", second.StatusCode)
	}
}

func hasSessionCookie(resp *http.Response) bool {
	for _, c := range resp.Cookies() {
		if c.Name == "hej_session" && c.Value != "" {
			return true
		}
	}
	return false
}

func TestVerifyPin_SuccessSetsSession(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	// Issue a PIN directly so the test knows the code.
	code, err := app.pins.Issue("+4530000001")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	resp := postJSON(t, srv.URL+"/api/auth/verify", `{"phone":"30000001","pin":"`+code+`"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !hasSessionCookie(resp) {
		t.Fatal("expected a session cookie to be set")
	}

	var id struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&id); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if id.Role != "spejder" {
		t.Errorf("role = %q, want spejder", id.Role)
	}
	if id.UserID == "" {
		t.Error("user_id must not be empty")
	}
}

func TestVerifyPin_WrongPINIs401NoSession(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	if _, err := app.pins.Issue("+4530000001"); err != nil {
		t.Fatalf("issue: %v", err)
	}

	resp := postJSON(t, srv.URL+"/api/auth/verify", `{"phone":"30000001","pin":"000000"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	if hasSessionCookie(resp) {
		t.Fatal("no session cookie should be set on failure")
	}
}

func TestVerifyPin_NoPINForNumberIs401(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	// No PIN ever issued for this recognized number.
	resp := postJSON(t, srv.URL+"/api/auth/verify", `{"phone":"30000002","pin":"123456"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestMe_RequiresAuth(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/me")
	if err != nil {
		t.Fatalf("GET /api/me: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestMe_WithSession(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	code, err := app.pins.Issue("+4530000001")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	verify := postJSON(t, srv.URL+"/api/auth/verify", `{"phone":"30000001","pin":"`+code+`"}`)
	cookies := verify.Cookies()
	io.Copy(io.Discard, verify.Body)
	verify.Body.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/me", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/me: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var id struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&id); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if id.Role != "spejder" || id.UserID == "" {
		t.Fatalf("identity = %+v, want role spejder + non-empty id", id)
	}
}

func TestLogout_ClearsCookie(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp := postJSON(t, srv.URL+"/api/auth/logout", ``)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	cleared := false
	for _, c := range resp.Cookies() {
		if c.Name == "hej_session" && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("expected the session cookie to be cleared (MaxAge < 0)")
	}
}
