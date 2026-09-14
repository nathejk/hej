package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

// The own-phone verification published at login (PRD 015, task 226).
//
// Every test here drives the *real* login endpoint rather than calling the helper directly,
// because the property being defended is "logging in records this", and a unit test of the helper
// would keep passing if the call site were deleted.

// loginTestApp is confirmTestApp with a name that says what these tests are about, plus the mock
// directory's phone numbers spelled out below.
func loginTestApp(t *testing.T, pub *cqrstest.Publisher, p person.Person) *application {
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

// login runs a full PIN login against the given app and returns the cookies it set. It fails the
// test on any response other than 200, so "the login worked" is the assertion rather than
// something the caller re-checks. Assertions about events read the publisher directly.
func login(t *testing.T, app *application, phone, normalized string) []*http.Cookie {
	t.Helper()
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	code, err := app.pins.Issue(normalized)
	if err != nil {
		t.Fatalf("issue pin: %v", err)
	}
	resp := postJSON(t, srv.URL+"/api/auth/verify", `{"phone":"`+phone+`","pin":"`+code+`"}`)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("verify status = %d, want 200", resp.StatusCode)
	}
	return resp.Cookies()
}

func TestLogin_PublishesOwnPhoneVerified(t *testing.T) {
	pub := &cqrstest.Publisher{}
	app := loginTestApp(t, pub, person.Person{
		PersonID: "mock-spejder-1",
		Year:     "2026",
		AppRole:  person.RoleSpejder,
	})
	login(t, app, confirmPhone, confirmNormalized)

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
	// The number the PIN was sent to, normalized — the same form the projection and the login
	// lookup compare against.
	if body.Phone != confirmNormalized {
		t.Errorf("phone = %q, want the number the PIN proved", body.Phone)
	}
	if body.VerifiedAt.IsZero() {
		t.Error("verifiedAt must be set by the publisher, not left for delivery time")
	}
	// The load-bearing absence: a login says nothing about the contact number, and an event that
	// named it could wipe a confirmation the member made last week.
	if body.PhoneContact != "" {
		t.Errorf("a login must not carry a contact number, got %q", body.PhoneContact)
	}
	if raw, ok := pub.Messages[0].RawBody().([]byte); ok {
		if strings.Contains(string(raw), "phoneContact") {
			t.Errorf("phoneContact must be absent from the encoded body: %s", raw)
		}
	}
}

// A bandit's own number is proven by exactly the same SMS challenge, so it is recorded too. The
// subject still reads `spejder`, which is a prefix rather than a population — see
// person.VerifiedSubject.
func TestLogin_PublishesForBandit(t *testing.T) {
	pub := &cqrstest.Publisher{}
	app := loginTestApp(t, pub, person.Person{
		PersonID: "mock-bandit-1",
		Year:     "2026",
		AppRole:  person.RoleBandit,
	})
	login(t, app, "30000002", "+4530000002")

	if len(pub.Messages) != 1 {
		t.Fatalf("published %d events, want 1", len(pub.Messages))
	}
	if got := pub.Subjects()[0]; got != "NATHEJK.2026.spejder.mock-bandit-1.verified" {
		t.Errorf("subject = %q", got)
	}
}

// Crew and gøglere have their own numbers verified through another route, so publishing here would
// restate a known fact — for the population that logs in most.
func TestLogin_PublishesNothingForCrewAndGoegler(t *testing.T) {
	for _, role := range []string{person.RoleCrew, person.RoleGoegler, person.RoleSamarit} {
		pub := &cqrstest.Publisher{}
		app := loginTestApp(t, pub, person.Person{
			PersonID: "mock-spejder-1",
			Year:     "2026",
			AppRole:  role,
		})
		login(t, app, confirmPhone, confirmNormalized)
		if len(pub.Messages) != 0 {
			t.Errorf("%s: published %d events, want 0", role, len(pub.Messages))
		}
	}
}

// Once per verified number, not once per login: a member logging in every morning restates
// nothing. The suppression compares the number, so this fixture is the state left behind by an
// earlier login.
func TestLogin_SuppressesWhenTheSameNumberIsAlreadyVerified(t *testing.T) {
	at := time.Date(2026, 8, 30, 19, 0, 0, 0, time.UTC)
	proven := confirmNormalized
	pub := &cqrstest.Publisher{}
	app := loginTestApp(t, pub, person.Person{
		PersonID:        "mock-spejder-1",
		Year:            "2026",
		AppRole:         person.RoleSpejder,
		PhoneVerifiedAt: &at,
		VerifiedPhone:   &proven,
	})
	login(t, app, confirmPhone, confirmNormalized)

	if len(pub.Messages) != 0 {
		t.Errorf("published %d events for an already-verified number, want 0", len(pub.Messages))
	}
}

// ...but a *different* number publishes again, and the later verification wins. Without this, a
// member who changed phones would keep the old number recorded forever, because the old
// verification suppressed the new one.
func TestLogin_PublishesWhenTheProvenNumberIsNew(t *testing.T) {
	at := time.Date(2026, 8, 30, 19, 0, 0, 0, time.UTC)
	old := "+4599999999"
	pub := &cqrstest.Publisher{}
	app := loginTestApp(t, pub, person.Person{
		PersonID:        "mock-spejder-1",
		Year:            "2026",
		AppRole:         person.RoleSpejder,
		PhoneVerifiedAt: &at,
		VerifiedPhone:   &old,
	})
	login(t, app, confirmPhone, confirmNormalized)

	if len(pub.Messages) != 1 {
		t.Fatalf("published %d events, want 1 for a newly proven number", len(pub.Messages))
	}
	var body messages.NathejkMemberVerified
	if err := pub.Messages[0].Body(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Phone != confirmNormalized {
		t.Errorf("phone = %q, want the number just proven", body.Phone)
	}
}

// The one that matters most: a broker outage must not stand between a member and the app. The
// publish is a by-product of logging in, not the act the member performed — so the session has to
// come out of it, not just a 200.
func TestLogin_SucceedsWhenThereIsNoBroker(t *testing.T) {
	app := loginTestApp(t, &cqrstest.Publisher{}, person.Person{
		PersonID: "mock-spejder-1",
		Year:     "2026",
		AppRole:  person.RoleSpejder,
	})
	app.commands = commandsWithNoPublisher()
	if cookies := login(t, app, confirmPhone, confirmNormalized); len(cookies) == 0 {
		t.Error("want a session cookie even though the event could not be published")
	}
}

// ...and the same when the broker is there but refuses the publish.
func TestLogin_SucceedsWhenThePublishFails(t *testing.T) {
	app := loginTestApp(t, &cqrstest.Publisher{Err: errors.New("broker said no")}, person.Person{
		PersonID: "mock-spejder-1",
		Year:     "2026",
		AppRole:  person.RoleSpejder,
	})
	if cookies := login(t, app, confirmPhone, confirmNormalized); len(cookies) == 0 {
		t.Error("want a session cookie even though the publish failed")
	}
}

// A member with no row in the projection yet must still be able to log in: the user directory can
// answer when the projection cannot, and login must not depend on the slower of the two.
func TestLogin_SucceedsWithoutAPersonRow(t *testing.T) {
	pub := &cqrstest.Publisher{}
	app := newTestApp(t)
	app.config.eventYear = "2026"
	app.commands = commandsWithPublisher(t, pub)
	app.models = data.NewModels(
		users.NewMockDirectory(),
		scans.NewMockSource(),
		nil,
		&stubPeople{found: false},
	)
	if cookies := login(t, app, confirmPhone, confirmNormalized); len(cookies) == 0 {
		t.Error("want a session cookie for a member with no projection row")
	}

	if len(pub.Messages) != 0 {
		t.Errorf("published %d events with no person row, want 0", len(pub.Messages))
	}
}
