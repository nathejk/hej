package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jrgensen/cqrs/cqrstest"
	"github.com/nathejk/shared-go/messages"

	"nathejk.dk/internal/ratelimit"
	"nathejk.dk/internal/session"
	"nathejk.dk/nathejk/table/person"
)

// Three recall attempts per login session (PRD 015, task 227).

// spejderWhoMustConfirm is a member the check applies to, with an own number the PIN can prove so
// the give-up outcome has something to carry.
func spejderWhoMustConfirm() person.Person {
	guardian := "+4520000001"
	return person.Person{
		PersonID:    "mock-spejder-1",
		Year:        "2026",
		AppRole:     person.RoleSpejder,
		Phone:       confirmNormalized,
		PhoneParent: &guardian,
	}
}

func postConfirmDigits(t *testing.T, srv *httptest.Server, cookies []*http.Cookie, digits string) *http.Response {
	t.Helper()
	return postJSONWithCookies(t, srv.URL+"/api/me/profile/confirm",
		`{"digits":"`+digits+`","acknowledged":true}`, cookies)
}

// decodeAttemptFailure reads the structured 400 body, failing the test if it is not one.
func decodeAttemptFailure(t *testing.T, resp *http.Response) confirmAttemptFailedResponse {
	t.Helper()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 400; body = %s", resp.StatusCode, body)
	}
	var out confirmAttemptFailedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return out
}

// The shape of the whole rule in one test: two misses leave the member in the check, the third
// ends it.
func TestConfirmAttempts_ThirdFailureClosesTheCheck(t *testing.T) {
	pub := &cqrstest.Publisher{}
	app := confirmTestApp(t, pub, spejderWhoMustConfirm())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, confirmPhone, confirmNormalized)
	pub.Reset()

	first := decodeAttemptFailure(t, postConfirmDigits(t, srv, cookies, "99"))
	if first.CheckClosed {
		t.Error("the first miss must not end the check")
	}
	if first.AttemptsRemaining != 2 {
		t.Errorf("attempts_remaining = %d after one miss, want 2", first.AttemptsRemaining)
	}

	second := decodeAttemptFailure(t, postConfirmDigits(t, srv, cookies, "98"))
	if second.CheckClosed {
		t.Error("the second miss must not end the check")
	}
	if second.AttemptsRemaining != 1 {
		t.Errorf("attempts_remaining = %d after two misses, want 1", second.AttemptsRemaining)
	}

	third := decodeAttemptFailure(t, postConfirmDigits(t, srv, cookies, "97"))
	if !third.CheckClosed {
		t.Error("the third miss must end the check, or the member is stuck in it")
	}
	if third.AttemptsRemaining != 0 {
		t.Errorf("attempts_remaining = %d after three misses, want 0", third.AttemptsRemaining)
	}
	// Kind rather than accusatory: not knowing a parent's number by heart is expected of a
	// 13-year-old, and this is the last thing the step says to them.
	if !strings.Contains(third.Error, "komme videre") {
		t.Errorf("exhaustion message = %q, want it to say they can carry on", third.Error)
	}
}

// The third failure records the same outcome as a skip. This is task 228's last criterion: both
// paths must be one fact on the stream, not two shapes a consumer has to learn.
func TestConfirmAttempts_ExhaustionPublishesTheSameEventAsASkip(t *testing.T) {
	exhausted := func(t *testing.T) messages.NathejkMemberVerified {
		t.Helper()
		pub := &cqrstest.Publisher{}
		app := confirmTestApp(t, pub, spejderWhoMustConfirm())
		srv := httptest.NewServer(app.routes())
		defer srv.Close()
		cookies := authedCookies(t, app, srv, confirmPhone, confirmNormalized)
		pub.Reset()

		for _, digits := range []string{"99", "98", "97"} {
			resp := postConfirmDigits(t, srv, cookies, digits)
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		if len(pub.Messages) != 1 {
			t.Fatalf("exhaustion published %d events, want 1", len(pub.Messages))
		}
		if got := pub.Subjects()[0]; got != "NATHEJK.2026.spejder.mock-spejder-1.verified" {
			t.Errorf("subject = %q", got)
		}
		var body messages.NathejkMemberVerified
		if err := pub.Messages[0].Body(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return body
	}

	skipped := func(t *testing.T) messages.NathejkMemberVerified {
		t.Helper()
		pub := &cqrstest.Publisher{}
		app := confirmTestApp(t, pub, spejderWhoMustConfirm())
		srv := httptest.NewServer(app.routes())
		defer srv.Close()
		cookies := authedCookies(t, app, srv, confirmPhone, confirmNormalized)
		pub.Reset()

		resp := postSkip(t, srv, cookies)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if len(pub.Messages) != 1 {
			t.Fatalf("skip published %d events, want 1", len(pub.Messages))
		}
		var body messages.NathejkMemberVerified
		if err := pub.Messages[0].Body(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return body
	}

	a, b := exhausted(t), skipped(t)
	if a.MemberID != b.MemberID || a.Phone != b.Phone || a.PhoneContact != b.PhoneContact {
		t.Errorf("exhaustion and skip must record the same fact:\n exhausted = %+v\n skipped   = %+v", a, b)
	}
	if a.PhoneContact != "" {
		t.Errorf("neither path may name a contact number, got %q", a.PhoneContact)
	}
}

// A fourth attempt gets the same 409 as every other "nothing to confirm" reason, so the PWA has one
// thing to handle: carry on into the app.
func TestConfirmAttempts_FourthAttemptIs409(t *testing.T) {
	app := confirmTestApp(t, &cqrstest.Publisher{}, spejderWhoMustConfirm())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, confirmPhone, confirmNormalized)
	for _, digits := range []string{"99", "98", "97"} {
		resp := postConfirmDigits(t, srv, cookies, digits)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	resp := postConfirmDigits(t, srv, cookies, confirmDigits)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409 once the check is over", resp.StatusCode)
	}
}

// Even the *right* answer is refused once the check is closed. Worth its own assertion: the
// tempting implementation checks the digits first and lets a lucky fourth guess through, which
// would mean the recorded outcome and the member's screen disagree.
func TestConfirmAttempts_CorrectDigitsAfterExhaustionStillRefused(t *testing.T) {
	pub := &cqrstest.Publisher{}
	app := confirmTestApp(t, pub, spejderWhoMustConfirm())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, confirmPhone, confirmNormalized)
	for _, digits := range []string{"99", "98", "97"} {
		resp := postConfirmDigits(t, srv, cookies, digits)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	pub.Reset()

	resp := postConfirmDigits(t, srv, cookies, confirmDigits)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if len(pub.Messages) != 0 {
		t.Errorf("published %d events after the check closed, want 0", len(pub.Messages))
	}
}

// A new login is a new session, and a member who comes back tomorrow having asked their mother
// gets a fresh three (PRD 015 §6). The counter is keyed by the session's expiry, so this also
// pins that key: with a member-only key, these attempts would continue the earlier count.
func TestConfirmAttempts_ANewLoginResetsTheCount(t *testing.T) {
	app := confirmTestApp(t, &cqrstest.Publisher{}, spejderWhoMustConfirm())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	first := authedCookies(t, app, srv, confirmPhone, confirmNormalized)
	for _, digits := range []string{"99", "98", "97"} {
		resp := postConfirmDigits(t, srv, first, digits)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	// A second login. The counter is keyed by the session's expiry, so the new session needs a
	// different one — which a real second login gets from the clock, and this gets by issuing with
	// a different TTL. Same member, same phone, same PIN flow.
	app.sessions = session.NewManager([]byte("test-secret"), 2*time.Hour, false)
	second := authedCookies(t, app, srv, confirmPhone, confirmNormalized)

	failure := decodeAttemptFailure(t, postConfirmDigits(t, srv, second, "99"))
	if failure.CheckClosed {
		t.Error("a new login must not inherit the previous session's exhausted count")
	}
	if failure.AttemptsRemaining != 2 {
		t.Errorf("attempts_remaining = %d on a fresh login, want 2", failure.AttemptsRemaining)
	}
}

// Two members behind one campsite wifi must each get their own three attempts. The per-IP limiter
// stays a separate concern: if this rule were enforced by tightening it, the second member would
// be locked out having guessed nothing.
func TestConfirmAttempts_ArePerMemberNotPerIP(t *testing.T) {
	app := confirmTestApp(t, &cqrstest.Publisher{}, spejderWhoMustConfirm())
	// Generous enough that the IP limiter cannot be what refuses anybody here.
	app.confirmLimiter = ratelimit.New(100, time.Minute)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	one := authedCookies(t, app, srv, confirmPhone, confirmNormalized)
	for _, digits := range []string{"99", "98", "97"} {
		resp := postConfirmDigits(t, srv, one, digits)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	// A different member, same client address (httptest is always 127.0.0.1).
	two := authedCookies(t, app, srv, "30000005", "+4530000005")
	failure := decodeAttemptFailure(t, postConfirmDigits(t, srv, two, "99"))
	if failure.CheckClosed || failure.AttemptsRemaining != 2 {
		t.Errorf("a second member on the same IP must get their own three attempts: %+v", failure)
	}
}

// No response on the failure path may carry any digit of the registered number. The member is being
// asked to recall it; a hint turns the check into a copying exercise (PRD 015 §6).
func TestConfirmAttempts_NeverLeakTheNumber(t *testing.T) {
	app := confirmTestApp(t, &cqrstest.Publisher{}, spejderWhoMustConfirm())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, confirmPhone, confirmNormalized)
	for _, digits := range []string{"99", "98", "97"} {
		resp := postConfirmDigits(t, srv, cookies, digits)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		// The registered number, and its two masked digits on their own.
		for _, secret := range []string{"20000001", "+4520000001", `"01"`} {
			if strings.Contains(string(body), secret) {
				t.Errorf("response leaked %q: %s", secret, body)
			}
		}
	}
}
