package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jrgensen/cqrs/cqrstest"

	"nathejk.dk/nathejk/table/glimt"
	"nathejk.dk/nathejk/table/person"
)

// Moderation API tests (task 308).
//
// The queue is the widest read in the service — no visibility filter at all — so the section check is
// the only thing between it and every photograph in the event. Most of this file is that check, from
// every angle.

// teamPerson is the caller with the Team-section assignment.
func teamPerson() person.Person {
	return person.Person{
		PersonID:    "mock-spejder-1",
		AppRole:     person.RoleSpejder,
		Name:        "Astrid",
		TeamNumber:  "42",
		TeamName:    "Ørnene",
		SectionSlug: person.SectionTeam,
	}
}

// mixedScopes is one glimt per audience plus a reported one, to prove the queue ignores scope.
func mixedScopes() []glimt.Glimt {
	at := func(min int) time.Time { return time.Date(2026, 9, 17, 21, min, 0, 0, time.UTC) }
	return []glimt.Glimt{
		{GlimtID: "g-bandit-group", AuthorPersonID: "a-bandit", AuthorGroup: "bandit",
			TeamNumber: "7", TeamName: "Klan Nord", Audience: glimt.AudienceGroup, CreatedAt: at(1)},
		{GlimtID: "g-nathejk", AuthorPersonID: "a-crew", AuthorGroup: "crew",
			TeamName: "PR", Audience: glimt.AudienceNathejk, CreatedAt: at(2)},
		{GlimtID: "g-reported", AuthorPersonID: "other-spejder", AuthorGroup: "spejder",
			TeamNumber: "43", TeamName: "Ulvene", Audience: glimt.AudiencePublic,
			CreatedAt: at(3), ReportCount: 2},
	}
}

func TestModerationQueue_RequiresTheTeamSection(t *testing.T) {
	// Every role, none of them in the Team section. The point is that no *role* grants this —
	// every Team member is `crew` as far as roles go, and so are the kitchen and PR.
	app, store, _ := glimtApp(t, mixedScopes(), spejderPerson())

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, _ := readBody(t, srv.URL+"/api/glimt/moderation", cookies)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	// And crucially the read never happened: the gate is before the query, so a refused
	// caller costs nothing and reaches nothing.
	if store.moderationCalls != 0 {
		t.Errorf("the queue was read %d times for a non-moderator", store.moderationCalls)
	}
}

func TestModerationQueue_UnauthenticatedIs401(t *testing.T) {
	app, store, _ := glimtApp(t, mixedScopes(), teamPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, _ := readBody(t, srv.URL+"/api/glimt/moderation", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
	if store.moderationCalls != 0 {
		t.Errorf("the queue was read for an unauthenticated caller")
	}
}

func TestModerationQueue_ShowsEveryScopeWithTheAuthor(t *testing.T) {
	app, _, _ := glimtApp(t, mixedScopes(), teamPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, payload := readBody(t, srv.URL+"/api/glimt/moderation", cookies)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", resp.StatusCode, payload)
	}

	var out glimtModerationResponse
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Glimt) != 3 {
		t.Fatalf("queue has %d entries, want 3 (every scope)", len(out.Glimt))
	}

	// A bandit group-scoped glimt is visible to a spejder in the Team section — which is the
	// whole override, and the reach PRD 019 §6 requires us to disclose in the UI.
	found := false
	for _, g := range out.Glimt {
		if g.ID == "g-bandit-group" {
			found = true
			if g.AuthorPersonID != "a-bandit" {
				t.Errorf("no author on the queue entry: %+v", g)
			}
		}
	}
	if !found {
		t.Error("the queue omitted a group-scoped glimt from another group")
	}

	// This is the one response in the API that carries an author.
	if !strings.Contains(string(payload), "author_person_id") {
		t.Errorf("the queue must carry authors, or a report cannot be answered: %s", payload)
	}
}

// TestModerationQueue_ResolvesTheAuthorName — a bare person id is not something a human moderating at
// 03:00 can act on.
func TestModerationQueue_ResolvesTheAuthorName(t *testing.T) {
	app, store, _ := glimtApp(t, nil, teamPerson())
	store.rows = []glimt.Glimt{{
		GlimtID: "g-1", AuthorPersonID: "mock-spejder-1", AuthorGroup: "spejder",
		Audience: glimt.AudienceGroup, CreatedAt: time.Now().UTC(),
	}}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	_, payload := readBody(t, srv.URL+"/api/glimt/moderation", cookies)
	var out glimtModerationResponse
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Glimt) != 1 || out.Glimt[0].AuthorName != "Astrid" {
		t.Errorf("author name not resolved: %+v", out.Glimt)
	}
}

func TestModerationQueue_IsNotCached(t *testing.T) {
	// A stale queue would have a moderator reviewing something already handled — or worse,
	// believing a reported glimt is still up.
	app, _, _ := glimtApp(t, mixedScopes(), teamPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, _ := readBody(t, srv.URL+"/api/glimt/moderation", cookies)
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
}

func TestHideAndUnhide_PublishTheEvents(t *testing.T) {
	app, _, pub := glimtApp(t, mixedScopes(), teamPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, body := postGlimtJSON(t, srv.URL+"/api/glimt/items/g-reported/hide",
		moderateGlimtRequest{Reason: "ikke ok"}, cookies)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("hide = %d, want 204 (body %s)", resp.StatusCode, body)
	}
	if got := pub.Subjects(); len(got) != 1 || got[0] != "NATHEJK.2026.glimt.g-reported.hidden" {
		t.Fatalf("subjects = %v", got)
	}
	var hidden glimt.Hidden
	if err := pub.Messages[0].Body(&hidden); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Attributable to a person, not to "the system": a takedown of a participant's photograph
	// is a decision somebody made.
	if hidden.HiddenBy != "mock-spejder-1" {
		t.Errorf("hiddenBy = %q", hidden.HiddenBy)
	}
	if hidden.Reason != "ikke ok" || hidden.HiddenAt.IsZero() {
		t.Errorf("hidden event = %+v", hidden)
	}

	resp, _ = postGlimtJSON(t, srv.URL+"/api/glimt/items/g-reported/unhide",
		moderateGlimtRequest{}, cookies)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("unhide = %d, want 204", resp.StatusCode)
	}
	if got := pub.Subjects(); len(got) != 2 || got[1] != "NATHEJK.2026.glimt.g-reported.unhidden" {
		t.Fatalf("subjects = %v", got)
	}
}

// TestHideAndUnhide_LeaveTheMediaAlone is the difference from a delete, and what makes hiding cheap
// enough to be the automatic response to a report.
func TestHideAndUnhide_LeaveTheMediaAlone(t *testing.T) {
	app, store, _ := glimtApp(t, nil, teamPerson())
	row := mediaGlimt(t, app, "g-1", "other-spejder", "spejder", glimt.AudiencePublic)
	store.rows = []glimt.Glimt{row}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, _ := postGlimtJSON(t, srv.URL+"/api/glimt/items/g-1/hide", moderateGlimtRequest{}, cookies)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("hide = %d", resp.StatusCode)
	}
	for name, ref := range map[string]string{"full": row.Media[0].Ref, "thumb": row.Media[0].ThumbRef} {
		if ok, _ := app.blobs.Exists(t.Context(), blobRefOf(ref)); !ok {
			t.Errorf("hiding deleted the %s bytes; an unhide would be impossible", name)
		}
	}
}

func TestHideAndUnhide_RequireTheTeamSection(t *testing.T) {
	for _, action := range []string{"hide", "unhide"} {
		t.Run(action, func(t *testing.T) {
			app, _, pub := glimtApp(t, mixedScopes(), spejderPerson())
			srv := httptest.NewServer(app.routes())
			defer srv.Close()
			cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

			resp, _ := postGlimtJSON(t, srv.URL+"/api/glimt/items/g-reported/"+action,
				moderateGlimtRequest{}, cookies)
			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("status = %d, want 403", resp.StatusCode)
			}
			if len(pub.Subjects()) != 0 {
				t.Error("a refused moderation action published an event")
			}
		})
	}
}

// TestModeration_RevocationTakesEffectImmediately is the property the per-request lookup exists for.
//
// The same authenticated session, with the assignment taken away between two calls. A session claim
// could not do this: it would keep working for the cookie's whole life, and the person who revoked
// the assignment would have no way to make it stop.
func TestModeration_RevocationTakesEffectImmediately(t *testing.T) {
	people := &stubPeople{p: teamPerson(), found: true}
	app := photoTestApp(t, nil, people)
	app.config.eventYear = "2026"
	store := &stubGlimt{rows: mixedScopes()}
	app.models = newModelsWithGlimt(people, store)

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	allowed, _ := readBody(t, srv.URL+"/api/glimt/moderation", cookies)
	if allowed.StatusCode != http.StatusOK {
		t.Fatalf("with the assignment = %d, want 200", allowed.StatusCode)
	}

	// Reassigned out of the Team section. Same cookie, same session.
	revoked := teamPerson()
	revoked.SectionSlug = "koekken"
	people.p = revoked

	refused, _ := readBody(t, srv.URL+"/api/glimt/moderation", cookies)
	if refused.StatusCode != http.StatusForbidden {
		t.Errorf("after revocation = %d, want 403 — the same session must lose the capability",
			refused.StatusCode)
	}
}

func TestModeration_UnknownGlimtIs404(t *testing.T) {
	// Confirmed to exist before publishing, so a typo'd id cannot put an event on the log about
	// nothing — which a replay would then apply to a glimt created later with that id.
	app, _, pub := glimtApp(t, mixedScopes(), teamPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, _ := postGlimtJSON(t, srv.URL+"/api/glimt/items/never-existed/hide",
		moderateGlimtRequest{}, cookies)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	if len(pub.Subjects()) != 0 {
		t.Error("an event was published for a glimt that does not exist")
	}
}

func TestModeration_RejectsALongReason(t *testing.T) {
	app, _, _ := glimtApp(t, mixedScopes(), teamPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, _ := postGlimtJSON(t, srv.URL+"/api/glimt/items/g-reported/hide",
		moderateGlimtRequest{Reason: strings.Repeat("æ", maxGlimtModerationReason+1)}, cookies)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestModeration_NoPublisherIs503(t *testing.T) {
	app, _, _ := glimtApp(t, mixedScopes(), teamPerson())
	app.commands = commandsWithNoPublisher()
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, _ := postGlimtJSON(t, srv.URL+"/api/glimt/items/g-reported/hide",
		moderateGlimtRequest{}, cookies)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
}

// GET /api/me carries `moderates_glimt` so the client knows whether to draw the moderation entry
// (task 309).
//
// The client cannot derive this: moderation comes from `sectionSlug`, which is deliberately kept out
// of the session and never sent to the client. The alternative — having the client probe
// /api/glimt/moderation and read the 403 — would cost every participant a refused request per
// foreground, which is the exact mistake the contacts prefetch made (see `hasContactsPane`).
func TestMe_ReportsGlimtModeration(t *testing.T) {
	app, _, _ := glimtApp(t, nil, teamPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp, payload := readBody(t, srv.URL+"/api/me", cookies)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", resp.StatusCode, payload)
	}
	var out identityResponse
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.ModeratesGlimt {
		t.Errorf("a Team-section member is not told they moderate: %s", payload)
	}
}

func TestMe_DoesNotReportModerationForEveryoneElse(t *testing.T) {
	app, _, _ := glimtApp(t, nil, spejderPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	_, payload := readBody(t, srv.URL+"/api/me", cookies)
	var out identityResponse
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.ModeratesGlimt {
		t.Errorf("a spejder outside the Team section is told they moderate: %s", payload)
	}
	// Present-and-false rather than absent: a client must not have to distinguish "not a
	// moderator" from "this build does not implement it".
	if !strings.Contains(string(payload), "moderates_glimt") {
		t.Errorf("moderates_glimt omitted when false, so false and unimplemented look alike: %s", payload)
	}
}

// The field is a hint for drawing a link, never the permission. If someone ever wires the client gate
// to something cached, this is the test that says the endpoint does not care.
func TestMe_ModerationFieldIsNotThePermission(t *testing.T) {
	people := &stubPeople{p: teamPerson(), found: true}
	app := photoTestApp(t, &cqrstest.Publisher{}, people)
	app.config.eventYear = "2026"
	store := &stubGlimt{rows: mixedScopes()}
	app.models = newModelsWithGlimt(people, store)

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	if resp, _ := readBody(t, srv.URL+"/api/glimt/moderation", cookies); resp.StatusCode != http.StatusOK {
		t.Fatalf("Team member refused the queue: %d", resp.StatusCode)
	}

	// Assignment taken away. No new login, no new /api/me — the very next request must fail,
	// whatever the client was last told.
	people.p = person.Person{PersonID: "mock-spejder-1", AppRole: person.RoleSpejder, SectionSlug: "koekken"}
	if resp, _ := readBody(t, srv.URL+"/api/glimt/moderation", cookies); resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403 after the Team assignment was revoked", resp.StatusCode)
	}
}
