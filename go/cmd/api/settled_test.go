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

// Once a member has started, the contact number is settled and the app stops changing it
// (PRD 015, task 231).

// startedSpejder is a member who has come past the counter. `MemberStatus` is what says so.
func startedSpejder() person.Person {
	contact := "+4520000001"
	return person.Person{
		PersonID:     "mock-spejder-1",
		Year:         "2026",
		AppRole:      person.RoleSpejder,
		Phone:        confirmNormalized,
		PhoneParent:  &contact,
		MemberStatus: person.MemberStatusRacing,
	}
}

func postGuardian(t *testing.T, srv *httptest.Server, cookies []*http.Cookie, number string) *http.Response {
	t.Helper()
	return postJSONWithCookies(t, srv.URL+"/api/me/profile/guardian",
		`{"phone":"`+number+`","acknowledged":true}`, cookies)
}

// The refusal. This reverses part of task 148 on purpose — see the comment at the guard.
func TestSetGuardian_RefusedOnceTheMemberHasStarted(t *testing.T) {
	pub := &cqrstest.Publisher{}
	app := confirmTestApp(t, pub, startedSpejder())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, confirmPhone, confirmNormalized)
	pub.Reset()

	resp := postGuardian(t, srv, cookies, "22334455")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// 409, not 403 or 400: nothing about the caller is wrong, the act simply no longer applies.
	// The PWA needs it distinguishable from a validation failure, which is a 400 here.
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", resp.StatusCode, body)
	}
	// Says what to do instead. A dead end would send the member to the nødtelefon, or nowhere.
	if !strings.Contains(string(body), "leder") {
		t.Errorf("the refusal should point somewhere useful: %s", body)
	}
	if len(pub.Messages) != 0 {
		t.Errorf("published %d events for a refused change, want 0", len(pub.Messages))
	}
}

// ...and a member who has not started can still correct the number, exactly as task 148 shipped it.
func TestSetGuardian_StillWorksBeforeTheStart(t *testing.T) {
	pub := &cqrstest.Publisher{}
	app := confirmTestApp(t, pub, unverifiedSpejder())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, confirmPhone, confirmNormalized)
	pub.Reset()

	resp := postGuardian(t, srv, cookies, "22334455")
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 before the start", resp.StatusCode)
	}
	if len(pub.Messages) != 1 {
		t.Errorf("published %d events, want 1", len(pub.Messages))
	}
}

// /confirm's behaviour for a started member is asserted rather than assumed from PRD 005: it is the
// half of this rule that was already true, and "already true" is how a regression hides.
func TestConfirmProfile_StillRefusedForAStartedMember(t *testing.T) {
	app := confirmTestApp(t, &cqrstest.Publisher{}, startedSpejder())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, confirmPhone, confirmNormalized)
	resp := postConfirmDigits(t, srv, cookies, confirmDigits)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409 for a member who has started", resp.StatusCode)
	}
}

// The client cannot infer the rule, so the profile states it.
func TestShowProfile_ContactSettledFollowsTheStart(t *testing.T) {
	contact := "+4520000001"
	user := users.User{
		ID: "mock-spejder-1", Role: users.RoleSpejder, Name: "Sofie Spejder",
		Phone: "+4530000001", PhoneParent: &contact,
	}

	for _, tc := range []struct {
		name   string
		status string
		want   bool
	}{
		{"not started", "", false},
		{"racing", person.MemberStatusRacing, true},
	} {
		app := newTestApp(t)
		app.config.eventYear = "2026"
		app.models = data.NewModels(
			stubDirectory{u: user},
			scans.NewMockSource(),
			nil,
			&stubPeople{found: true, p: person.Person{
				PersonID: "mock-spejder-1", Year: "2026", AppRole: person.RoleSpejder,
				Phone: "+4530000001", PhoneParent: &contact, MemberStatus: tc.status,
			}},
			nil,
		)

		srv := httptest.NewServer(app.routes())
		cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
		resp := getWithCookies(t, srv.URL+"/api/me/profile", cookies)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		srv.Close()

		var out struct {
			ContactSettled bool `json:"contact_settled"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatalf("%s: decode: %v", tc.name, err)
		}
		if out.ContactSettled != tc.want {
			t.Errorf("%s: contact_settled = %v, want %v", tc.name, out.ContactSettled, tc.want)
		}
	}
}

// An outage must not lock a member out of correcting their record. Same degradation choice as
// confirmationRequired: the app stays useful, and nothing irreversible happens either way.
func TestShowProfile_ContactSettledIsFalseWithoutAProjection(t *testing.T) {
	contact := "+4520000001"
	app := profileAppFor(t, users.User{
		ID: "mock-spejder-1", Role: users.RoleSpejder, Name: "Sofie Spejder",
		Phone: "+4530000001", PhoneParent: &contact,
	})

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	resp := getWithCookies(t, srv.URL+"/api/me/profile", cookies)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var out struct {
		ContactSettled bool `json:"contact_settled"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.ContactSettled {
		t.Error("contact_settled must be false when the projection cannot answer")
	}
}
