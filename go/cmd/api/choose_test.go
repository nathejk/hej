package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nathejk.dk/internal/users"
)

// post sends a JSON body and returns the status, decoded body and response.
//
// Named differently from auth_test.go's postJSON to avoid clashing with it; both live
// in package main.
func post(t *testing.T, srv *httptest.Server, path, body string) (int, map[string]any, *http.Response) {
	t.Helper()

	resp, err := http.Post(srv.URL+path, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()

	var decoded map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&decoded)
	return resp.StatusCode, decoded, resp
}

// verifyFor issues a PIN directly (the pattern auth_test.go already uses) and posts it
// to /auth/verify, returning the response.
func verifyFor(t *testing.T, app *application, srv *httptest.Server, phone string) (int, map[string]any, *http.Response) {
	t.Helper()

	code, err := app.pins.Issue(phone)
	if err != nil {
		t.Fatalf("issuing a PIN for %s: %v", phone, err)
	}
	return post(t, srv, "/api/auth/verify", `{"phone":"`+phone+`","pin":"`+code+`"}`)
}

func sessionCookie(resp *http.Response) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == "hej_session" && c.Value != "" {
			return c
		}
	}
	return nil
}

// A uniquely-held number still logs straight in. The chooser must not become a
// tollgate for the 87% of members who do not share a phone.
func TestVerifySingleOwnerSignsInDirectly(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	code, body, resp := verifyFor(t, app, srv, "+4530000001")
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d (%v)", code, body)
	}
	if body["choice_token"] != nil {
		t.Fatal("a single owner must not be asked to choose")
	}
	if body["user_id"] == nil {
		t.Fatalf("want an identity in the response, got %v", body)
	}
	if sessionCookie(resp) == nil {
		t.Fatal("want a session cookie for a single owner")
	}
}

// A shared number returns candidates and a token instead of a session — a different
// shape of success, not a failure.
func TestVerifySharedNumberAsksToChoose(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	code, body, resp := verifyFor(t, app, srv, users.MockSharedPhone)
	if code != http.StatusOK {
		t.Fatalf("want 200 (verification did succeed), got %d", code)
	}
	if body["choice_token"] == nil {
		t.Fatalf("want a choice token, got %v", body)
	}
	if body["user_id"] != nil {
		t.Fatal("no identity may be returned before the user has chosen")
	}
	if sessionCookie(resp) != nil {
		t.Fatal("no session may be issued before the user has chosen — that would be a guess")
	}

	candidates, ok := body["candidates"].([]any)
	if !ok || len(candidates) != 2 {
		t.Fatalf("want 2 candidates, got %v", body["candidates"])
	}

	// The payload must stay minimal: one person is being shown another's details.
	first := candidates[0].(map[string]any)
	if first["name"] == nil || first["user_id"] == nil {
		t.Fatalf("candidate needs a name and an id: %v", first)
	}
	if name, _ := first["name"].(string); strings.Contains(name, " ") {
		t.Errorf("candidate name should be a first name only, got %q", name)
	}
	for _, forbidden := range []string{"address", "phone", "birthday", "email", "role"} {
		if _, present := first[forbidden]; present {
			t.Errorf("candidate must not expose %q: %v", forbidden, first)
		}
	}
}

// The happy path for a shared number: choose, get a session.
func TestChooseCompletesLogin(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body, _ := verifyFor(t, app, srv, users.MockSharedPhone)
	token := body["choice_token"].(string)
	chosen := body["candidates"].([]any)[1].(map[string]any)["user_id"].(string)

	code, chooseBody, resp := post(t, srv, "/api/auth/choose",
		`{"token":"`+token+`","user_id":"`+chosen+`"}`)

	if code != http.StatusOK {
		t.Fatalf("want 200, got %d (%v)", code, chooseBody)
	}
	if chooseBody["user_id"] != chosen {
		t.Fatalf("signed in as %v, want %s", chooseBody["user_id"], chosen)
	}
	if sessionCookie(resp) == nil {
		t.Fatal("want a session cookie after choosing")
	}
}

// The attack this endpoint exists to resist: a valid token, but a user who does not
// own the verified number. Without the re-check, a verified PIN for any number would
// become a session as any user in the system.
func TestChooseRejectsAUserWhoDoesNotOwnTheNumber(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body, _ := verifyFor(t, app, srv, users.MockSharedPhone)
	token := body["choice_token"].(string)

	// A real, seeded user — but on a different phone number.
	code, _, resp := post(t, srv, "/api/auth/choose",
		`{"token":"`+token+`","user_id":"mock-samarit-1"}`)

	if code != http.StatusUnauthorized {
		t.Fatalf("want 401 for a non-owner, got %d", code)
	}
	if sessionCookie(resp) != nil {
		t.Fatal("no session may be issued for a user who does not own the verified number")
	}
}

func TestChooseRejectsAForgedToken(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	// Well-formed but not signed with the server's secret.
	code, _, resp := post(t, srv, "/api/auth/choose",
		`{"token":"eyJwIjoiKzQ1MzAwMDAwMDgiLCJlIjo5OTk5OTk5OTk5fQ.bogus","user_id":"mock-sibling-a"}`)

	if code != http.StatusUnauthorized {
		t.Fatalf("want 401 for a forged token, got %d", code)
	}
	if sessionCookie(resp) != nil {
		t.Fatal("a forged token must not produce a session")
	}
}

// The chooser must be unreachable without a verified PIN: no token, no session.
func TestChooseWithoutATokenIsRejected(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	code, _, resp := post(t, srv, "/api/auth/choose", `{"user_id":"mock-sibling-a"}`)
	if code != http.StatusUnauthorized {
		t.Fatalf("want 401 with no token, got %d", code)
	}
	if sessionCookie(resp) != nil {
		t.Fatal("no token must mean no session")
	}
}

// Anti-enumeration: an unknown number behaves exactly like a known one at request-pin,
// and a wrong PIN is indistinguishable from an unknown number at verify.
func TestUnknownNumberIsIndistinguishable(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	known, _, _ := post(t, srv, "/api/auth/request-pin", `{"phone":"+4530000001"}`)
	unknown, _, _ := post(t, srv, "/api/auth/request-pin", `{"phone":"+4599999999"}`)
	if known != unknown {
		t.Fatalf("request-pin distinguishes known (%d) from unknown (%d)", known, unknown)
	}

	code, _, _ := post(t, srv, "/api/auth/verify", `{"phone":"+4599999999","pin":"123456"}`)
	if code != http.StatusUnauthorized {
		t.Fatalf("want 401 for an unknown number, got %d", code)
	}
}
