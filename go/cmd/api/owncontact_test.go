package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jrgensen/cqrs/cqrstest"

	"nathejk.dk/internal/data"
	"nathejk.dk/internal/scans"
	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/person"
)

// A contact number that is really the member's own is treated as no number at all
// (PRD 015, task 229).

// stubDirectory is a users.Directory holding exactly one user, so a test can put a record on file
// that the mock directory does not have — here, the mistake this rule exists for.
type stubDirectory struct{ u users.User }

func (d stubDirectory) LookupAll(phone string) []users.User {
	if phone == d.u.Phone {
		return []users.User{d.u}
	}
	return nil
}

func (d stubDirectory) Lookup(phone string) (users.User, bool) {
	if phone == d.u.Phone {
		return d.u, true
	}
	return users.User{}, false
}

func (d stubDirectory) Get(id string) (users.User, bool) {
	if id == d.u.ID {
		return d.u, true
	}
	return users.User{}, false
}

// profileAppFor builds an app whose profile read answers for exactly this user.
func profileAppFor(t *testing.T, u users.User) *application {
	t.Helper()
	app := newTestApp(t)
	app.config.eventYear = "2026"
	app.models = data.NewModels(stubDirectory{u: u}, scans.NewMockSource(), nil, &stubPeople{})
	return app
}

func TestContactNumberForOwner(t *testing.T) {
	ptr := func(s string) *string { return &s }

	for _, tc := range []struct {
		name    string
		contact *string
		own     string
		want    *string
	}{
		// The populations with no contact number at all must stay nil, or the client starts
		// showing a bandit a row asking for something they are not supposed to have.
		{"no contact number at all", nil, "+4530000001", nil},
		{"expected but missing", ptr(""), "+4530000001", ptr("")},
		{"a genuinely different number", ptr("+4520000001"), "+4530000001", ptr("+4520000001")},

		// The collision, in the three styles the register actually contains.
		{"identical", ptr("+4530000001"), "+4530000001", ptr("")},
		{"spaced and unprefixed", ptr("30 00 00 01"), "+4530000001", ptr("")},
		{"double-zero prefix", ptr("004530000001"), "+4530000001", ptr("")},

		// Two blanks are not "the same number": a member whose own number is missing from the
		// register must not have their contact number blanked as a side effect.
		{"own number missing", ptr("+4520000001"), "", ptr("+4520000001")},

		// Neither will normalize, so they are compared as written. Equal here means equal.
		{"unparseable and equal", ptr("ring til mor"), "ring til mor", ptr("")},
		{"unparseable and different", ptr("ring til mor"), "ring til far", ptr("ring til mor")},
	} {
		got := contactNumberForOwner(tc.contact, tc.own)
		switch {
		case got == nil && tc.want == nil:
		case got == nil || tc.want == nil:
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		case *got != *tc.want:
			t.Errorf("%s: got %q, want %q", tc.name, *got, *tc.want)
		}
	}
}

// The end-to-end shape: the member sees "expected but not registered", never their own number
// dressed up as their emergency contact.
func TestShowProfile_BlanksAContactNumberEqualToTheirOwn(t *testing.T) {
	registeredAsContact := "30 00 00 01"
	app := profileAppFor(t, users.User{
		ID:          "mock-spejder-1",
		Role:        users.RoleSpejder,
		Name:        "Sofie Spejder",
		Phone:       "+4530000001",
		PhoneParent: &registeredAsContact,
	})

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	resp := getWithCookies(t, srv.URL+"/api/me/profile", cookies)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", resp.StatusCode, body)
	}

	var out struct {
		PhoneParent *string `json:"phone_parent"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// "" and not null: null would tell a spejder they are not supposed to have a contact number,
	// and would switch off the question for exactly the member who needs it.
	if out.PhoneParent == nil {
		t.Fatal(`phone_parent must be "", not null: the member is expected to have one`)
	}
	if *out.PhoneParent != "" {
		t.Errorf("phone_parent = %q, want it blanked", *out.PhoneParent)
	}
	// And the number must not survive anywhere in the body, in any formatting.
	if strings.Contains(string(body), "30 00 00 01") {
		t.Errorf("the colliding number leaked: %s", body)
	}
}

// A member with no contact number is untouched: the rule must not turn a bandit's nil into "".
func TestShowProfile_OwnNumberRuleIsANoOpForBandits(t *testing.T) {
	app := profileAppFor(t, users.User{
		ID:          "mock-bandit-1",
		Role:        users.RoleBandit,
		Name:        "Bo Bandit",
		Phone:       "+4530000002",
		PhoneParent: nil,
	})

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000002", "+4530000002")
	resp := getWithCookies(t, srv.URL+"/api/me/profile", cookies)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var out struct {
		PhoneParent *string `json:"phone_parent"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.PhoneParent != nil {
		t.Errorf("phone_parent = %q, want null for a population with no contact number", *out.PhoneParent)
	}
}

// A real contact number is left alone. Worth asserting alongside the blanking: a rule that blanked
// everything would pass every test above.
func TestShowProfile_KeepsAGenuineContactNumber(t *testing.T) {
	contact := "+4520000001"
	app := profileAppFor(t, users.User{
		ID:          "mock-spejder-1",
		Role:        users.RoleSpejder,
		Name:        "Sofie Spejder",
		Phone:       "+4530000001",
		PhoneParent: &contact,
	})

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	resp := getWithCookies(t, srv.URL+"/api/me/profile", cookies)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var out struct {
		PhoneParent *string `json:"phone_parent"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.PhoneParent == nil || *out.PhoneParent != contact {
		t.Errorf("phone_parent = %v, want the registered contact number", out.PhoneParent)
	}
}

// After the start, the app shows the number **check-in** recorded, not the register's. A member
// reading a stale register value off their own screen would believe we will call a phone nobody at
// the counter ever wrote down (PRD 015, task 230).
func TestShowProfile_PrefersTheNumberCheckInRecorded(t *testing.T) {
	register := "+4520000001"
	checkIn := "+4544556677"

	app := profileAppFor(t, users.User{
		ID:          "mock-spejder-1",
		Role:        users.RoleSpejder,
		Name:        "Sofie Spejder",
		Phone:       "+4530000001",
		PhoneParent: &register,
	})
	// The projection is the only place the check-in number lives, so it has to answer here.
	app.models = data.NewModels(
		stubDirectory{u: users.User{
			ID: "mock-spejder-1", Role: users.RoleSpejder, Name: "Sofie Spejder",
			Phone: "+4530000001", PhoneParent: &register,
		}},
		scans.NewMockSource(),
		nil,
		&stubPeople{found: true, p: person.Person{
			PersonID:            "mock-spejder-1",
			Year:                "2026",
			AppRole:             person.RoleSpejder,
			Phone:               "+4530000001",
			PhoneParent:         &register,
			StartedPhoneContact: &checkIn,
			MemberStatus:        person.MemberStatusRacing,
		}},
	)

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	resp := getWithCookies(t, srv.URL+"/api/me/profile", cookies)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var out struct {
		PhoneParent *string `json:"phone_parent"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.PhoneParent == nil || *out.PhoneParent != checkIn {
		t.Errorf("phone_parent = %v, want the number check-in recorded (%q)", out.PhoneParent, checkIn)
	}
	if strings.Contains(string(body), register) {
		t.Errorf("the register's superseded number must not be shown: %s", body)
	}
}

// A blanked number cannot be confirmed. Without this, a member could recall the two digits of their
// own number — which they know perfectly — and be fast-tracked through check-in on a record that
// cannot serve its purpose.
func TestConfirmProfile_CannotConfirmANumberEqualToTheirOwn(t *testing.T) {
	own := confirmNormalized
	pub := &cqrstest.Publisher{}
	app := confirmTestApp(t, pub, person.Person{
		PersonID:    "mock-spejder-1",
		Year:        "2026",
		AppRole:     person.RoleSpejder,
		Phone:       own,
		PhoneParent: &own,
	})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, confirmPhone, confirmNormalized)
	pub.Reset()

	// The last two digits of their own number, which the register wrongly holds as the contact.
	resp := postConfirmDigits(t, srv, cookies, own[len(own)-2:])
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: a member's own number is not a contact number", resp.StatusCode)
	}
	if len(pub.Messages) != 0 {
		t.Errorf("published %d events, want 0 — nothing was verified", len(pub.Messages))
	}
}
