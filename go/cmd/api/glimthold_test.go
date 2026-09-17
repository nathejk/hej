package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nathejk.dk/nathejk/table/glimt"
)

// Hold collection tests (task 319).

// holdFixture spans two spejder holds, a bandit klan, and a crew section with no hold number.
func holdFixture() []glimt.Glimt {
	at := func(min int) time.Time { return time.Date(2026, 9, 17, 21, min, 0, 0, time.UTC) }
	return []glimt.Glimt{
		// Hold 42 — the caller's own, three glimt, deliberately out of chronological order in
		// the fixture so the handler's ordering is doing the work rather than the slice's.
		{GlimtID: "a-third", AuthorPersonID: "mock-spejder-1", AuthorGroup: "spejder",
			TeamNumber: "42", TeamName: "Ørnene", Audience: glimt.AudienceGroup, CreatedAt: at(30)},
		{GlimtID: "a-first", AuthorPersonID: "mock-spejder-1", AuthorGroup: "spejder",
			TeamNumber: "42", TeamName: "Ørnene", Audience: glimt.AudienceGroup, CreatedAt: at(10)},
		{GlimtID: "a-second", AuthorPersonID: "patrol-mate", AuthorGroup: "spejder",
			TeamNumber: "42", TeamName: "Ørnene", Audience: glimt.AudienceNathejk, CreatedAt: at(20)},

		// Hold 43 — another patrulje.
		{GlimtID: "b-1", AuthorPersonID: "other-spejder", AuthorGroup: "spejder",
			TeamNumber: "43", TeamName: "Ulvene", Audience: glimt.AudienceGroup, CreatedAt: at(15)},

		// A klan whose only glimt is group-scoped: invisible to a spejder, so the hold must not
		// appear in their index at all.
		{GlimtID: "klan-1", AuthorPersonID: "a-bandit", AuthorGroup: "bandit",
			TeamNumber: "7", TeamName: "Klan Nord", Audience: glimt.AudienceGroup, CreatedAt: at(5)},

		// Crew have a section rather than a numbered hold, so they are not a hold at all.
		{GlimtID: "crew-1", AuthorPersonID: "a-crew", AuthorGroup: "crew",
			TeamName: "PR", Audience: glimt.AudienceNathejk, CreatedAt: at(40)},
	}
}

func TestHoldIndex_ListsOnlyHoldsTheCallerCanSee(t *testing.T) {
	app, _, _ := glimtApp(t, holdFixture(), spejderPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, payload := readBody(t, srv.URL+"/api/glimt/hold", cookies)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d (body %s)", resp.StatusCode, payload)
	}

	var out glimtHoldsResponse
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	byNumber := map[string]glimtHoldSummary{}
	for _, h := range out.Holds {
		byNumber[h.Number] = h
	}

	if _, present := byNumber["7"]; present {
		t.Error("a klan whose only glimt is group-scoped appears in a spejder's index — that leaks that they posted")
	}
	for _, want := range []string{"42", "43"} {
		if _, present := byNumber[want]; !present {
			t.Errorf("hold %s is missing from the index", want)
		}
	}
	// Crew have no hold number, so they are not a hold.
	if _, present := byNumber[""]; present {
		t.Error("an empty hold number appeared in the index")
	}
}

// TestHoldIndex_CountsOnlyWhatTheCallerCanSee — a count including invisible glimt would tell a
// spejder how much another group had posted, which is a small leak and still one they were not meant
// to have.
func TestHoldIndex_CountsOnlyWhatTheCallerCanSee(t *testing.T) {
	rows := holdFixture()
	// Add a group-scoped bandit glimt to hold 43 — impossible in practice, but it isolates the
	// property: the count must not include rows the filter dropped.
	rows = append(rows, glimt.Glimt{
		GlimtID: "b-hidden-from-spejder", AuthorPersonID: "a-bandit", AuthorGroup: "bandit",
		TeamNumber: "43", TeamName: "Ulvene", Audience: glimt.AudienceGroup,
		CreatedAt: time.Date(2026, 9, 17, 21, 16, 0, 0, time.UTC),
	})

	app, _, _ := glimtApp(t, rows, spejderPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	_, payload := readBody(t, srv.URL+"/api/glimt/hold", cookies)
	var out glimtHoldsResponse
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, h := range out.Holds {
		if h.Number == "43" && h.Count != 1 {
			t.Errorf("hold 43 count = %d, want 1 — the count must reflect what the caller may see", h.Count)
		}
	}
}

// TestHoldIndex_CarriesTheCallersOwnNumber is the one-tap shortcut into the post-race browse.
func TestHoldIndex_CarriesTheCallersOwnNumber(t *testing.T) {
	app, _, _ := glimtApp(t, holdFixture(), spejderPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	_, payload := readBody(t, srv.URL+"/api/glimt/hold", cookies)
	var out glimtHoldsResponse
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.OwnNumber != "42" {
		t.Errorf("own_number = %q, want 42", out.OwnNumber)
	}
}

func TestHoldIndex_CrewGetNoOwnNumber(t *testing.T) {
	// Crew have a section rather than a numbered hold. The client omits the shortcut rather
	// than linking to a collection that cannot exist.
	crew := spejderPerson()
	crew.TeamNumber = ""
	crew.TeamName = ""
	crew.SectionName = "Postmandskab"

	app, _, _ := glimtApp(t, holdFixture(), crew)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	_, payload := readBody(t, srv.URL+"/api/glimt/hold", cookies)
	var out glimtHoldsResponse
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.OwnNumber != "" {
		t.Errorf("own_number = %q, want empty for crew", out.OwnNumber)
	}
}

// TestHoldCollection_IsOldestFirst is the deliberate asymmetry with the feed.
//
// A race reads forward in time: somebody reliving an evening wants it in the order it happened.
//
// Note what this can and cannot prove. The **authority** on ordering is the SQL, asserted directly in
// `nathejk/table/glimt/querier_test.go` — a handler test running against a stub cannot verify an
// ORDER BY. What this asserts is that the two endpoints are wired to the two different reads, and that
// nothing in the handler re-sorts on the way out. The fixture lists hold 42's glimt out of
// chronological order so a handler that passed rows straight through unsorted would still fail.
func TestHoldCollection_IsOldestFirst(t *testing.T) {
	app, _, _ := glimtApp(t, holdFixture(), spejderPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, payload := readBody(t, srv.URL+"/api/glimt/hold/42", cookies)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d (body %s)", resp.StatusCode, payload)
	}

	got := feedIDs(t, payload)
	want := []string{"a-first", "a-second", "a-third"}
	if len(got) != len(want) {
		t.Fatalf("collection = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("collection = %v, want %v (oldest first)", got, want)
		}
	}

	// And the feed is the other way round, from the same data. If a later change unifies them,
	// this is the pair of assertions that should stop it.
	_, feed := readBody(t, srv.URL+"/api/glimt/feed", cookies)
	feedOrder := feedIDs(t, feed)
	if len(feedOrder) < 2 {
		t.Fatalf("feed too short to compare: %v", feedOrder)
	}
	if feedOrder[0] == want[0] {
		t.Error("the feed and the hold collection are ordered the same way; they answer different questions")
	}
}

func TestHoldCollection_FiltersVisibility(t *testing.T) {
	// Asking for another group's hold returns only what was shared beyond that group — an empty
	// result here is a normal answer, not an error.
	app, _, _ := glimtApp(t, holdFixture(), spejderPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, payload := readBody(t, srv.URL+"/api/glimt/hold/7", cookies)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 with an empty collection", resp.StatusCode)
	}
	if got := feedIDs(t, payload); len(got) != 0 {
		t.Errorf("a spejder saw %v from a klan's group-scoped collection", got)
	}
}

func TestHoldCollection_ModeratorSeesEverything(t *testing.T) {
	app, _, _ := glimtApp(t, holdFixture(), teamPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	_, payload := readBody(t, srv.URL+"/api/glimt/hold/7", cookies)
	if got := feedIDs(t, payload); len(got) != 1 || got[0] != "klan-1" {
		t.Errorf("collection = %v, want the klan's group-scoped glimt", got)
	}
}

func TestHoldCollection_RequiresAuthAndANumber(t *testing.T) {
	app, _, _ := glimtApp(t, holdFixture(), spejderPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	anon, _ := readBody(t, srv.URL+"/api/glimt/hold/42", nil)
	if anon.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauthenticated = %d, want 401", anon.StatusCode)
	}
	anonIndex, _ := readBody(t, srv.URL+"/api/glimt/hold", nil)
	if anonIndex.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauthenticated index = %d, want 401", anonIndex.StatusCode)
	}
}

func TestHoldEndpoints_MissingProjectionIsUnavailable(t *testing.T) {
	app, _, _ := glimtApp(t, holdFixture(), spejderPerson())
	app.models = newModelsWithGlimt(&stubPeople{p: spejderPerson(), found: true}, nil)

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	for _, path := range []string{"/api/glimt/hold", "/api/glimt/hold/42"} {
		resp, _ := readBody(t, srv.URL+path, cookies)
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("%s = %d, want 503", path, resp.StatusCode)
		}
	}
}
