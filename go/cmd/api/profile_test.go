package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"nathejk.dk/internal/users"
)

func decodeProfile(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body
}

func TestProfile_RequiresAuth(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/me/profile")
	if err != nil {
		t.Fatalf("GET profile: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestProfile_ReturnsOwnDetails(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	resp := getWithCookies(t, srv.URL+"/api/me/profile", cookies)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body := decodeProfile(t, resp)

	for _, field := range []string{"name", "role", "address", "postal_code", "city", "phone"} {
		if v, _ := body[field].(string); v == "" {
			t.Errorf("%s is empty, want the seeded spejder's value", field)
		}
	}
	if body["role"] != string(users.RoleSpejder) {
		t.Errorf("role = %v, want spejder", body["role"])
	}
	if body["team"] == "" {
		t.Error("team should be the seeded patrulje")
	}
	// The seeded spejder has a guardian number on file.
	if parent, ok := body["phone_parent"].(string); !ok || parent == "" {
		t.Errorf("phone_parent = %v, want the seeded guardian number", body["phone_parent"])
	}
}

// The three guardian-number states must survive serialization. This is the whole
// reason the field is a pointer: a client that cannot tell null from "" cannot
// tell "you have no guardian number to register" from "yours is missing", and
// would nag the wrong people.
func TestProfile_GuardianPhoneNullForPopulationsWithoutOne(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	// The seeded bandit is an adult klan member: no guardian number exists for
	// this population, so the field must be null rather than "".
	cookies := authedCookies(t, app, srv, "30000002", "+4530000002")
	resp := getWithCookies(t, srv.URL+"/api/me/profile", cookies)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body := decodeProfile(t, resp)

	if v, present := body["phone_parent"]; !present {
		t.Error("phone_parent must be present in the JSON, as null")
	} else if v != nil {
		t.Errorf("phone_parent = %v, want null", v)
	}
}

func TestProfile_GuardianPhoneEmptyWhenExpectedButMissing(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	// mock-sibling-b is a spejder whose guardian number is on file as empty. He
	// shares a phone with his sister, so signing in as him means going through the
	// chooser — which is also a small end-to-end check that a chosen account gets
	// its *own* profile and not the first candidate's.
	_, verified, _ := verifyFor(t, app, srv, users.MockSharedPhone)
	token, ok := verified["choice_token"].(string)
	if !ok {
		t.Fatalf("expected a choice token for the shared number, got %v", verified)
	}
	_, _, chooseResp := post(t, srv, "/api/auth/choose",
		`{"token":"`+token+`","user_id":"mock-sibling-b"}`)
	cookie := sessionCookie(chooseResp)
	if cookie == nil {
		t.Fatal("choose did not set a session cookie")
	}

	resp := getWithCookies(t, srv.URL+"/api/me/profile", []*http.Cookie{cookie})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body := decodeProfile(t, resp)

	if body["name"] != "Villads Mikkelsen" {
		t.Errorf("name = %v, want the chosen sibling's", body["name"])
	}
	parent, isString := body["phone_parent"].(string)
	if !isString || parent != "" {
		t.Errorf(`phone_parent = %v, want "" (expected but not registered)`, body["phone_parent"])
	}
}

// A session whose user no longer resolves is a gone record, not an empty profile.
func TestProfile_UnknownUserGets404(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	// Swap the directory for one that knows nobody, simulating a member deleted
	// while their session was still valid.
	app.models.Users = emptyDirectory{}

	resp := getWithCookies(t, srv.URL+"/api/me/profile", cookies)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// emptyDirectory recognizes nobody.
type emptyDirectory struct{}

func (emptyDirectory) LookupAll(string) []users.User    { return nil }
func (emptyDirectory) Lookup(string) (users.User, bool) { return users.User{}, false }
func (emptyDirectory) Get(string) (users.User, bool)    { return users.User{}, false }
