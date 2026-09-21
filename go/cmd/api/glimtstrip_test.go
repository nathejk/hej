package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nathejk.dk/nathejk/table/glimt"
)

// The frontpage glimt strip (task 336).
//
// The strip itself was built with the frontpage shell (task 332), which already asserts that it shows
// only publicly visible glimt, uses lazy thumbnails, and carries alt text attributing the hold. What is
// here is the part that distinguishes "surfaced" from "reimplemented" — the properties that would break
// silently if somebody wrote a second query for the strip.

// **The retention window must be inherited, not re-expressed.** A zero cutoff serves the entire archive
// to the open web, and the strip is exactly the copy nobody would think to check for it.
func TestFrontpageStripAppliesThePublicRetentionCutoff(t *testing.T) {
	app, store, _ := publicApp(t)
	app.config.glimtPublicRetention = 30 * 24 * time.Hour

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	getPublic(t, srv.URL+"/2026", nil)

	if len(store.publicCutoffs) == 0 {
		t.Fatal("the frontpage did not read the public feed at all")
	}
	if store.publicCutoffs[0].IsZero() {
		t.Error("the frontpage read the feed with a zero cutoff, which serves the entire archive " +
			"to the open web")
	}
}

// And zero retention means "as long as the glimt itself", not "immediately" — the reading that would
// silently empty the page in a dev environment where every retention value is 0.
func TestFrontpageStripHasNoCutoffWhenRetentionIsOff(t *testing.T) {
	app, store, _ := publicApp(t)
	app.config.glimtPublicRetention = 0

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	getPublic(t, srv.URL+"/2026", nil)

	if len(store.publicCutoffs) == 0 {
		t.Fatal("the frontpage did not read the public feed at all")
	}
	if !store.publicCutoffs[0].IsZero() {
		t.Error("a cutoff was applied with retention disabled")
	}
}

// An expired glimt must be absent from the strip *and* from the full page. Two surfaces, one rule —
// which is the whole point of sharing `publicGlimt` rather than writing a second query.
func TestExpiredGlimtIsAbsentFromBothPublicSurfaces(t *testing.T) {
	app, store, _ := publicApp(t)
	app.config.glimtPublicRetention = 24 * time.Hour

	// One fresh, one long expired. The stub applies the cutoff it is given, so this exercises the real
	// filter rather than a stub shortcut.
	store.rows = []glimt.Glimt{
		{GlimtID: "g-fresh", AuthorPersonID: "a", AuthorGroup: "spejder",
			TeamNumber: "42", TeamName: "Ørnene", Audience: glimt.AudiencePublic,
			CreatedAt: time.Now().UTC().Add(-time.Hour),
			Media:     []glimt.Media{{Ordinal: 0, Ref: refA, ThumbRef: refB, Kind: "image"}}},
		{GlimtID: "g-expired", AuthorPersonID: "a", AuthorGroup: "spejder",
			TeamNumber: "43", TeamName: "Ulvene", Audience: glimt.AudiencePublic,
			CreatedAt: time.Now().UTC().Add(-90 * 24 * time.Hour),
			Media:     []glimt.Media{{Ordinal: 0, Ref: refC, ThumbRef: refB, Kind: "image"}}},
	}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for _, path := range []string{"/2026", "/2026/glimt"} {
		_, body := getPublic(t, srv.URL+path, nil)
		page := string(body)
		if !strings.Contains(page, "g-fresh") {
			t.Errorf("%s: the fresh glimt should be shown", path)
		}
		if strings.Contains(page, "g-expired") {
			t.Errorf("%s: an expired glimt must not be shown", path)
		}
	}
}

// A hidden glimt too, on both surfaces. Hiding is the safety mechanism behind an anonymous report
// (PRD 019), so a photograph somebody objected to appearing on the frontpage would defeat it.
func TestHiddenGlimtIsAbsentFromBothPublicSurfaces(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for _, path := range []string{"/2026", "/2026/glimt"} {
		_, body := getPublic(t, srv.URL+path, nil)
		if strings.Contains(string(body), "g-public-hidden") {
			t.Errorf("%s: a hidden glimt must not be shown", path)
		}
	}
}

// The strip is a taste with a link, not a second copy of the feed. Bounded so the frontpage does not
// become the page it links to.
func TestFrontpageStripIsBoundedAndLinksToTheFullPage(t *testing.T) {
	app, store, _ := publicApp(t)

	// More public glimt than the strip shows.
	var rows []glimt.Glimt
	for i := 0; i < publicFrontpageGlimtLimit+5; i++ {
		rows = append(rows, glimt.Glimt{
			GlimtID: "g-" + strings.Repeat("x", i+1), AuthorPersonID: "a", AuthorGroup: "spejder",
			TeamNumber: "42", TeamName: "Ørnene", Audience: glimt.AudiencePublic,
			CreatedAt: time.Now().UTC().Add(-time.Duration(i) * time.Minute),
			Media:     []glimt.Media{{Ordinal: 0, Ref: refA, ThumbRef: refB, Kind: "image"}},
		})
	}
	store.rows = rows

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	_, body := getPublic(t, srv.URL+"/2026", nil)
	page := string(body)

	if got := strings.Count(page, "variant=thumb"); got > publicFrontpageGlimtLimit {
		t.Errorf("the strip shows %d thumbnails, more than its %d limit", got, publicFrontpageGlimtLimit)
	}
	if !strings.Contains(page, `href="/2026/glimt"`) {
		t.Error("the strip must link to the full page")
	}
	if !strings.Contains(page, "Se alle glimt") {
		t.Error("want a visible link, not just an href")
	}
}

// Attribution is to the hold, never to a person — PRD 019's rule, and this surface's whole privacy
// claim (task 337). The strip renders `publicHoldLabel`, so a person's name has nowhere to come from,
// but the alt text is where one would appear if somebody changed it.
func TestFrontpageStripAttributesTheHoldAndNeverAPerson(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026", nil)
	page := string(body)

	if !strings.Contains(page, "Glimt fra Patrulje 42") {
		t.Errorf("want the hold label in the alt text\n%s", page)
	}
	// The fixture's author ids, which are the only person-shaped values in the source data.
	for _, forbidden := range []string{"a-spejder", "a-crew", "authorPersonId"} {
		if strings.Contains(page, forbidden) {
			t.Errorf("the strip leaks %q: a glimt is attributed to its hold, never to a person", forbidden)
		}
	}
}

// **Public glimt carry no location and must never be plotted.** A member tapping "Offentligt" agreed to
// share a photograph, not a position (PRD 011 §6, §11 Q7). Asserted here rather than left to task 342,
// because the map's own tests will be about what it *does* draw — this is about what may never reach it.
func TestPublicGlimtCarryNoPositionAnywhere(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for _, path := range []string{"/2026", "/2026/glimt", "/api/public/glimt"} {
		_, body := getPublic(t, srv.URL+path, nil)
		page := strings.ToLower(string(body))
		for _, forbidden := range []string{"latitude", "longitude", `"lat"`, `"lng"`, "coordinate"} {
			if strings.Contains(page, forbidden) {
				t.Errorf("%s carries %q: a public glimt must never carry a position", path, forbidden)
			}
		}
	}

	// And the map's own data source must never grow a glimt branch. That endpoint is task 342's, so it
	// may not exist yet — in which case this asserts nothing, and says so rather than passing quietly.
	// Once 342 lands it becomes a real guard without anybody having to remember to add one.
	resp, body := getPublic(t, srv.URL+"/api/public/albums", nil)
	if resp.StatusCode == http.StatusNotFound {
		t.Log("the album map source does not exist yet (task 342); the map half of this test is inert")
		return
	}
	for _, forbidden := range []string{"g-public", "glimt"} {
		if strings.Contains(string(body), forbidden) {
			t.Errorf("the album map source mentions %q; glimt must have no route to the map", forbidden)
		}
	}
}

// The empty state matches the existing public page's wording rather than inventing a second phrasing
// for the same fact.
func TestFrontpageStripEmptyStateMatchesTheGlimtPage(t *testing.T) {
	app, store, _ := publicApp(t)
	store.rows = nil

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	const wording = "Der er ikke delt nogen offentlige billeder endnu."
	for _, path := range []string{"/2026", "/2026/glimt"} {
		_, body := getPublic(t, srv.URL+path, nil)
		if !strings.Contains(string(body), wording) {
			t.Errorf("%s: want the shared wording %q", path, wording)
		}
	}
}

// The strip must not require script, like the rest of the surface.
func TestFrontpageStripNeedsNoScript(t *testing.T) {
	app, _, _ := publicApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, body := getPublic(t, srv.URL+"/2026", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if strings.Contains(strings.ToLower(string(body)), "<script") {
		t.Error("the frontpage must work with JavaScript disabled")
	}
}
