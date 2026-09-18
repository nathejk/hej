package sms

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewSenderPicksProviderFromDSN(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, tc := range []struct {
		dsn     string
		wantLog bool
		wantErr bool
	}{
		{dsn: "", wantLog: true},
		{dsn: "log://", wantLog: true},
		{dsn: "cpsms://key@api.cpsms.dk"},
		{dsn: "cpsms://api.cpsms.dk", wantErr: true}, // no key
		{dsn: "carrierpigeon://x", wantErr: true},    // unknown provider
	} {
		got, err := NewSender(tc.dsn, logger)
		if tc.wantErr {
			if err == nil {
				t.Errorf("NewSender(%q): want error, got %T", tc.dsn, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("NewSender(%q): %v", tc.dsn, err)
			continue
		}
		if _, isLog := got.(LogSender); isLog != tc.wantLog {
			t.Errorf("NewSender(%q) = %T, wantLog=%v", tc.dsn, got, tc.wantLog)
		}
	}
}

// The BFF holds numbers as "+4530000001"; CPSMS wants bare digits with the country
// code. Getting this wrong means every message is silently addressed to nobody.
func TestCpsmsSendsDigitsOnlyRecipient(t *testing.T) {
	var got struct {
		To      string `json:"to"`
		From    string `json:"from"`
		Message string `json:"message"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Basic secret-key" {
			t.Errorf("Authorization = %q", auth)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode: %v", err)
		}
		io.WriteString(w, `{"success":[{"to":"4530000001","cost":25}]}`)
	}))
	defer srv.Close()

	s := senderTo(t, srv.URL, "")
	if err := s.Send(context.Background(), "+45 30 00 00 01", "PIN 123456"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got.To != "4530000001" {
		t.Errorf("to = %q, want 4530000001", got.To)
	}
	if got.From != defaultSenderName {
		t.Errorf("from = %q, want %q", got.From, defaultSenderName)
	}
	if got.Message != "PIN 123456" {
		t.Errorf("message = %q", got.Message)
	}
}

func TestCpsmsReportsProviderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"error":{"code":401,"message":"Unauthorized"}}`)
	}))
	defer srv.Close()

	err := senderTo(t, srv.URL, "").Send(context.Background(), "+4530000001", "hi")
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("err = %v, want a 401 provider error", err)
	}
}

// A 200 with no recipient accepted is a failure: reporting it as success would leave
// a user waiting for a PIN that was never sent.
func TestCpsmsEmptySuccessIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"success":[]}`)
	}))
	defer srv.Close()

	if err := senderTo(t, srv.URL, "").Send(context.Background(), "+4530000001", "hi"); err == nil {
		t.Fatal("want an error when no recipient was accepted")
	}
}

// senderTo points a CpsmsSender at a test server (which speaks http, not https).
func senderTo(t *testing.T, url, from string) *CpsmsSender {
	t.Helper()
	s, err := NewCpsms("placeholder", "secret-key", from)
	if err != nil {
		t.Fatalf("NewCpsms: %v", err)
	}
	s.apiURL = url + "/v2/send"
	return s
}
