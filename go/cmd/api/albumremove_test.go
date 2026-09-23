package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jrgensen/cqrs/cqrstest"

	"nathejk.dk/internal/blob"
)

// Curator removal (task 335). Two properties carry the weight: only the Team section may remove, and a
// removal must not delete bytes something else still shows.

// removalApp is an album app whose caller does or does not hold the Team section.
//
// Built from `glimtApp` rather than `albumApp` because the moderation check reads the caller's person
// record, and that is the seam `glimtApp` parameterises — the same one glimtmoderation_test.go uses.
func removalApp(t *testing.T, moderator bool) (*application, *albumStore, *cqrstest.Publisher, *httptest.Server, []*http.Cookie) {
	t.Helper()

	caller := spejderPerson()
	if moderator {
		caller = teamPerson()
	}
	app, glimtStore, pub := glimtApp(t, publicGlimtRows(), caller)
	app.publicGlimtReadLimiter = nil

	store := seedAlbums(t, app)
	app.models.Albums = store
	app.models.Glimt = glimtStore

	srv := httptest.NewServer(app.routes())
	t.Cleanup(srv.Close)
	return app, store, pub, srv, authedCookies(t, app, srv, "30000001", "+4530000001")
}

func deleteAs(t *testing.T, srv *httptest.Server, path string, cookies []*http.Cookie, body string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodDelete, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", path, err)
	}
	resp.Body.Close()
	return resp
}

// **The authorization property.** Removal is the one write on this surface, and it is the Team
// section's alone — reusing the glimt moderation check rather than inventing a second notion of curator.
func TestAlbumRemovalRequiresTheTeamSection(t *testing.T) {
	_, _, _, srv, cookies := removalApp(t, false)

	for _, path := range []string{"/api/albums/al-1/items/0", "/api/albums/al-1"} {
		if resp := deleteAs(t, srv, path, cookies, ""); resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s: want 403 without the Team section, got %d", path, resp.StatusCode)
		}
	}
}

func TestAlbumRemovalRequiresASession(t *testing.T) {
	_, _, _, srv, _ := removalApp(t, true)

	for _, path := range []string{"/api/albums/al-1/items/0", "/api/albums/al-1"} {
		if resp := deleteAs(t, srv, path, nil, ""); resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s: want 401 without a session, got %d", path, resp.StatusCode)
		}
	}
}

func TestRemoveAlbumItemPublishesTheEvent(t *testing.T) {
	_, _, pub, srv, cookies := removalApp(t, true)

	if resp := deleteAs(t, srv, "/api/albums/al-1/items/0", cookies,
		`{"reason":"forælder har bedt om det"}`); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", resp.StatusCode)
	}

	subjects := pub.Subjects()
	if len(subjects) != 1 {
		t.Fatalf("want one event, got %v", subjects)
	}
	if subjects[0] != "NATHEJK.2026.album.al-1.itemremoved" {
		t.Errorf("unexpected subject %q", subjects[0])
	}
}

// A removal of something that does not exist must not put a no-op on an append-only log.
func TestRemoveAlbumItemRefusesUnknownTargets(t *testing.T) {
	_, _, pub, srv, cookies := removalApp(t, true)

	for _, path := range []string{
		"/api/albums/al-1/items/99",
		"/api/albums/al-1/items/x",
		"/api/albums/findes-ikke/items/0",
		// A draft album's items are not removable here; the whole album can be deleted instead.
		"/api/albums/al-draft/items/0",
	} {
		if resp := deleteAs(t, srv, path, cookies, ""); resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: want 404, got %d", path, resp.StatusCode)
		}
	}
	if len(pub.Subjects()) != 0 {
		t.Errorf("nothing should have been published, got %v", pub.Subjects())
	}
}

func TestDeleteAlbumPublishesTheEvent(t *testing.T) {
	_, _, pub, srv, cookies := removalApp(t, true)

	if resp := deleteAs(t, srv, "/api/albums/al-2", cookies, ""); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", resp.StatusCode)
	}
	subjects := pub.Subjects()
	if len(subjects) != 1 || subjects[0] != "NATHEJK.2026.album.al-2.deleted" {
		t.Fatalf("want one deleted event, got %v", subjects)
	}
}

func TestRemovalBoundsTheReason(t *testing.T) {
	_, _, _, srv, cookies := removalApp(t, true)

	long := `{"reason":"` + strings.Repeat("a", maxAlbumRemovalReason+1) + `"}`
	if resp := deleteAs(t, srv, "/api/albums/al-1/items/0", cookies, long); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400 for an over-long reason, got %d", resp.StatusCode)
	}
}

// # The purge tests after PRD 022
//
// Five tests here used to assert that removing an album item deleted its bytes where nothing else
// referenced them, with a careful sharing check against the glimt and against other album items.
//
// That behaviour is **gone on purpose**, and the reason is worth more than the tests were. Before the
// library split an album item *was* the photograph, so taking it out of the album was the only way that
// photograph stopped existing and a purge was correct. Now the photograph lives in `photo` and outlives
// every album it appears in — so purging on removal would blank it in the library and in every other
// album, which is the exact bug the old sharing check existed to prevent, reintroduced from the other end.
//
// Byte deletion moved to the library's own delete path (task 379), where `photo.RefsInUse` can ask the
// question about the thing being deleted. What is asserted here instead is the new invariant: **a removal
// from an album never touches bytes.** The glimt side of the old hazard is still covered, in
// albumblobs_test.go, which now stubs the library.

// Removing a photograph from an album must leave its bytes alone: it is still in the library, and possibly
// in other albums.
func TestRemovingAnAlbumItemKeepsTheBytes(t *testing.T) {
	app, store, _, srv, cookies := removalApp(t, true)

	kept, err := app.blobs.Put(context.Background(), []byte("a photograph taken out of one album"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	thumb, err := app.blobs.Put(context.Background(), []byte("its thumbnail"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	store.albums[0].items[0].Ref = kept.String()
	store.albums[0].items[0].ThumbRef = thumb.String()

	if resp := deleteAs(t, srv, "/api/albums/al-1/items/0", cookies, ""); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", resp.StatusCode)
	}

	for name, ref := range map[string]blob.Ref{"full": kept, "thumbnail": thumb} {
		if exists, _ := app.blobs.Exists(context.Background(), ref); !exists {
			t.Errorf("the %s object was deleted: the photograph is still in the library", name)
		}
	}
}

// Deleting a whole album does not delete its photographs. The album was an arrangement; the photographs
// were only arranged.
func TestDeletingAnAlbumKeepsItsPhotographsBytes(t *testing.T) {
	app, store, _, srv, cookies := removalApp(t, true)

	first, err := app.blobs.Put(context.Background(), []byte("item one"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	second, err := app.blobs.Put(context.Background(), []byte("item two"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	store.albums[0].items[0].Ref = first.String()
	store.albums[0].items[0].ThumbRef = ""
	store.albums[0].items[1].Ref = second.String()
	store.albums[0].items[1].ThumbRef = ""

	if resp := deleteAs(t, srv, "/api/albums/al-1", cookies, ""); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", resp.StatusCode)
	}

	for name, ref := range map[string]blob.Ref{"first": first, "second": second} {
		if exists, _ := app.blobs.Exists(context.Background(), ref); !exists {
			t.Errorf("%s item's bytes were deleted with the album; the photograph outlives it", name)
		}
	}
}

// The fold is what makes a removal visible, so assert the projection's own behaviour end to end: after
// the event is applied, the photograph is gone from every public read.
func TestRemovedItemLeavesEveryPublicSurface(t *testing.T) {
	_, store, _, srv, cookies := removalApp(t, true)

	if resp := deleteAs(t, srv, "/api/albums/al-1/items/0", cookies, ""); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", resp.StatusCode)
	}

	// The stub is not a projection, so apply the fold's effect by hand — which is exactly what the
	// consumer's own test asserts it writes (`UPDATE album_item SET deleted=1`).
	store.albums[0].items = store.albums[0].items[1:]

	_, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	page := string(body)
	if strings.Contains(page, "media/0?variant=thumb") {
		t.Error("the removed photograph is still on the album page")
	}
	if strings.Contains(page, "Ved målstregen") {
		t.Error("the removed photograph's caption is still on the album page")
	}

	// And its bytes are no longer reachable by URL, because the media route needs a live row.
	resp, _ := getPublic(t, srv.URL+"/api/public/albums/al-1/media/0", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("a removed photograph must not remain fetchable, got %d", resp.StatusCode)
	}
}

// The cache window is the bound on "promptly", and task 332 already asserts it is 60s. This asserts the
// two are connected: if somebody lengthens the page cache, this test says why they should not.
func TestRemovalPromptnessIsBoundedByThePageCache(t *testing.T) {
	_, _, _, srv, _ := removalApp(t, true)

	resp, _ := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	if got := resp.Header.Get("Cache-Control"); !strings.Contains(got, "max-age=60") {
		t.Fatalf("a removal is only as prompt as the page cache: want max-age=60, got %q", got)
	}
}

// The exclusion-set test that used to live here went with the purge (see the note above): there is no
// sharing check on this path any more, because nothing on it deletes bytes. The property it guarded — that
// a delete path must not let the thing being deleted vote to keep its own bytes — still matters and is now
// the library's, asserted in albumblobs_test.go and, for the deletion itself, in task 379.

// Removal is soft in the fold, which is where it is decided — see the album projection's
// `TestItemRemovedIsASoftDelete`. Recorded here as a pointer rather than duplicated: a destructive
// delete would make putting a photograph back require re-uploading bytes we deliberately destroyed.
