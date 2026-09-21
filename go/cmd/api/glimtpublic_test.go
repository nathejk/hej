package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jrgensen/cqrs/cqrstest"

	"nathejk.dk/nathejk/table/glimt"
)

// The public Glimt page and its API (PRD 019 §0, §7, §8, task 323).
//
// # The test this file exists for
//
// `TestPublicGlimt_AnAuthenticatedCookieChangesNothing`. PRD 019 §8 names it as a trap: a logged-in
// member's browser *will* send `hej_session` to these handlers, and if any of them read it the public
// page silently becomes a different page for members than for parents — at which point "is this
// public-safe?" stops being answerable, because the answer depends on who asked.
//
// The structural half of that guarantee is in `glimtopenapi_test.go`
// (`TestPublicGlimtRoutesAreNotBehindAuth`): these routes are not wrapped in `requireAuth`, which is
// the only place a session enters the request context, and no handler in the chain calls
// `contextGetSession`. This file proves the resulting behaviour with a real cookie attached, because
// a structural argument is only as good as the assumption it rests on.

// Content-addressed refs, 64 hex chars as ComputeRef emits them. Named so the "no blob ref in a
// public payload" assertion can look for them: a hash in a public response would be a forwardable,
// unrevokable capability — worse here than anywhere else in the feature.
var (
	refA = strings.Repeat("a", 64)
	refB = strings.Repeat("b", 64)
	refC = strings.Repeat("c", 64)
)

func publicGlimtRows() []glimt.Glimt {
	at := func(h int) time.Time { return time.Date(2026, 9, 17, h, 0, 0, 0, time.UTC) }
	hidden := at(20)
	return []glimt.Glimt{
		{GlimtID: "g-public", AuthorPersonID: "a-spejder", AuthorGroup: "spejder",
			TeamNumber: "42", TeamName: "Ørnene", Audience: glimt.AudiencePublic,
			Caption: "ved posten", CreatedAt: at(21),
			Media: []glimt.Media{
				{Ordinal: 0, Ref: refA, ThumbRef: refB, Kind: "image", Width: 1600, Height: 1200},
				{Ordinal: 1, Ref: refC, ThumbRef: refB, Kind: "image", Width: 1200, Height: 1600},
			}},
		{GlimtID: "g-group", AuthorPersonID: "a-spejder", AuthorGroup: "spejder",
			TeamNumber: "42", TeamName: "Ørnene", Audience: glimt.AudienceGroup,
			Caption: "kun os", CreatedAt: at(22),
			Media: []glimt.Media{{Ordinal: 0, Ref: refA, ThumbRef: refB, Kind: "image"}}},
		{GlimtID: "g-nathejk", AuthorPersonID: "a-crew", AuthorGroup: "crew",
			TeamName: "PR", Audience: glimt.AudienceNathejk, CreatedAt: at(22)},
		{GlimtID: "g-public-hidden", AuthorPersonID: "a-spejder", AuthorGroup: "spejder",
			TeamNumber: "43", TeamName: "Ulvene", Audience: glimt.AudiencePublic,
			CreatedAt: at(19), HiddenAt: &hidden,
			Media: []glimt.Media{{Ordinal: 0, Ref: refA, ThumbRef: refB, Kind: "image"}}},
	}
}

func publicApp(t *testing.T) (*application, *stubGlimt, *cqrstest.Publisher) {
	t.Helper()
	app, store, pub := glimtApp(t, publicGlimtRows(), spejderPerson())
	// Off by default in these tests: the read limiter is by IP and every request here comes from
	// the same one, so a tight limit would turn an unrelated assertion into a 429.
	app.publicGlimtReadLimiter = nil
	return app, store, pub
}

func getPublic(t *testing.T, url string, cookies []*http.Cookie) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, body
}

func TestPublicGlimt_ServesOnlyPublicAndNotHidden(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, payload := getPublic(t, srv.URL+"/api/public/glimt", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", resp.StatusCode, payload)
	}

	var out publicGlimtFeedResponse
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	ids := []string{}
	for _, g := range out.Glimt {
		ids = append(ids, g.ID)
	}
	if len(ids) != 1 || ids[0] != "g-public" {
		t.Errorf("public feed = %v, want only [g-public] — a group- or nathejk-scoped glimt, or a "+
			"hidden one, reached the open web", ids)
	}
}

// **The one that matters.** A member's cookie must change nothing.
func TestPublicGlimt_AnAuthenticatedCookieChangesNothing(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	// A real session for a real member, issued the same way every other test does it.
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")
	if len(cookies) == 0 {
		t.Fatal("no session cookie was issued, so this test would pass for the wrong reason")
	}

	for _, path := range []string{"/api/public/glimt", "/2026/glimt"} {
		anonResp, anon := getPublic(t, srv.URL+path, nil)
		authResp, authed := getPublic(t, srv.URL+path, cookies)

		if anonResp.StatusCode != authResp.StatusCode {
			t.Errorf("%s: status %d anonymous vs %d signed in", path,
				anonResp.StatusCode, authResp.StatusCode)
		}
		if string(anon) != string(authed) {
			t.Errorf("%s: the response differs for a signed-in member. The public surface must "+
				"answer the same thing to a member and to a parent, or \"is this public-safe?\" "+
				"stops being a testable question (PRD 019 §8)", path)
		}
	}
}

// The audience is re-checked per glimt, so an id is not enough to pull bytes off the public route.
//
// This is the public counterpart of the most important test in the Glimt backend: a media URL is not
// a bearer token. Here the risk is sharper, because there is no session to fall back on — if this
// check were missing, a guessed or forwarded id would be an unauthenticated read of a group-scoped
// photograph of a child.
func TestPublicGlimtMedia_RefusesNonPublicGlimt(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	for _, id := range []string{"g-group", "g-nathejk", "g-public-hidden"} {
		t.Run(id, func(t *testing.T) {
			// Anonymously, and *with a valid member session*, because the second is the case a
			// reader would assume is fine and is not.
			for name, c := range map[string][]*http.Cookie{"anonymous": nil, "signed in": cookies} {
				resp, _ := getPublic(t, srv.URL+"/api/public/glimt/"+id+"/media/0", c)
				// 404 rather than 403: the caller is anonymous, so a 403 would confirm that a
				// glimt with this id exists and is not public — worth nothing to a parent and
				// something to somebody probing.
				if resp.StatusCode != http.StatusNotFound {
					t.Errorf("%s: status = %d, want 404", name, resp.StatusCode)
				}
			}
		})
	}
}

// storeRealBytes puts actual bytes in the blob store and returns a public glimt referencing them.
//
// Needed because `blobs.Put` derives the ref from the content — the whole point of a
// content-addressed store — so a row cannot simply name `refA` and expect bytes to be there. The
// other tests in this file only need the metadata and are happy with synthetic refs.
func storeRealBytes(t *testing.T, app *application) glimt.Glimt {
	t.Helper()
	ref, err := app.blobs.Put(context.Background(), testImage(t, 64, 64))
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	return glimt.Glimt{
		GlimtID: "g-public", AuthorGroup: "spejder", TeamNumber: "42", TeamName: "Ørnene",
		Audience: glimt.AudiencePublic, CreatedAt: time.Date(2026, 9, 17, 21, 0, 0, 0, time.UTC),
		Media: []glimt.Media{{Ordinal: 0, Ref: string(ref), ThumbRef: string(ref), Kind: "image"}},
	}
}

func TestPublicGlimtMedia_ServesAPublicItem(t *testing.T) {
	app, store, _ := publicApp(t)
	store.rows = []glimt.Glimt{storeRealBytes(t, app)}
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, _ := getPublic(t, srv.URL+"/api/public/glimt/g-public/media/0?variant=thumb", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	// `public`, unlike the authenticated route's `private`. Safe precisely because the answer does
	// not depend on who asked — and valuable, because a public link is the one that gets shared
	// widely enough for a proxy to matter.
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "public") {
		t.Errorf("Cache-Control = %q, want a shared-cacheable value", cc)
	}
	if strings.Contains(resp.Header.Get("Cache-Control"), "private") {
		t.Error("the public media route answers `private`, which defeats the reason it exists")
	}
}

// The authenticated route must keep answering `private`. The two share a streaming function, so this
// is the assertion that stops the public route's change from having leaked into it.
func TestGlimtMedia_AuthenticatedRouteStaysPrivate(t *testing.T) {
	app, store, _ := publicApp(t)
	store.rows = []glimt.Glimt{storeRealBytes(t, app)}
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, _ := getPublic(t, srv.URL+"/api/glimt/items/g-public/media/0", cookies)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "private") {
		t.Errorf("Cache-Control = %q, want private — a shared cache keyed on URL alone would serve "+
			"one member's group-scoped photo to the next caller", cc)
	}
}

// The payload carries nothing personal. Asserted against the raw bytes rather than the decoded
// struct, because the thing to catch is a field somebody *added*, which a typed decode would ignore.
func TestPublicGlimt_CarriesNothingPersonal(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for _, path := range []string{"/api/public/glimt", "/2026/glimt"} {
		_, payload := getPublic(t, srv.URL+path, nil)
		body := string(payload)

		for _, forbidden := range []string{
			"author_person_id", "authorPersonId", "a-spejder", "a-crew",
			"person_id", "personId", "phone", "telefon", "phoneParent", "phone_parent",
			// The blob refs. A content hash in a public payload would be a forwardable,
			// unrevokable capability — worse here than anywhere else in the feature.
			refA, refB, refC,
		} {
			if strings.Contains(body, forbidden) {
				t.Errorf("%s leaks %q", path, forbidden)
			}
		}

		// The hold attribution *must* be there: a parent recognising their own child's patrulje is
		// the only thing that makes a public feed worth publishing (PRD 019 §0).
		if !strings.Contains(body, "42") {
			t.Errorf("%s carries no hold attribution, which is the point of the page", path)
		}
	}
}

// The page works without JavaScript, and every media item is reachable without a swipe.
//
// The maintainer's requirement (2026-09-18): the public page has to be usable on a desktop computer,
// so multiple media must be reachable **without a gesture**. The app's fix — prev/next buttons on an
// Embla carousel — cannot carry here, because there is no bundle. So: one img tag per item.
func TestPublicGlimtPage_RendersEveryMediaItemWithoutScript(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, payload := getPublic(t, srv.URL+"/2026/glimt", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want HTML", ct)
	}
	body := string(payload)

	// g-public has two items, and both must be rendered — not one plus a carousel.
	for _, ordinal := range []string{"/media/0", "/media/1"} {
		if !strings.Contains(body, "/api/public/glimt/g-public"+ordinal) {
			t.Errorf("item %s is not on the page; a glimt's media must all be reachable without a "+
				"swipe (task 323)", ordinal)
		}
	}

	// No script of any kind. A page that needs JavaScript for its media is a second code path that
	// only some visitors get — which is the reason this page is server-rendered at all.
	for _, forbidden := range []string{"<script", "onclick", "onload=", "javascript:"} {
		if strings.Contains(strings.ToLower(body), forbidden) {
			t.Errorf("the public page contains %q, but it must work with JavaScript disabled",
				forbidden)
		}
	}

	// And no carousel machinery smuggled in as CSS that needs a gesture to operate.
	if strings.Contains(body, "scroll-snap") {
		t.Error("the page uses scroll-snap, which needs a gesture — see task 323's analysis")
	}
}

func TestPublicGlimtPage_OmitsWhatIsNotPublic(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, payload := getPublic(t, srv.URL+"/2026/glimt", nil)
	body := string(payload)

	if strings.Contains(body, "kun os") {
		t.Error("a group-scoped caption is on the public page")
	}
	if strings.Contains(body, "g-public-hidden") || strings.Contains(body, "Ulvene") {
		t.Error("a hidden glimt is still on the public page")
	}
	if !strings.Contains(body, "ved posten") {
		t.Error("the public glimt's caption is missing")
	}
}

// A caption is participant-authored text on an unauthenticated page, so it must be escaped.
func TestPublicGlimtPage_EscapesCaptions(t *testing.T) {
	app, store, _ := publicApp(t)
	store.rows = []glimt.Glimt{{
		GlimtID: "g-x", AuthorGroup: "spejder", TeamNumber: "42", TeamName: "Ørnene",
		Audience: glimt.AudiencePublic, CreatedAt: time.Now().UTC(),
		Caption: `<script>alert(1)</script>`,
	}}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, payload := getPublic(t, srv.URL+"/2026/glimt", nil)
	if strings.Contains(string(payload), "<script>alert") {
		t.Error("a caption was rendered unescaped")
	}
}

// The public retention cutoff must be passed. A zero time there serves the whole archive to the open
// web, which is the failure this criterion was written against — and it is invisible, because the
// page looks perfect either way.
func TestPublicGlimt_PassesTheRetentionCutoff(t *testing.T) {
	app, store, _ := publicApp(t)
	app.config.glimtPublicRetention = 30 * 24 * time.Hour

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	getPublic(t, srv.URL+"/api/public/glimt", nil)

	if len(store.publicCutoffs) == 0 {
		t.Fatal("PublicFeed was not called")
	}
	if store.publicCutoffs[0].IsZero() {
		t.Error("PublicFeed was given a zero cutoff, which serves the entire archive to the open web")
	}
}

func TestPublicGlimt_NoCutoffWhenRetentionIsOff(t *testing.T) {
	app, store, _ := publicApp(t)
	// Zero means "as long as the glimt itself", not "immediately" — the reading that would
	// silently empty the public page in a dev environment where everything is set to 0.
	app.config.glimtPublicRetention = 0

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	getPublic(t, srv.URL+"/api/public/glimt", nil)

	if len(store.publicCutoffs) == 0 {
		t.Fatal("PublicFeed was not called")
	}
	if !store.publicCutoffs[0].IsZero() {
		t.Error("a cutoff was applied with retention disabled")
	}
}

func TestPublicReport_HidesAPublicGlimt(t *testing.T) {
	app, _, pub := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	// No cookies, deliberately: a parent who spots a problem has no session and will not make one.
	resp, _ := postGlimtJSON(t, srv.URL+"/api/public/glimt/g-public/report",
		reportGlimtRequest{Reason: "mit barn skal ikke være med"}, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	if len(pub.Subjects()) != 1 {
		t.Fatalf("published %d events, want 1", len(pub.Subjects()))
	}
}

// The reporter recorded is a sentinel, not an IP address.
//
// `glimt_report` is keyed by reporter, so an anonymous report needs *a* value. Recording the IP would
// put a personal identifier of the one participant in this feature who never agreed to anything into
// an append-only audit table, to solve a duplicate-counting problem that does not matter.
func TestPublicReport_RecordsNoIPAddress(t *testing.T) {
	app, _, pub := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	postGlimtJSON(t, srv.URL+"/api/public/glimt/g-public/report", reportGlimtRequest{}, nil)

	if len(pub.Messages) != 1 {
		t.Fatalf("published %d events, want 1", len(pub.Messages))
	}
	var reported glimt.Reported
	if err := pub.Messages[0].Body(&reported); err != nil {
		t.Fatalf("body: %v", err)
	}

	if reported.ReporterPersonID != publicReporterSentinel {
		t.Errorf("reporter = %q, want the anonymous sentinel %q",
			reported.ReporterPersonID, publicReporterSentinel)
	}
	// Belt and braces: nothing that looks like an address anywhere in the recorded reporter.
	for _, ip := range []string{"127.0.0.1", "::1", "[::1]", "192.168"} {
		if strings.Contains(reported.ReporterPersonID, ip) {
			t.Errorf("an IP address was recorded in the audit trail: %q", reported.ReporterPersonID)
		}
	}
}

// Only a public glimt is reportable here. Otherwise this is an unauthenticated way to take down a
// group-scoped photograph — and an unauthenticated way to discover that one exists.
func TestPublicReport_RefusesNonPublicGlimt(t *testing.T) {
	app, _, pub := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for _, id := range []string{"g-group", "g-nathejk", "g-missing"} {
		resp, _ := postGlimtJSON(t, srv.URL+"/api/public/glimt/"+id+"/report",
			reportGlimtRequest{}, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", id, resp.StatusCode)
		}
	}
	if len(pub.Subjects()) != 0 {
		t.Errorf("published %d events for glimt that are not public", len(pub.Subjects()))
	}
}

func TestPublicReport_IsRateLimitedByIP(t *testing.T) {
	app, _, _ := publicApp(t)
	app.publicReportLimiter = limiterOrNil(2, time.Hour)

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for i := 0; i < 2; i++ {
		resp, _ := postGlimtJSON(t, srv.URL+"/api/public/glimt/g-public/report",
			reportGlimtRequest{}, nil)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("report %d: status = %d, want 204", i+1, resp.StatusCode)
		}
	}
	resp, _ := postGlimtJSON(t, srv.URL+"/api/public/glimt/g-public/report",
		reportGlimtRequest{}, nil)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429 past the limit", resp.StatusCode)
	}
}

func TestPublicReport_RejectsALongReason(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, _ := postGlimtJSON(t, srv.URL+"/api/public/glimt/g-public/report",
		reportGlimtRequest{Reason: strings.Repeat("æ", maxPublicReportReason+1)}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

// The page must not be answered by the SPA fallback, which would serve index.html.
func TestPublicGlimtPage_IsNotTheSPAFallback(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, payload := getPublic(t, srv.URL+"/2026/glimt", nil)
	body := string(payload)
	if strings.Contains(body, "<div id=\"app\"") || strings.Contains(body, "/assets/index") {
		t.Error("the app shell was served instead of the public page")
	}
	if !strings.Contains(body, "Glimt fra Nathejk") {
		t.Errorf("this does not look like the public page: %.200s", body)
	}
}

func TestPublicHoldLabel(t *testing.T) {
	cases := []struct {
		name string
		hold holdAttribution
		want string
	}{
		{"spejder", holdAttribution{Number: "42", Name: "Ørnene", Group: "spejder"}, "Patrulje 42 · Ørnene"},
		{"bandit", holdAttribution{Number: "7", Name: "Nord", Group: "bandit"}, "Klan 7 · Nord"},
		// Crew have a section rather than a numbered hold: the bare name, with no group word, or
		// it would read "Crew Postmandskab".
		{"crew", holdAttribution{Name: "Postmandskab", Group: "crew"}, "Postmandskab"},
		{"no name", holdAttribution{Number: "42", Group: "spejder"}, "Patrulje 42"},
		// Must never fall back to nothing: an unattributed entry reads as though the page were
		// hiding who posted, when the truth is that we never knew.
		{"nothing known", holdAttribution{}, "Ukendt hold"},
	}
	for _, c := range cases {
		if got := publicHoldLabel(c.hold); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// Reads are limited by IP, since there is nobody to key on — and the ceiling has to be generous
// because an IP may be a whole school.
func TestPublicGlimtReads_AreBoundedButLoose(t *testing.T) {
	app, _, _ := publicApp(t)
	app.publicGlimtReadLimiter = limiterOrNil(3, time.Minute)

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for i := 0; i < 3; i++ {
		if resp, _ := getPublic(t, srv.URL+"/api/public/glimt", nil); resp.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("read %d was throttled inside the limit", i+1)
		}
	}
	if resp, _ := getPublic(t, srv.URL+"/api/public/glimt", nil); resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429 past the read limit", resp.StatusCode)
	}

	// The production default is much looser than the member-keyed one, for exactly this reason.
	cfg := config{glimtPublicReadsPerMinute: 3000, glimtReadsPerMinute: 600}
	if cfg.glimtPublicReadsPerMinute <= cfg.glimtReadsPerMinute {
		t.Error("the by-IP public read limit is not looser than the by-member one, though an IP " +
			"may legitimately be a whole school")
	}
}
