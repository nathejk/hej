package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jrgensen/cqrs/cqrstest"

	"nathejk.dk/internal/commands"
	"nathejk.dk/internal/ratelimit"
	"nathejk.dk/internal/track"
)

// Ids from the mock directory (internal/users). Spelled out here rather than exported from
// there: these tests assert on a *subject string*, and the point is to notice if the id that
// ends up in it ever changes shape.
const (
	mockSpejderID = "mock-spejder-1"
	mockBanditID  = "mock-bandit-1"
)

// trackTestApp returns an app with a fake publisher installed, plus the publisher so a test
// can assert on what was published.
func trackTestApp(t *testing.T) (*application, *cqrstest.Publisher, *httptest.Server) {
	t.Helper()
	app := newTestApp(t)
	app.config.eventYear = "2026"

	pub := &cqrstest.Publisher{}
	holder := commands.NewPublisherHolder()
	holder.Set(pub)
	app.commands = commands.New(holder)

	srv := httptest.NewServer(app.routes())
	t.Cleanup(srv.Close)
	return app, pub, srv
}

func trackBody(points ...string) string {
	return `{"points":[` + strings.Join(points, ",") + `]}`
}

func onePoint(tsOffset time.Duration) string {
	return fmt.Sprintf(`{"ts":%d,"lat":55.7,"lng":12.2,"accuracy":7.5}`, time.Now().Add(tsOffset).UnixMilli())
}

func decodeCounts(t *testing.T, resp *http.Response) map[string]int {
	t.Helper()
	var got map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return got
}

func TestTrackRequiresAuthentication(t *testing.T) {
	_, pub, srv := trackTestApp(t)

	resp := postJSON(t, srv.URL+"/api/track", trackBody(onePoint(0)))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	if len(pub.Messages) != 0 {
		t.Fatalf("published %d messages while unauthenticated, want 0", len(pub.Messages))
	}
}

func TestTrackPublishesToPerPersonSubject(t *testing.T) {
	app, pub, srv := trackTestApp(t)
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp := postJSONWithCookies(t, srv.URL+"/api/track", trackBody(onePoint(-time.Minute), onePoint(0)), cookies)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 202 (%s)", resp.StatusCode, body)
	}
	counts := decodeCounts(t, resp)
	if counts["accepted"] != 2 || counts["dropped"] != 0 {
		t.Fatalf("counts = %v, want accepted 2 dropped 0", counts)
	}

	if len(pub.Messages) != 1 {
		t.Fatalf("published %d messages, want 1 (one batch, one event)", len(pub.Messages))
	}
	want := "TELEMETRY.2026.track." + mockSpejderID + ".reported"
	if got := pub.Messages[0].Subject().Subject(); got != want {
		t.Fatalf("subject = %q, want %q", got, want)
	}

	var body track.Reported
	if err := pub.Messages[0].Body(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.PersonID != mockSpejderID {
		t.Fatalf("body person = %q, want %q", body.PersonID, mockSpejderID)
	}
	if body.Year != "2026" {
		t.Fatalf("body year = %q, want 2026", body.Year)
	}
	if len(body.Points) != 2 {
		t.Fatalf("body carries %d points, want 2", len(body.Points))
	}
}

// THE security property (task 084): the person is the session's, and a body that tries to
// name someone else cannot even be expressed — ReadJSON disallows unknown fields, so the
// attempt is a 400 rather than a silently-ignored field. Both halves are asserted: the
// rejection, and that nothing was published under the other person's subject.
func TestTrackCannotReportAsAnotherPerson(t *testing.T) {
	app, pub, srv := trackTestApp(t)
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	victim := mockBanditID
	for _, field := range []string{"personId", "person_id", "userId", "uid"} {
		body := fmt.Sprintf(`{"%s":"%s","points":[%s]}`, field, victim, onePoint(0))
		resp := postJSONWithCookies(t, srv.URL+"/api/track", body, cookies)
		got := resp.StatusCode
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if got != http.StatusBadRequest {
			t.Fatalf("body naming %q via %q: status = %d, want 400", victim, field, got)
		}
	}

	for _, subject := range pub.Subjects() {
		if strings.Contains(subject, victim) {
			t.Fatalf("published on the victim's subject %q", subject)
		}
	}
}

func TestTrackRejectsEmptyAndOversizedBatches(t *testing.T) {
	app, pub, srv := trackTestApp(t)
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp := postJSONWithCookies(t, srv.URL+"/api/track", `{"points":[]}`, cookies)
	empty := resp.StatusCode
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if empty != http.StatusBadRequest {
		t.Fatalf("empty batch: status = %d, want 400", empty)
	}

	points := make([]string, track.MaxPointsPerBatch+1)
	for i := range points {
		points[i] = onePoint(-time.Duration(i) * time.Second)
	}
	resp = postJSONWithCookies(t, srv.URL+"/api/track", trackBody(points...), cookies)
	oversized := resp.StatusCode
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	// 413 specifically, so the client knows to split rather than to give up.
	if oversized != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized batch: status = %d, want 413", oversized)
	}

	if len(pub.Messages) != 0 {
		t.Fatalf("published %d messages for rejected batches, want 0", len(pub.Messages))
	}
}

// A mixed batch is accepted with the junk counted, and only the good points are published.
func TestTrackDropsJunkPointsAndReportsTheCount(t *testing.T) {
	app, pub, srv := trackTestApp(t)
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	body := trackBody(
		onePoint(-time.Minute),
		`{"ts":0,"lat":55.7,"lng":12.2,"accuracy":5}`,
		`{"ts":1787836501238,"lat":0,"lng":0,"accuracy":5}`,
		onePoint(0),
	)
	resp := postJSONWithCookies(t, srv.URL+"/api/track", body, cookies)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", resp.StatusCode)
	}
	counts := decodeCounts(t, resp)
	if counts["accepted"] != 2 || counts["dropped"] != 2 {
		t.Fatalf("counts = %v, want accepted 2 dropped 2", counts)
	}

	var published track.Reported
	if err := pub.Messages[0].Body(&published); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(published.Points) != 2 {
		t.Fatalf("published %d points, want only the 2 good ones", len(published.Points))
	}
}

// Every point junk: accepted with 0, and nothing published. A 4xx here would make the client
// retry a batch that can never succeed, blocking every later point behind it.
func TestTrackAcceptsAnAllJunkBatchWithoutPublishing(t *testing.T) {
	app, pub, srv := trackTestApp(t)
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp := postJSONWithCookies(t, srv.URL+"/api/track", trackBody(`{"ts":0,"lat":0,"lng":0,"accuracy":0}`), cookies)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", resp.StatusCode)
	}
	counts := decodeCounts(t, resp)
	if counts["accepted"] != 0 || counts["dropped"] != 1 {
		t.Fatalf("counts = %v, want accepted 0 dropped 1", counts)
	}
	if len(pub.Messages) != 0 {
		t.Fatalf("published %d messages with nothing valid to send, want 0", len(pub.Messages))
	}
}

// With no broker the batch must NOT be accepted: it exists only in the phone's IndexedDB
// until the server takes it, so a 202 would tell the client to delete the only copy.
func TestTrackFailsRetryablyWithoutAPublisher(t *testing.T) {
	app, _, srv := trackTestApp(t)
	app.commands = commands.New(commands.NewPublisherHolder()) // no publisher installed
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp := postJSONWithCookies(t, srv.URL+"/api/track", trackBody(onePoint(0)), cookies)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 so the client retries", resp.StatusCode)
	}
}

// A publish that fails for any other reason must also be retryable rather than accepted —
// including a subject no stream claims, which the acked JetStream publish surfaces as an
// error.
func TestTrackFailsRetryablyWhenPublishErrors(t *testing.T) {
	app, pub, srv := trackTestApp(t)
	pub.Err = fmt.Errorf("no responders available for subject")
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp := postJSONWithCookies(t, srv.URL+"/api/track", trackBody(onePoint(0)), cookies)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}

// Rate limited per user, not per IP: the whole point is that participants share networks.
func TestTrackIsRateLimitedPerUser(t *testing.T) {
	app, _, srv := trackTestApp(t)
	app.trackLimiter = ratelimit.New(1, time.Minute)
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp := postJSONWithCookies(t, srv.URL+"/api/track", trackBody(onePoint(-time.Minute)), cookies)
	first := resp.StatusCode
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if first != http.StatusAccepted {
		t.Fatalf("first request: status = %d, want 202", first)
	}

	resp = postJSONWithCookies(t, srv.URL+"/api/track", trackBody(onePoint(0)), cookies)
	second := resp.StatusCode
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if second != http.StatusTooManyRequests {
		t.Fatalf("second request: status = %d, want 429", second)
	}

	// A different user on the same connection must be unaffected — that is what "per user"
	// buys, and an IP-keyed limiter would fail here while passing everything above.
	other := authedCookies(t, app, srv, "30000002", "+4530000002")
	resp = postJSONWithCookies(t, srv.URL+"/api/track", trackBody(onePoint(0)), other)
	otherStatus := resp.StatusCode
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if otherStatus != http.StatusAccepted {
		t.Fatalf("second user: status = %d, want 202 — the limiter is not per user", otherStatus)
	}
}
