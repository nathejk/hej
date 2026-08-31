package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jrgensen/cqrs/cqrstest"
	"github.com/nathejk/shared-go/messages"

	"nathejk.dk/internal/data"
	"nathejk.dk/internal/ratelimit"
	"nathejk.dk/internal/scans"
	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/person"
)

// The mock directory's spejder, whose guardian number is +4520000001 — so "01" are the two
// digits a member would type.
const (
	confirmPhone      = "30000001"
	confirmNormalized = "+4530000001"
	confirmDigits     = "01"
)

// confirmTestApp is an app with a test publisher, a fixed year, and a person projection
// standing in for the database.
func confirmTestApp(t *testing.T, pub *cqrstest.Publisher, p person.Person) *application {
	t.Helper()
	app := newTestApp(t)
	app.config.eventYear = "2026"
	app.commands = commandsWithPublisher(t, pub)
	app.confirmLimiter = ratelimit.New(100, time.Minute)
	app.models = data.NewModels(
		users.NewMockDirectory(),
		scans.NewMockSource(),
		nil,
		&stubPeople{p: p, found: true},
	)
	return app
}

// unverifiedSpejder is the state the confirmation step exists for: a guardian number on
// file, not verified, not started.
func unverifiedSpejder() person.Person {
	guardian := "+4520000001"
	return person.Person{PersonID: "mock-spejder-1", PhoneParent: &guardian}
}

func TestConfirmProfile_RequiresAuth(t *testing.T) {
	app := confirmTestApp(t, &cqrstest.Publisher{}, unverifiedSpejder())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp := postJSON(t, srv.URL+"/api/me/profile/confirm", `{"digits":"01","acknowledged":true}`)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestConfirmProfile_PublishesVerification(t *testing.T) {
	pub := &cqrstest.Publisher{}
	app := confirmTestApp(t, pub, unverifiedSpejder())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, confirmPhone, confirmNormalized)
	resp := postJSONWithCookies(t, srv.URL+"/api/me/profile/confirm",
		`{"digits":"`+confirmDigits+`","acknowledged":true}`, cookies)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}

	if len(pub.Messages) != 1 {
		t.Fatalf("published %d events, want 1", len(pub.Messages))
	}
	// Decoded rather than inspected as a struct, so the assertions run through the same
	// JSON round-trip a real consumer does and can catch a wrong field tag.
	var body messages.NathejkMemberVerified
	if err := pub.Messages[0].Body(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.MemberID != "mock-spejder-1" {
		t.Errorf("memberId = %q, want the session's user", body.MemberID)
	}
	// The acknowledged number is the substance of the event: without it, a later guardian
	// number change could not invalidate this verification.
	if body.PhoneParentAcknowledged != "+4520000001" {
		t.Errorf("phoneParentAcknowledged = %q, want the guardian number on file", body.PhoneParentAcknowledged)
	}
	// Populated even though nothing reads it yet — the log is append-only, so a field left empty
	// today cannot be filled in for these events later (task 148 is what will read it). On this
	// path the two are equal: the member confirmed the number we hold.
	if body.PhoneParentRegistered != "+4520000001" {
		t.Errorf("phoneParentRegistered = %q, want the register's value", body.PhoneParentRegistered)
	}
	if body.VerifiedAt.IsZero() {
		t.Error("verifiedAt must be set by the publisher, not left for delivery time")
	}
}

// The digits are compared as digits: what the member sees is grouped for reading, and a
// stray space must not fail a correct answer. The check is meant to catch a member who does
// not know the number, not one who typed it with a space.
func TestConfirmProfile_AcceptsSpacedDigits(t *testing.T) {
	app := confirmTestApp(t, &cqrstest.Publisher{}, unverifiedSpejder())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, confirmPhone, confirmNormalized)
	resp := postJSONWithCookies(t, srv.URL+"/api/me/profile/confirm",
		`{"digits":" 0 1 ","acknowledged":true}`, cookies)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
}

func TestConfirmProfile_WrongDigitsIs400AndPublishesNothing(t *testing.T) {
	pub := &cqrstest.Publisher{}
	app := confirmTestApp(t, pub, unverifiedSpejder())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, confirmPhone, confirmNormalized)
	resp := postJSONWithCookies(t, srv.URL+"/api/me/profile/confirm",
		`{"digits":"99","acknowledged":true}`, cookies)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if len(pub.Messages) != 0 {
		t.Errorf("published %d events on a wrong answer, want 0", len(pub.Messages))
	}
}

// The acknowledgement is the substance of the step — the digits only establish that the
// member looked. Consent must never be inferred from the fact that a POST arrived.
func TestConfirmProfile_MissingAcknowledgementIsRejected(t *testing.T) {
	pub := &cqrstest.Publisher{}
	app := confirmTestApp(t, pub, unverifiedSpejder())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, confirmPhone, confirmNormalized)
	resp := postJSONWithCookies(t, srv.URL+"/api/me/profile/confirm",
		`{"digits":"`+confirmDigits+`","acknowledged":false}`, cookies)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if len(pub.Messages) != 0 {
		t.Errorf("published %d events without an acknowledgement, want 0", len(pub.Messages))
	}
}

// 409 covers every reason there is nothing to confirm. Here: already verified.
func TestConfirmProfile_AlreadyVerifiedIs409(t *testing.T) {
	guardian := "+4520000001"
	at := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	app := confirmTestApp(t, &cqrstest.Publisher{}, person.Person{
		PersonID:          "mock-spejder-1",
		PhoneParent:       &guardian,
		AcknowledgedPhone: &guardian,
		VerifiedAt:        &at,
	})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, confirmPhone, confirmNormalized)
	resp := postJSONWithCookies(t, srv.URL+"/api/me/profile/confirm",
		`{"digits":"`+confirmDigits+`","acknowledged":true}`, cookies)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
}

// A member with no guardian number on file (bandit, crew, gøgler) can never be required to
// confirm one — the spejder-only rule, enforced here rather than only in the client.
func TestConfirmProfile_NoGuardianNumberIs409(t *testing.T) {
	app := confirmTestApp(t, &cqrstest.Publisher{}, person.Person{PersonID: "mock-bandit-1"})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000002", "+4530000002")
	resp := postJSONWithCookies(t, srv.URL+"/api/me/profile/confirm",
		`{"digits":"01","acknowledged":true}`, cookies)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
}

// A confirmation the log never saw did not happen, so it must not answer 204: the member
// would stop being asked and no organizer would ever see the flag.
func TestConfirmProfile_NoPublisherIs503(t *testing.T) {
	app := confirmTestApp(t, &cqrstest.Publisher{}, unverifiedSpejder())
	app.commands = commandsWithNoPublisher()
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, confirmPhone, confirmNormalized)
	resp := postJSONWithCookies(t, srv.URL+"/api/me/profile/confirm",
		`{"digits":"`+confirmDigits+`","acknowledged":true}`, cookies)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}

func TestConfirmProfile_RateLimited(t *testing.T) {
	app := confirmTestApp(t, &cqrstest.Publisher{}, unverifiedSpejder())
	app.confirmLimiter = ratelimit.New(1, time.Minute)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, confirmPhone, confirmNormalized)
	first := postJSONWithCookies(t, srv.URL+"/api/me/profile/confirm",
		`{"digits":"99","acknowledged":true}`, cookies)
	io.Copy(io.Discard, first.Body)
	first.Body.Close()

	second := postJSONWithCookies(t, srv.URL+"/api/me/profile/confirm",
		`{"digits":"99","acknowledged":true}`, cookies)
	io.Copy(io.Discard, second.Body)
	second.Body.Close()
	if second.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second attempt status = %d, want 429", second.StatusCode)
	}
}

func TestLastTwoDigitsMatch(t *testing.T) {
	for _, tc := range []struct {
		number, typed string
		want          bool
	}{
		{"+4520000001", "01", true},
		{"20 00 00 01", "01", true},
		{"+4520000001", " 0 1 ", true},
		{"+4520000001", "1", false},
		{"+4520000001", "10", false},
		{"+4520000001", "001", false},
		// An empty or too-short number can never match: answering "correct" for a number
		// that is not there would record a verification of nothing.
		{"", "01", false},
		{"1", "01", false},
	} {
		if got := lastTwoDigitsMatch(tc.number, tc.typed); got != tc.want {
			t.Errorf("lastTwoDigitsMatch(%q, %q) = %v, want %v", tc.number, tc.typed, got, tc.want)
		}
	}
}
