package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jrgensen/cqrs/cqrstest"
	"github.com/nathejk/shared-go/messages"

	"nathejk.dk/nathejk/table/person"
)

// POST /api/me/profile/skip — the recorded give-up (PRD 015, task 228).

// skipTestApp is confirmTestApp for a member who is *expected* to reach the check: a contact
// number on file, an own number the PIN can prove, and an app role that publishes.
func skipTestApp(t *testing.T, pub *cqrstest.Publisher) *application {
	t.Helper()
	guardian := "+4520000001"
	return confirmTestApp(t, pub, person.Person{
		PersonID:    "mock-spejder-1",
		Year:        "2026",
		AppRole:     person.RoleSpejder,
		Phone:       confirmNormalized,
		PhoneParent: &guardian,
	})
}

func postSkip(t *testing.T, srv *httptest.Server, cookies []*http.Cookie) *http.Response {
	t.Helper()
	return postJSONWithCookies(t, srv.URL+"/api/me/profile/skip", `{}`, cookies)
}

func TestSkipProfileCheck_RequiresAuth(t *testing.T) {
	app := skipTestApp(t, &cqrstest.Publisher{})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp := postJSON(t, srv.URL+"/api/me/profile/skip", `{}`)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

// The event says what is true: the member's own number is verified, the contact number is not.
func TestSkipProfileCheck_PublishesOwnPhoneWithoutContact(t *testing.T) {
	pub := &cqrstest.Publisher{}
	app := skipTestApp(t, pub)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, confirmPhone, confirmNormalized)
	// The login itself publishes an own-phone verification (task 226); this test is about what the
	// skip adds, so start counting from here.
	pub.Reset()

	resp := postSkip(t, srv, cookies)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}

	if len(pub.Messages) != 1 {
		t.Fatalf("published %d events, want 1", len(pub.Messages))
	}
	if got := pub.Subjects()[0]; got != "NATHEJK.2026.spejder.mock-spejder-1.verified" {
		t.Errorf("subject = %q", got)
	}

	var body messages.NathejkMemberVerified
	if err := pub.Messages[0].Body(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Phone != confirmNormalized {
		t.Errorf("phone = %q, want the member's own number", body.Phone)
	}
	if body.PhoneContact != "" {
		t.Errorf("phoneContact = %q, want it empty: that is the whole point", body.PhoneContact)
	}
	if body.VerifiedAt.IsZero() {
		t.Error("verifiedAt must be set by the publisher")
	}
}

// The give-up outcome must fire even when the member's own number was already recorded. The
// once-per-number suppression is a login-side optimisation; applying it here would produce silence
// for exactly the members this PRD is about.
func TestSkipProfileCheck_PublishesEvenWhenOwnPhoneAlreadyVerified(t *testing.T) {
	at := time.Date(2026, 8, 30, 19, 0, 0, 0, time.UTC)
	proven := confirmNormalized
	guardian := "+4520000001"
	pub := &cqrstest.Publisher{}
	app := confirmTestApp(t, pub, person.Person{
		PersonID:        "mock-spejder-1",
		Year:            "2026",
		AppRole:         person.RoleSpejder,
		Phone:           confirmNormalized,
		PhoneParent:     &guardian,
		PhoneVerifiedAt: &at,
		VerifiedPhone:   &proven,
	})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, confirmPhone, confirmNormalized)
	// The login published nothing, because this number was already recorded — which is the state
	// that makes this test meaningful rather than a duplicate of the one above.
	if len(pub.Messages) != 0 {
		t.Fatalf("login published %d events for an already-verified number, want 0", len(pub.Messages))
	}

	resp := postSkip(t, srv, cookies)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if len(pub.Messages) != 1 {
		t.Fatalf("published %d events, want 1", len(pub.Messages))
	}
}

// A double submit is one outcome. The client cannot always tell a dropped response from a failure,
// so it will retry — and two events would read as two members' worth of signal.
func TestSkipProfileCheck_DoubleSubmitPublishesOnce(t *testing.T) {
	pub := &cqrstest.Publisher{}
	app := skipTestApp(t, pub)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, confirmPhone, confirmNormalized)
	pub.Reset()

	for i := 0; i < 3; i++ {
		resp := postSkip(t, srv, cookies)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("submit %d: status = %d, want 204", i+1, resp.StatusCode)
		}
	}

	if len(pub.Messages) != 1 {
		t.Errorf("published %d events for three submits, want 1", len(pub.Messages))
	}
}

// A population with no contact number never sees this step, so a skip from one is a client bug
// rather than a fact about the member — accepted quietly, and nothing is published.
func TestSkipProfileCheck_PublishesNothingWithoutAContactNumber(t *testing.T) {
	pub := &cqrstest.Publisher{}
	app := confirmTestApp(t, pub, person.Person{
		PersonID: "mock-bandit-1",
		Year:     "2026",
		AppRole:  person.RoleBandit,
		Phone:    "+4530000002",
	})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000002", "+4530000002")
	pub.Reset()

	resp := postSkip(t, srv, cookies)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	if len(pub.Messages) != 0 {
		t.Errorf("published %d events for a member with no contact number, want 0", len(pub.Messages))
	}
}

// Unlike the login-side publish, a broken broker is reported here: this publish *is* the act being
// recorded. The client still lets the member into the app, which is its own decision to make.
func TestSkipProfileCheck_NoPublisherIs503(t *testing.T) {
	app := skipTestApp(t, &cqrstest.Publisher{})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, confirmPhone, confirmNormalized)
	app.commands = commandsWithNoPublisher()

	resp := postSkip(t, srv, cookies)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
	// Plain language, not a stack trace: this reaches a 13-year-old's screen.
	if !strings.Contains(string(body), "kan ikke gemmes lige nu") {
		t.Errorf("body = %s", body)
	}
}
