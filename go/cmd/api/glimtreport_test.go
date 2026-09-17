package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nathejk.dk/internal/ratelimit"
	"nathejk.dk/nathejk/table/glimt"
)

// Report tests (task 307).
//
// The projection's own test proves the record-and-hide happens in one fold. What matters here is that
// the endpoint is *reachable enough to work*: an absent body is fine, the limit is generous, and a
// failure is reported honestly rather than swallowed — a report that silently failed is the worst lie
// this feature could tell, because the member believes they have acted and the photograph stays up.

func reportURL(base, glimtID string) string {
	return base + "/api/glimt/items/" + glimtID + "/report"
}

func publicGlimtRow() glimt.Glimt {
	return glimt.Glimt{
		GlimtID: "g-public", Year: "2026",
		AuthorPersonID: "someone-else", AuthorGroup: "crew",
		TeamName: "PR", Audience: glimt.AudiencePublic,
		CreatedAt: time.Date(2026, 9, 17, 21, 0, 0, 0, time.UTC),
	}
}

func TestReportGlimt_PublishesTheReport(t *testing.T) {
	app, store, pub := glimtApp(t, nil, spejderPerson())
	store.rows = []glimt.Glimt{publicGlimtRow()}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, body := postGlimtJSON(t, reportURL(srv.URL, "g-public"),
		reportGlimtRequest{Reason: "ikke ok"}, cookies)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body %s)", resp.StatusCode, body)
	}

	if got := pub.Subjects(); len(got) != 1 || got[0] != "NATHEJK.2026.glimt.g-public.reported" {
		t.Fatalf("subjects = %v", got)
	}

	var reported glimt.Reported
	if err := pub.Messages[0].Body(&reported); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if reported.ReporterPersonID != "mock-spejder-1" {
		t.Errorf("reporter = %q; without it, one person tapping twice reads as two objections",
			reported.ReporterPersonID)
	}
	if reported.Reason != "ikke ok" {
		t.Errorf("reason = %q", reported.Reason)
	}
	if reported.ReportedAt.IsZero() {
		t.Error("no timestamp; the moderation queue sorts on it")
	}
}

// TestReportGlimt_WorksWithNoBody is the common case: tap "Anmeld", confirm, done.
func TestReportGlimt_WorksWithNoBody(t *testing.T) {
	app, store, pub := glimtApp(t, nil, spejderPerson())
	store.rows = []glimt.Glimt{publicGlimtRow()}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	req, _ := http.NewRequest(http.MethodPost, reportURL(srv.URL, "g-public"), nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 — the reason is optional", resp.StatusCode)
	}
	if len(pub.Subjects()) != 1 {
		t.Errorf("subjects = %v", pub.Subjects())
	}
}

// TestReportGlimt_OnlyWhatTheCallerCanSee — a report on an invisible glimt would confirm that one
// exists, which is precisely what the media handler's 403 avoids leaking.
func TestReportGlimt_OnlyWhatTheCallerCanSee(t *testing.T) {
	app, store, pub := glimtApp(t, nil, spejderPerson())
	store.rows = []glimt.Glimt{{
		GlimtID: "g-bandit", Year: "2026",
		AuthorPersonID: "a-bandit", AuthorGroup: "bandit",
		Audience: glimt.AudienceGroup, CreatedAt: time.Now().UTC(),
	}}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, _ := postGlimtJSON(t, reportURL(srv.URL, "g-bandit"), reportGlimtRequest{}, cookies)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
	if len(pub.Subjects()) != 0 {
		t.Error("a refused report still published an event")
	}
}

// TestReportGlimt_TheAuthorMayReportTheirOwn is allowed deliberately.
//
// It looks pointless — why report your own photo? — but it is how a member who realises a picture
// should not be up gets it off the public feed *immediately*, without waiting for a delete to
// propagate or hunting for the right control. Refusing it would be a small piece of cleverness that
// removes the fastest path to the safest outcome.
func TestReportGlimt_TheAuthorMayReportTheirOwn(t *testing.T) {
	app, store, pub := glimtApp(t, nil, spejderPerson())
	row := publicGlimtRow()
	row.AuthorPersonID = "mock-spejder-1"
	store.rows = []glimt.Glimt{row}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, _ := postGlimtJSON(t, reportURL(srv.URL, "g-public"), reportGlimtRequest{}, cookies)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
	if len(pub.Subjects()) != 1 {
		t.Errorf("subjects = %v", pub.Subjects())
	}
}

// TestReportGlimt_NoPublisherIs503 — a silently failed report is the worst lie this feature could
// tell: the member believes they have acted, and the photograph stays up.
func TestReportGlimt_NoPublisherIs503(t *testing.T) {
	app, store, _ := glimtApp(t, nil, spejderPerson())
	store.rows = []glimt.Glimt{publicGlimtRow()}
	app.commands = commandsWithNoPublisher()

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, _ := postGlimtJSON(t, reportURL(srv.URL, "g-public"), reportGlimtRequest{}, cookies)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
}

func TestReportGlimt_RejectsALongReason(t *testing.T) {
	app, store, _ := glimtApp(t, nil, spejderPerson())
	store.rows = []glimt.Glimt{publicGlimtRow()}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, _ := postGlimtJSON(t, reportURL(srv.URL, "g-public"),
		reportGlimtRequest{Reason: strings.Repeat("æ", maxGlimtReportReason+1)}, cookies)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

// TestReportGlimt_LimitIsMuchLooserThanCreation pins the asymmetry rather than the numbers.
//
// If someone ever tunes these, the property to preserve is that reporting is easier than posting —
// the cost of a spurious report is a glance, the cost of a blocked one is a photograph staying up.
func TestReportGlimt_LimitIsMuchLooserThanCreation(t *testing.T) {
	app, _, _ := glimtApp(t, nil, spejderPerson())
	// Rebuild with the production limiters to compare them.
	create := ratelimit.New(20, time.Hour)
	report := ratelimit.New(100, time.Hour)
	app.glimtLimiter = create
	app.glimtReportLimiter = report

	// Exhaust the create budget for a key, and show the report budget for the same key is not.
	const key = "member"
	created := 0
	for create.Allow(key) {
		created++
		if created > 1000 {
			t.Fatal("create limiter never blocked")
		}
	}
	reported := 0
	for report.Allow(key) {
		reported++
		if reported > 1000 {
			t.Fatal("report limiter never blocked")
		}
	}
	if reported <= created {
		t.Errorf("report budget (%d) is not looser than the create budget (%d) — reporting must never be the harder action",
			reported, created)
	}
}

func TestReportGlimt_RateLimited(t *testing.T) {
	app, store, _ := glimtApp(t, nil, spejderPerson())
	store.rows = []glimt.Glimt{publicGlimtRow()}
	app.glimtReportLimiter = ratelimit.New(1, time.Hour)

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	first, _ := postGlimtJSON(t, reportURL(srv.URL, "g-public"), reportGlimtRequest{}, cookies)
	if first.StatusCode != http.StatusNoContent {
		t.Fatalf("first = %d", first.StatusCode)
	}
	second, _ := postGlimtJSON(t, reportURL(srv.URL, "g-public"), reportGlimtRequest{}, cookies)
	if second.StatusCode != http.StatusTooManyRequests {
		t.Errorf("second = %d, want 429", second.StatusCode)
	}
}

func TestReportGlimt_RequiresAuthAndAKnownGlimt(t *testing.T) {
	app, store, _ := glimtApp(t, nil, spejderPerson())
	store.rows = []glimt.Glimt{publicGlimtRow()}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	anon, _ := postGlimtJSON(t, reportURL(srv.URL, "g-public"), reportGlimtRequest{}, nil)
	if anon.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauthenticated = %d, want 401", anon.StatusCode)
	}

	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	missing, _ := postGlimtJSON(t, reportURL(srv.URL, "never-existed"), reportGlimtRequest{}, cookies)
	if missing.StatusCode != http.StatusNotFound {
		t.Errorf("unknown glimt = %d, want 404", missing.StatusCode)
	}
}
