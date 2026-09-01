package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nathejk.dk/internal/users"
)

// Switching profile (PRD 012, task 179).
//
// The switch deliberately requires no new SMS: the caller already proved PIN control of the number
// at login, and every profile on it is reachable through the login chooser. What these tests pin is
// the part that keeps that safe — the token is minted for the caller's *own* number, read from the
// directory, and a number with one profile gets nothing.

func switchApp(t *testing.T) *application {
	t.Helper()
	app := newTestApp(t)
	app.config.eventYear = "2026"
	return app
}

// postSwitch signs in as the given number and asks to switch.
func postSwitch(t *testing.T, app *application, srv *httptest.Server, phone, normalized string) (*http.Response, []byte) {
	t.Helper()
	cookies := authedCookiesChoosing(t, app, srv, phone, normalized)
	return postWithCookies(t, srv.URL+"/api/auth/switch", cookies)
}

func postWithCookies(t *testing.T, url string, cookies []*http.Cookie) (*http.Response, []byte) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, body
}

// authedCookiesChoosing signs in, completing the chooser when the number is shared.
//
// The plain authedCookies helper assumes a single-owner number: for a shared one /auth/verify
// answers with candidates instead of a session, so a test about switching — which needs a shared
// number by definition — has to finish the choice first.
func authedCookiesChoosing(t *testing.T, app *application, srv *httptest.Server, phone, normalized string) []*http.Cookie {
	t.Helper()

	code, err := app.pins.Issue(normalized)
	if err != nil {
		t.Fatalf("issue pin: %v", err)
	}
	resp := postJSON(t, srv.URL+"/api/auth/verify", `{"phone":"`+phone+`","pin":"`+code+`"}`)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("verify status = %d, want 200", resp.StatusCode)
	}

	var out chooseRequiredResponse
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decoding verify: %v", err)
	}
	if out.ChoiceToken == "" {
		// Single owner: the session cookie is already set.
		return resp.Cookies()
	}

	chosen := postJSON(t, srv.URL+"/api/auth/choose",
		`{"token":"`+out.ChoiceToken+`","user_id":"`+out.Candidates[0].UserID+`"}`)
	io.Copy(io.Discard, chosen.Body)
	chosen.Body.Close()
	if chosen.StatusCode != http.StatusOK {
		t.Fatalf("choose status = %d, want 200", chosen.StatusCode)
	}
	return chosen.Cookies()
}

func TestSwitchProfile_RequiresAuth(t *testing.T) {
	app := switchApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, _ := postWithCookies(t, srv.URL+"/api/auth/switch", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestSwitchProfile_ReturnsTheNumbersProfiles(t *testing.T) {
	app := switchApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, body := postSwitch(t, app, srv, "30000008", users.MockSharedPhone)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", resp.StatusCode, body)
	}

	var out chooseRequiredResponse
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if out.ChoiceToken == "" {
		t.Error("no choice token: the client cannot complete the switch")
	}
	if len(out.Candidates) < 2 {
		t.Fatalf("want the number's profiles, got %d", len(out.Candidates))
	}
	// The same shape a shared-number /auth/verify returns, so the client needs one code path.
	for _, c := range out.Candidates {
		if c.UserID == "" || c.Name == "" {
			t.Errorf("candidate is missing what the user picks by: %+v", c)
		}
	}
}

// A number with one profile must not produce a chooser offering the profile the user is already in.
func TestSwitchProfile_RefusesASingleProfileNumber(t *testing.T) {
	app := switchApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, body := postSwitch(t, app, srv, "30000002", "+4530000002")
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want 409. body: %s", resp.StatusCode, body)
	}
	if strings.Contains(string(body), "choice_token") {
		t.Error("a token was minted for a number with nothing to switch to")
	}
}

// The completed switch: the token from /auth/switch is redeemed through the unchanged
// /auth/choose, which issues a session replacing the previous one — so there is no signed-out gap.
func TestSwitchProfile_CompletesThroughChoose(t *testing.T) {
	app := switchApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookiesChoosing(t, app, srv, "30000008", users.MockSharedPhone)
	resp, body := postWithCookies(t, srv.URL+"/api/auth/switch", cookies)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("switch status = %d: %s", resp.StatusCode, body)
	}

	var out chooseRequiredResponse
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decoding: %v", err)
	}

	// Pick the profile that is *not* the one the helper signed in as.
	first := out.Candidates[0].UserID
	other := out.Candidates[len(out.Candidates)-1].UserID
	if first == other {
		t.Fatal("expected at least two distinct profiles")
	}

	chosen := postJSON(t, srv.URL+"/api/auth/choose",
		`{"token":"`+out.ChoiceToken+`","user_id":"`+other+`"}`)
	chosenBody, _ := io.ReadAll(chosen.Body)
	chosen.Body.Close()

	if chosen.StatusCode != http.StatusOK {
		t.Fatalf("choose status = %d: %s", chosen.StatusCode, chosenBody)
	}
	var identity identityResponse
	if err := json.Unmarshal(chosenBody, &identity); err != nil {
		t.Fatalf("decoding identity: %v", err)
	}
	if identity.UserID != other {
		t.Errorf("signed in as %q, want %q", identity.UserID, other)
	}
	// A replacement session cookie, not an addition.
	if len(chosen.Cookies()) == 0 {
		t.Error("no session cookie issued for the chosen profile")
	}
}

// The property that keeps the switch as safe as login: a token minted for one number cannot be
// redeemed for a profile on another. /auth/choose re-checks ownership, and this asserts the switch
// path inherits that rather than bypassing it.
func TestSwitchProfile_TokenCannotReachAnotherNumbersProfile(t *testing.T) {
	app := switchApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookiesChoosing(t, app, srv, "30000008", users.MockSharedPhone)
	resp, body := postWithCookies(t, srv.URL+"/api/auth/switch", cookies)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("switch status = %d: %s", resp.StatusCode, body)
	}

	var out chooseRequiredResponse
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decoding: %v", err)
	}

	// A profile on a different number entirely.
	stranger, found := app.models.Users.Lookup("+4530000002")
	if !found {
		t.Fatal("fixture: expected a single-owner number to look up")
	}

	chosen := postJSON(t, srv.URL+"/api/auth/choose",
		`{"token":"`+out.ChoiceToken+`","user_id":"`+stranger.ID+`"}`)
	io.Copy(io.Discard, chosen.Body)
	chosen.Body.Close()

	if chosen.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401: a switch token must not reach another number's profile", chosen.StatusCode)
	}
}

// Guardian numbers never enter the PWA (`.rules`). The candidate payload is shared with login, so
// this should already hold — asserted rather than assumed, on a fixture that has one.
func TestSwitchProfile_NeverCarriesAGuardianNumber(t *testing.T) {
	app := switchApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := postSwitch(t, app, srv, "30000008", users.MockSharedPhone)

	for _, forbidden := range []string{"+4520", "phoneParent", "phone_parent", "guardian"} {
		if strings.Contains(string(body), forbidden) {
			t.Errorf("switch payload carries %q: %s", forbidden, body)
		}
	}
}

func TestMe_ReportsProfileCount(t *testing.T) {
	app := switchApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	// Shared number: more than one profile, so the client offers the switcher.
	shared := authedCookiesChoosing(t, app, srv, "30000008", users.MockSharedPhone)
	sharedResp := getWithCookies(t, srv.URL+"/api/me", shared)
	sharedBody, _ := io.ReadAll(sharedResp.Body)
	sharedResp.Body.Close()

	var sharedIdentity identityResponse
	if err := json.Unmarshal(sharedBody, &sharedIdentity); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if sharedIdentity.ProfileCount < 2 {
		t.Errorf("profile_count = %d, want >= 2 for a shared number", sharedIdentity.ProfileCount)
	}

	// Single owner: exactly one, so the control stays hidden.
	solo := authedCookiesChoosing(t, app, srv, "30000002", "+4530000002")
	soloResp := getWithCookies(t, srv.URL+"/api/me", solo)
	soloBody, _ := io.ReadAll(soloResp.Body)
	soloResp.Body.Close()

	var soloIdentity identityResponse
	if err := json.Unmarshal(soloBody, &soloIdentity); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if soloIdentity.ProfileCount != 1 {
		t.Errorf("profile_count = %d, want 1", soloIdentity.ProfileCount)
	}
}
