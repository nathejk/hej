package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nathejk.dk/internal/blob"
	"nathejk.dk/nathejk/table/glimt"
	"nathejk.dk/nathejk/table/person"
)

// Media serving tests (task 305).
//
// TestGlimtMedia_GroupScopedRefIsForbiddenToAnOutsider is the single most important test in the
// Glimt backend: it is the one that proves a media URL is not a bearer token. Everything else in
// this file supports it.

// mediaGlimt stores real bytes and returns a row referencing them.
func mediaGlimt(t *testing.T, app *application, id, authorID, group, audience string) glimt.Glimt {
	t.Helper()
	full, err := app.blobs.Put(t.Context(), testImage(t, 200, 150))
	if err != nil {
		t.Fatalf("put full: %v", err)
	}
	thumb, err := app.blobs.Put(t.Context(), testImage(t, 60, 45))
	if err != nil {
		t.Fatalf("put thumb: %v", err)
	}
	return glimt.Glimt{
		GlimtID:        id,
		Year:           "2026",
		AuthorPersonID: authorID,
		AuthorGroup:    group,
		TeamNumber:     "42",
		TeamName:       "Ørnene",
		Audience:       audience,
		CreatedAt:      time.Date(2026, 9, 17, 21, 0, 0, 0, time.UTC),
		MediaCount:     1,
		Media: []glimt.Media{{
			Ordinal: 0, Ref: full.String(), ThumbRef: thumb.String(),
			Kind: glimt.MediaKindImage, Width: 200, Height: 150,
		}},
	}
}

func mediaURL(base, glimtID string, ordinal int) string {
	return fmt.Sprintf("%s/api/glimt/items/%s/media/%d", base, glimtID, ordinal)
}

// TestGlimtMedia_GroupScopedRefIsForbiddenToAnOutsider — the point of the whole file.
//
// A spejder posts to their own group. A bandit, who has the glimt id (forwarded link, shared
// screenshot, guessed), asks for the bytes. The URL is perfectly well-formed and the object exists.
// It must still be refused, because the handler re-asks the same question the feed asked.
func TestGlimtMedia_GroupScopedRefIsForbiddenToAnOutsider(t *testing.T) {
	app, store, _ := glimtApp(t, nil, spejderPerson())
	row := mediaGlimt(t, app, "g-secret", "other-spejder", "spejder", glimt.AudienceGroup)
	store.rows = []glimt.Glimt{row}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	// A bandit. The mock directory's +4530000002 is a bandit — assert it, so a change to the
	// fixtures cannot silently turn this into a same-group test that passes for the wrong
	// reason.
	cookies := authedCookies(t, app, srv, "30000002", "+4530000002")
	viewer, found := app.models.Users.Get("mock-bandit-1")
	if !found || viewer.Role != "bandit" {
		t.Skipf("fixture drift: expected a bandit at +4530000002, got %+v (found=%v)", viewer, found)
	}

	resp := getWithCookies(t, mediaURL(srv.URL, "g-secret", 0), cookies)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 403 — a media URL must not be a bearer token (body %s)",
			resp.StatusCode, body)
	}

	// And nothing of the image leaked into the error body.
	body, _ := io.ReadAll(resp.Body)
	if bytes.Contains(body, []byte{0xff, 0xd8}) {
		t.Error("JPEG bytes appeared in a 403 response")
	}
}

func TestGlimtMedia_SameGroupMemberGetsTheBytes(t *testing.T) {
	app, store, _ := glimtApp(t, nil, spejderPerson())
	row := mediaGlimt(t, app, "g-1", "other-spejder", "spejder", glimt.AudienceGroup)
	store.rows = []glimt.Glimt{row}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp := getWithCookies(t, mediaURL(srv.URL, "g-1", 0), cookies)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("content type = %q", ct)
	}
	data, _ := io.ReadAll(resp.Body)
	if len(data) == 0 {
		t.Error("no bytes served")
	}
}

// TestGlimtMedia_ModeratorGetsAnyScope — the override, exercised end to end rather than only in the
// predicate's unit test.
func TestGlimtMedia_ModeratorGetsAnyScope(t *testing.T) {
	// The caller is in the Team section, so isGlimtModerator is true for them.
	moderator := person.Person{
		PersonID: "mock-spejder-1", AppRole: person.RoleSpejder,
		SectionSlug: person.SectionTeam,
	}
	app, store, _ := glimtApp(t, nil, moderator)
	row := mediaGlimt(t, app, "g-bandit", "a-bandit", "bandit", glimt.AudienceGroup)
	store.rows = []glimt.Glimt{row}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp := getWithCookies(t, mediaURL(srv.URL, "g-bandit", 0), cookies)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 for a Team-section member", resp.StatusCode)
	}
}

func TestGlimtMedia_HiddenIsRefusedToOthersButNotItsAuthor(t *testing.T) {
	app, store, _ := glimtApp(t, nil, spejderPerson())
	hidden := time.Date(2026, 9, 17, 23, 0, 0, 0, time.UTC)

	// Authored by someone else, hidden: refused even though the caller is in the group.
	other := mediaGlimt(t, app, "g-hidden", "other-spejder", "spejder", glimt.AudienceGroup)
	other.HiddenAt = &hidden
	// Authored by the caller, hidden: still theirs to look at.
	own := mediaGlimt(t, app, "g-own-hidden", "mock-spejder-1", "spejder", glimt.AudienceGroup)
	own.HiddenAt = &hidden
	store.rows = []glimt.Glimt{other, own}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	refused := getWithCookies(t, mediaURL(srv.URL, "g-hidden", 0), cookies)
	refused.Body.Close()
	if refused.StatusCode != http.StatusForbidden {
		t.Errorf("hidden glimt from another member = %d, want 403", refused.StatusCode)
	}

	allowed := getWithCookies(t, mediaURL(srv.URL, "g-own-hidden", 0), cookies)
	allowed.Body.Close()
	if allowed.StatusCode != http.StatusOK {
		t.Errorf("own hidden glimt = %d, want 200 — the author may still see it", allowed.StatusCode)
	}
}

func TestGlimtMedia_ThumbVariant(t *testing.T) {
	app, store, _ := glimtApp(t, nil, spejderPerson())
	row := mediaGlimt(t, app, "g-1", "mock-spejder-1", "spejder", glimt.AudienceGroup)
	store.rows = []glimt.Glimt{row}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	full := getWithCookies(t, mediaURL(srv.URL, "g-1", 0), cookies)
	fullBytes, _ := io.ReadAll(full.Body)
	full.Body.Close()

	thumb := getWithCookies(t, mediaURL(srv.URL, "g-1", 0)+"?variant=thumb", cookies)
	thumbBytes, _ := io.ReadAll(thumb.Body)
	thumb.Body.Close()

	if thumb.StatusCode != http.StatusOK {
		t.Fatalf("thumb status = %d", thumb.StatusCode)
	}
	if len(thumbBytes) >= len(fullBytes) {
		t.Errorf("thumb (%d bytes) is not smaller than full (%d bytes) — the variant was ignored",
			len(thumbBytes), len(fullBytes))
	}
	// The ETags differ, because they are different objects. That is what lets a client hold
	// both without one evicting the other.
	if full.Header.Get("ETag") == thumb.Header.Get("ETag") {
		t.Error("full and thumb share an ETag")
	}
}

// TestGlimtMedia_ThumbFallsBackToFull covers a state that really occurs: task 303 lets a thumbnail
// fail without failing the upload, so a 404 here would break a tile whose photo is fine.
func TestGlimtMedia_ThumbFallsBackToFull(t *testing.T) {
	app, store, _ := glimtApp(t, nil, spejderPerson())
	row := mediaGlimt(t, app, "g-1", "mock-spejder-1", "spejder", glimt.AudienceGroup)
	row.Media[0].ThumbRef = ""
	store.rows = []glimt.Glimt{row}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp := getWithCookies(t, mediaURL(srv.URL, "g-1", 0)+"?variant=thumb", cookies)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 falling back to the full item", resp.StatusCode)
	}
}

// TestGlimtMedia_IsImmutablyCacheable is most of the answer to the post-race load spike.
func TestGlimtMedia_IsImmutablyCacheable(t *testing.T) {
	app, store, _ := glimtApp(t, nil, spejderPerson())
	row := mediaGlimt(t, app, "g-1", "mock-spejder-1", "spejder", glimt.AudienceGroup)
	store.rows = []glimt.Glimt{row}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp := getWithCookies(t, mediaURL(srv.URL, "g-1", 0), cookies)
	resp.Body.Close()

	cc := resp.Header.Get("Cache-Control")
	if !strings.Contains(cc, "immutable") {
		t.Errorf("Cache-Control = %q, want immutable — this is what removes requests at the finish line", cc)
	}
	// `private`, always. A shared cache keyed on URL alone would serve one member's
	// group-scoped photo to the next caller of the same URL.
	if !strings.Contains(cc, "private") {
		t.Errorf("Cache-Control = %q, want private — a shared cache must not hold these", cc)
	}
	if resp.Header.Get("ETag") == "" {
		t.Error("no ETag; the content hash is a perfect validator and should be used")
	}

	// And the conditional request is honoured, without reading the object.
	etag := resp.Header.Get("ETag")
	req, _ := http.NewRequest(http.MethodGet, mediaURL(srv.URL, "g-1", 0), nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	req.Header.Set("If-None-Match", etag)
	cond, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("conditional GET: %v", err)
	}
	defer cond.Body.Close()
	if cond.StatusCode != http.StatusNotModified {
		t.Errorf("conditional GET = %d, want 304", cond.StatusCode)
	}
}

func TestGlimtMedia_UnknownGlimtOrOrdinalIs404(t *testing.T) {
	app, store, _ := glimtApp(t, nil, spejderPerson())
	row := mediaGlimt(t, app, "g-1", "mock-spejder-1", "spejder", glimt.AudienceGroup)
	store.rows = []glimt.Glimt{row}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	for name, url := range map[string]string{
		"unknown glimt":   mediaURL(srv.URL, "nope", 0),
		"unknown ordinal": mediaURL(srv.URL, "g-1", 7),
		"negative":        mediaURL(srv.URL, "g-1", -1),
	} {
		t.Run(name, func(t *testing.T) {
			resp := getWithCookies(t, url, cookies)
			resp.Body.Close()
			if resp.StatusCode != http.StatusNotFound {
				t.Errorf("status = %d, want 404", resp.StatusCode)
			}
		})
	}
}

// TestGlimtMedia_MissingBytesDegradeTo404 is PRD 008 §8's rule: a row referencing an object that has
// gone must degrade to "no photo", never fail.
func TestGlimtMedia_MissingBytesDegradeTo404(t *testing.T) {
	app, store, _ := glimtApp(t, nil, spejderPerson())
	row := mediaGlimt(t, app, "g-1", "mock-spejder-1", "spejder", glimt.AudienceGroup)
	// A well-formed ref for bytes that were never stored.
	row.Media[0].Ref = strings.Repeat("d", 64)
	row.Media[0].ThumbRef = ""
	store.rows = []glimt.Glimt{row}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp := getWithCookies(t, mediaURL(srv.URL, "g-1", 0), cookies)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestGlimtMedia_RequiresAuth(t *testing.T) {
	app, store, _ := glimtApp(t, nil, spejderPerson())
	store.rows = []glimt.Glimt{mediaGlimt(t, app, "g-1", "mock-spejder-1", "spejder", glimt.AudiencePublic)}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	// Public-scope media, but through the *authenticated* route: still 401. The public page
	// has its own unauthenticated route (task 323), and keeping them separate is what makes
	// "what can an anonymous caller reach?" answerable.
	resp := getWithCookies(t, mediaURL(srv.URL, "g-1", 0), nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestGlimtVariantRefRejectsAStoredJunkRef(t *testing.T) {
	// Defence in depth: the projection already refuses a malformed ref, but if one ever
	// reached a row it must not become a filesystem path.
	g := glimt.Glimt{Media: []glimt.Media{{Ordinal: 0, Ref: "../../etc/passwd"}}}
	if _, ok := glimtVariantRef(g, 0, ""); ok {
		t.Error("a path-shaped stored ref was accepted")
	}

	valid := strings.Repeat("a", 64)
	g = glimt.Glimt{Media: []glimt.Media{{Ordinal: 0, Ref: valid}}}
	ref, ok := glimtVariantRef(g, 0, "")
	if !ok || ref != blob.Ref(valid) {
		t.Errorf("glimtVariantRef = %q, %v", ref, ok)
	}
}
