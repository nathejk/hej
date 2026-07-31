package session

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager([]byte("test-secret"), time.Hour, false)
}

func TestIssueAndRead_RoundTrip(t *testing.T) {
	m := newTestManager(t)
	rr := httptest.NewRecorder()
	issued := m.Issue(rr, "user-1", "spejder")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rr.Result().Cookies() {
		req.AddCookie(c)
	}

	got, err := m.Read(req)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.UserID != issued.UserID || got.Role != issued.Role {
		t.Fatalf("session mismatch: got %+v, issued %+v", got, issued)
	}
}

func TestRead_NoCookie(t *testing.T) {
	m := newTestManager(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := m.Read(req)
	if !errors.Is(err, ErrNoSession) {
		t.Fatalf("err = %v, want ErrNoSession", err)
	}
}

func TestRead_TamperedCookie(t *testing.T) {
	m := newTestManager(t)
	rr := httptest.NewRecorder()
	m.Issue(rr, "user-1", "spejder")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rr.Result().Cookies() {
		req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value + "x"})
	}
	_, err := m.Read(req)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestRead_Expired(t *testing.T) {
	now := time.Now()
	m := NewManager([]byte("s"), time.Minute, false)
	m.now = func() time.Time { return now }

	rr := httptest.NewRecorder()
	m.Issue(rr, "user-1", "spejder")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rr.Result().Cookies() {
		req.AddCookie(c)
	}
	now = now.Add(2 * time.Minute)

	_, err := m.Read(req)
	if !errors.Is(err, ErrExpired) {
		t.Fatalf("err = %v, want ErrExpired", err)
	}
}

func TestRead_WrongSecret(t *testing.T) {
	rr := httptest.NewRecorder()
	NewManager([]byte("a"), time.Hour, false).Issue(rr, "u", "spejder")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rr.Result().Cookies() {
		req.AddCookie(c)
	}
	_, err := NewManager([]byte("b"), time.Hour, false).Read(req)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}
