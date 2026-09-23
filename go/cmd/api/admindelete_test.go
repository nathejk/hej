package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jrgensen/cqrs/cqrstest"

	"nathejk.dk/nathejk/table/album"
	"nathejk.dk/nathejk/table/photo"
)

// The curator's two removals (PRD 022 §5, §6, task 379).
//
// # The distinction these tests exist to protect
//
// Removing a photograph from an album leaves it in the library and in every other album, and frees nothing.
// Deleting it from the library takes it out of everything and frees its bytes. *One of the two is what somebody
// means by "take it down"*, and they must not behave alike any more than they read alike.

// deleteApp returns an admin app with a library, albums, a glimt stub and a recording publisher.
func deleteApp(t *testing.T, curator *libraryCurator, albums *albumCurator) (*application, *httptest.Server, *cqrstest.Publisher) {
	t.Helper()

	app, srv := adminApp(t)
	app.models.PhotoCurator = curator
	app.models.AlbumCurator = albums
	pub := &cqrstest.Publisher{}
	app.commands = commandsWithPublisher(t, pub)
	return app, srv, pub
}

// ---------------------------------------------------------------------------
// Removing from an album.
// ---------------------------------------------------------------------------

// Removing is addressed by **photograph**, and the ordinal is resolved server-side. A stale ordinal in a browser
// would remove whatever now occupies that position, which is a different photograph.
func TestAdminRemovesAPhotographFromOneAlbum(t *testing.T) {
	albums := newAlbumCurator(&curatedAlbum{
		a: album.CuratorAlbum{ID: "al-1", Title: "Natten"},
		items: []album.CuratorItem{
			{Ordinal: 0, PhotoID: photoID("a")},
			{Ordinal: 3, PhotoID: photoID("c")},
		},
	})
	app, srv, pub := deleteApp(t, &libraryCurator{}, albums)

	ref, err := app.blobs.Put(context.Background(), []byte("a photograph in an album"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	got := deleteAdmin(t, srv, "/api/admin/albums/al-1/items/"+photoID("c")).StatusCode
	if got != http.StatusNoContent {
		t.Fatalf("want 204, got %d", got)
	}

	if len(pub.Subjects()) != 1 {
		t.Fatalf("want 1 event, got %d", len(pub.Subjects()))
	}
	var removed album.ItemRemoved
	if err := pub.Messages[0].Body(&removed); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	// The photograph sat at ordinal 3, not at its index in the list. Resolving server-side is what gets that right.
	if removed.Ordinal != 3 {
		t.Errorf("want the photograph's own ordinal 3, got %d", removed.Ordinal)
	}

	// **Nothing is purged.** The photograph outlives the album, so freeing its bytes here would blank it in the
	// library and in every other album.
	if exists, _ := app.blobs.Exists(context.Background(), ref); !exists {
		t.Error("removing from an album must not delete any bytes: the photograph is still in the library")
	}
}

// A photograph that is not in the album, or already removed from it, answers 404 either way — a second removal is
// not an error worth its own status, and the curator's view is the same.
func TestAdminRemoveFromAlbumRefusesANonMember(t *testing.T) {
	albums := newAlbumCurator(&curatedAlbum{
		a: album.CuratorAlbum{ID: "al-1"},
		items: []album.CuratorItem{
			{Ordinal: 0, PhotoID: photoID("a")},
			{Ordinal: 1, PhotoID: photoID("c"), Removed: true},
		},
	})
	_, srv, pub := deleteApp(t, &libraryCurator{}, albums)

	for name, path := range map[string]string{
		"not in the album": "/api/admin/albums/al-1/items/" + photoID("d"),
		"already removed":  "/api/admin/albums/al-1/items/" + photoID("c"),
		"unknown album":    "/api/admin/albums/al-nope/items/" + photoID("a"),
	} {
		if got := deleteAdmin(t, srv, path).StatusCode; got != http.StatusNotFound {
			t.Errorf("%s: want 404, got %d", name, got)
		}
	}
	if len(pub.Subjects()) != 0 {
		t.Errorf("nothing should publish, got %d events", len(pub.Subjects()))
	}
}

// ---------------------------------------------------------------------------
// Deleting from the library.
// ---------------------------------------------------------------------------

// A delete is soft, records the reason, and frees the bytes nothing else holds.
func TestAdminDeletesAPhotographFromTheLibrary(t *testing.T) {
	app, srv := adminApp(t)

	full, err := app.blobs.Put(context.Background(), []byte("the photograph"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	thumb, err := app.blobs.Put(context.Background(), []byte("its thumbnail"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{{
		ID: full.String(), Ref: full.String(), ThumbRef: thumb.String(), BoundsVerdict: photo.BoundsNone,
	}}}
	pub := &cqrstest.Publisher{}
	app.commands = commandsWithPublisher(t, pub)

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/admin/photos/"+full.String(),
		strings.NewReader(`{"reason":"en forælder har bedt om det"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.SetBasicAuth(testAdminUser, testAdminPass)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", resp.StatusCode)
	}

	// Soft: the event is a deletion, not a destruction, so the log carries the fact and the reason.
	if len(pub.Subjects()) != 1 || !strings.HasSuffix(pub.Subjects()[0], ".deleted") {
		t.Fatalf("want one deleted event, got %v", pub.Subjects())
	}
	var deleted photo.Deleted
	if err := pub.Messages[0].Body(&deleted); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if deleted.Reason != "en forælder har bedt om det" {
		t.Errorf("the reason must reach the event — it is the only durable record of why, got %q", deleted.Reason)
	}

	// **Both** objects are freed, because a thumbnail is as shareable as the image.
	for name, ref := range map[string]string{"full": full.String(), "thumbnail": thumb.String()} {
		if exists, _ := app.blobs.Exists(context.Background(), blobRefOf(ref)); exists {
			t.Errorf("the %s object should have been freed", name)
		}
	}
}

// **The rule that could destroy data.** Content addressing means a curator's photograph and a participant's glimt
// can be the same bytes, so a library delete must not free what a glimt still shows.
func TestAdminDeleteKeepsBytesAGlimtStillUses(t *testing.T) {
	app, srv := adminApp(t)

	shared, err := app.blobs.Put(context.Background(), []byte("a photograph in both"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	orphan, err := app.blobs.Put(context.Background(), []byte("its thumbnail, unshared"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{{
		ID: shared.String(), Ref: shared.String(), ThumbRef: orphan.String(), BoundsVerdict: photo.BoundsNone,
	}}}
	app.models.Glimt = &stubGlimt{rows: publicGlimtRows()}
	app.models.Glimt.(*stubGlimt).rows[0].Media[0].Ref = shared.String()
	app.commands = commandsWithPublisher(t, &cqrstest.Publisher{})

	if got := deleteAdmin(t, srv, "/api/admin/photos/"+shared.String()).StatusCode; got != http.StatusNoContent {
		t.Fatalf("want 204, got %d", got)
	}

	if exists, _ := app.blobs.Exists(context.Background(), shared); !exists {
		t.Error("the shared object was freed: a glimt that uses it is now blank")
	}
	if exists, _ := app.blobs.Exists(context.Background(), orphan); exists {
		t.Error("the unshared thumbnail should have been freed")
	}
}

// If any owner cannot be asked, nothing is freed. Leaking disk is recoverable and visible; blanking somebody
// else's photograph is neither.
func TestAdminDeleteFreesNothingWhenTheSharingCheckFails(t *testing.T) {
	app, srv := adminApp(t)

	ref, err := app.blobs.Put(context.Background(), []byte("a photograph"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{{
		ID: ref.String(), Ref: ref.String(), BoundsVerdict: photo.BoundsNone,
	}}}
	// The library's *own* ref check fails, which is the half that decides whether another photograph holds these
	// bytes.
	app.models.Photos = &stubPhotos{err: errors.New("database is down")}
	app.commands = commandsWithPublisher(t, &cqrstest.Publisher{})

	if got := deleteAdmin(t, srv, "/api/admin/photos/"+ref.String()).StatusCode; got != http.StatusNoContent {
		t.Fatalf("the deletion itself should still succeed, got %d", got)
	}
	if exists, _ := app.blobs.Exists(context.Background(), ref); !exists {
		t.Fatal("an unanswerable sharing question must leave the objects in place")
	}
}

// **The exclusion is not optional.** The fold is asynchronous, so the row being deleted is still live when the
// check runs — without naming it, it reports its own bytes as in use and nothing is ever freed. That is a bug that
// never fails, it just quietly fills a disk.
func TestAdminDeleteExcludesThePhotographBeingDeleted(t *testing.T) {
	app, srv := adminApp(t)

	ref, err := app.blobs.Put(context.Background(), []byte("a photograph"))
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}

	app.models.PhotoCurator = &libraryCurator{rows: []photo.LibraryPhoto{{
		ID: ref.String(), Ref: ref.String(), BoundsVerdict: photo.BoundsNone,
	}}}
	// A library that reports this very ref as in use — exactly what the real projection does before the fold runs.
	photos := &stubPhotos{inUse: map[string]bool{ref.String(): true}}
	app.models.Photos = photos
	app.commands = commandsWithPublisher(t, &cqrstest.Publisher{})

	deleteAdmin(t, srv, "/api/admin/photos/"+ref.String())

	if len(photos.excluded) != 1 {
		t.Fatalf("want one sharing check, got %d", len(photos.excluded))
	}
	if len(photos.excluded[0]) != 1 || photos.excluded[0][0] != ref.String() {
		t.Errorf("the photograph being deleted must be excluded from the check, got %v", photos.excluded[0])
	}
}

// Deleting an already-deleted photograph is 204, not 404: a curator clicking twice, or two curators acting on the
// same selection, should not meet an error for reaching the state they wanted.
func TestAdminDeleteIsIdempotent(t *testing.T) {
	_, srv, pub := deleteApp(t, &libraryCurator{
		rows: []photo.LibraryPhoto{libRow("a", func(p *photo.LibraryPhoto) { p.Deleted = true })},
	}, newAlbumCurator())

	if got := deleteAdmin(t, srv, "/api/admin/photos/"+photoID("a")).StatusCode; got != http.StatusNoContent {
		t.Errorf("want 204 for an already-deleted photograph, got %d", got)
	}
	if len(pub.Subjects()) != 0 {
		t.Errorf("a second delete must publish nothing, got %d", len(pub.Subjects()))
	}
}

func TestAdminDeleteRefusesAnUnknownPhotograph(t *testing.T) {
	_, srv, pub := deleteApp(t, &libraryCurator{}, newAlbumCurator())

	if got := deleteAdmin(t, srv, "/api/admin/photos/"+photoID("f")).StatusCode; got != http.StatusNotFound {
		t.Errorf("want 404, got %d", got)
	}
	if len(pub.Subjects()) != 0 {
		t.Errorf("nothing should publish, got %d", len(pub.Subjects()))
	}
}

// ---------------------------------------------------------------------------
// The shared purge, and the two doors.
// ---------------------------------------------------------------------------

// **PRD 022 §8.9's requirement.** Two doors to a removal is correct; the two disagreeing about blob purging is not.
//
// The way two copies disagree is not by being written differently but by one of them not being updated when a third
// owner of the blob store appears. So the union of owners is asked in exactly one place, and this asserts both
// paths reach it.
func TestBothDeletePathsShareOnePurgeHelper(t *testing.T) {
	glimtSrc := adminSource(t, "glimtdelete.go")
	adminSrc := adminSource(t, "admindelete.go")

	// Both delegate to the shared helper rather than carrying their own loop.
	if !strings.Contains(glimtSrc, "app.purgeBlobs(") {
		t.Error("the glimt delete path must use the shared purge helper")
	}
	if !strings.Contains(adminSrc, "app.purgeBlobs(") {
		t.Error("the library delete path must use the shared purge helper")
	}

	// And neither reimplements the ownership question.
	for name, src := range map[string]string{"glimtdelete.go": glimtSrc, "admindelete.go": adminSrc} {
		if strings.Contains(src, "RefsUsedElsewhere(") && name != "glimtdelete.go" {
			t.Errorf("%s asks an owner directly; the union belongs in blobRefsInUse", name)
		}
		if strings.Contains(src, "app.blobs.Delete(") {
			t.Errorf("%s deletes blobs directly; the purge belongs in blobRefsInUse's caller", name)
		}
	}
}

// Every owner of the blob store is asked, and nil is not an error. A nil projection means that owner has nothing to
// protect; a *failing* read means we cannot tell and the whole answer is an error.
func TestTheSharedPurgeAsksEveryOwner(t *testing.T) {
	src := adminSource(t, "blobpurge.go")

	for _, owner := range []string{"app.models.Glimt", "app.models.Photos"} {
		if !strings.Contains(src, owner) {
			t.Errorf("the purge check must ask %s", owner)
		}
	}
	// Nil-guarded, so a database-free run still frees disk.
	if strings.Count(src, "!= nil") < 2 {
		t.Error("each owner must be nil-guarded: no rows to protect is not the same as cannot tell")
	}
}

// The in-app removal endpoints are unchanged. They answer a different question — "somebody complained and I am at a
// barbecue with my phone" — and two doors is correct (PRD 022 §8.9).
func TestTheInAppRemovalEndpointsAreUnchanged(t *testing.T) {
	var found int
	for _, r := range allRegisteredRoutes(t) {
		switch r.path {
		case "/api/albums/:albumId", "/api/albums/:albumId/items/:ordinal":
			found++
			// Session-authenticated, not admin-credentialed: these are the Team-section surface.
			if !r.authenticated {
				t.Errorf("%s must stay behind requireAuth", r.path)
			}
			if wrapsRequireAdmin(t, r.path) {
				t.Errorf("%s must not move behind the admin credential; it is the in-app takedown", r.path)
			}
		}
	}
	if found != 2 {
		t.Errorf("want both in-app removal endpoints still registered, found %d", found)
	}
}

// ---------------------------------------------------------------------------
// The copy.
// ---------------------------------------------------------------------------

// **The copy is the substance of this task.** PRD 022 §5 and §7 both single it out: if the two actions read alike,
// the wrong one gets pressed under exactly the pressure that makes it matter.
//
// So this asserts the page says, in Danish, what each one does — and that the distinction is stated rather than
// left to be inferred from two button labels.
func TestTheDeletePanelDistinguishesTheTwoRemovals(t *testing.T) {
	body := renderAdminPage(t)

	// The safe one says the photographs stay in the archive.
	for _, want := range []string{
		"Fjern fra et album",
		"De bliver liggende i arkivet",
		"l\u00e6gge dem tilbage",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the remove-from-album copy is missing %q", want)
		}
	}

	// The destructive one says they disappear, that it cannot be undone, and — the sentence that matters most —
	// that this is the one to use when somebody has asked for a photograph to be taken down.
	for _, want := range []string{
		"Slet fra arkivet",
		"forsvinder helt",
		"fjernet fra alle album",
		"taget ned",
		"ikke</strong> fortrydes",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the delete-from-library copy is missing %q", want)
		}
	}
}

// The destructive action is the visually distinct one, and it is the only red button in the tool.
func TestTheDestructiveActionLooksDifferent(t *testing.T) {
	src := adminPageSource(t)

	if !strings.Contains(src, `id="dodelete" class="danger"`) {
		t.Error("the delete button must carry the danger class")
	}
	if !strings.Contains(src, "#delpanel button.danger") {
		t.Error("the danger class needs a style, or it is a comment rather than a signal")
	}
	// The remove button must not share it.
	if strings.Contains(src, `id="doremove" class="danger"`) {
		t.Error("removing from an album is not destructive and must not look like it is")
	}
}

// The irreversible action confirms, with the count in the question — because "select all in filter" makes a
// mis-aimed delete plausible, and v1 ships no undelete (PRD 022 §11 Q6).
func TestDeletingConfirmsWithTheCount(t *testing.T) {
	src := adminPageSource(t)

	confirm := src[strings.Index(src, "dodelete"):]
	if !strings.Contains(confirm, "window.confirm(") {
		t.Error("deleting must confirm: it is the one irreversible action in the tool")
	}
	if !strings.Contains(confirm, "'Slet ' + word") {
		t.Error("the confirmation must name how many photographs, since a mis-aimed select-all is plausible")
	}
	if !strings.Contains(confirm, "ikke fortrydes") {
		t.Error("the confirmation must say it cannot be undone")
	}

	// And removing from an album must **not** confirm: it is reversible in seconds, and a dialog there would train
	// the curator to dismiss the one that matters.
	remove := src[strings.Index(src, "doRemove.addEventListener"):]
	remove = remove[:strings.Index(remove, "document.getElementById('dodelete')")]
	if strings.Contains(remove, "window.confirm(") {
		t.Error("removing from an album must not confirm; a dialog there teaches the curator to dismiss dialogs")
	}
}
