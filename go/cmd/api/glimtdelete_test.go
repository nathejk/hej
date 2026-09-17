package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nathejk.dk/internal/blob"
	"nathejk.dk/nathejk/table/glimt"
	"nathejk.dk/nathejk/table/person"
)

// Delete tests (task 306).

func deleteWithCookies(t *testing.T, url string, cookies []*http.Cookie) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", url, err)
	}
	resp.Body.Close()
	return resp
}

func itemURL(base, glimtID string) string {
	return base + "/api/glimt/items/" + glimtID
}

func TestDeleteGlimt_AuthorPublishesAndPurges(t *testing.T) {
	app, store, pub := glimtApp(t, nil, spejderPerson())
	row := mediaGlimt(t, app, "g-1", "mock-spejder-1", "spejder", glimt.AudienceGroup)
	store.rows = []glimt.Glimt{row}

	fullRef := blob.Ref(row.Media[0].Ref)
	thumbRef := blob.Ref(row.Media[0].ThumbRef)

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp := deleteWithCookies(t, itemURL(srv.URL, "g-1"), cookies)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}

	if got := pub.Subjects(); len(got) != 1 || got[0] != "NATHEJK.2026.glimt.g-1.deleted" {
		t.Fatalf("subjects = %v", got)
	}

	// Both objects gone. Forgetting the thumbnail would leave a recognisable image on disk
	// while technically having deleted the glimt — and a 320px thumbnail of a child's face is
	// still a photograph of a child's face.
	for name, ref := range map[string]blob.Ref{"full": fullRef, "thumb": thumbRef} {
		if ok, _ := app.blobs.Exists(t.Context(), ref); ok {
			t.Errorf("%s bytes still present after delete", name)
		}
	}
}

// TestDeleteGlimt_RefusesEveryoneButTheAuthor covers the three refusals that matter, including the
// one that is easy to get wrong.
func TestDeleteGlimt_RefusesEveryoneButTheAuthor(t *testing.T) {
	t.Run("another member of the same hold", func(t *testing.T) {
		app, store, pub := glimtApp(t, nil, spejderPerson())
		// Same hold, same group — but not the author.
		store.rows = []glimt.Glimt{mediaGlimt(t, app, "g-1", "other-spejder", "spejder", glimt.AudienceGroup)}

		srv := httptest.NewServer(app.routes())
		defer srv.Close()
		cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

		resp := deleteWithCookies(t, itemURL(srv.URL, "g-1"), cookies)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("status = %d, want 403", resp.StatusCode)
		}
		if len(pub.Subjects()) != 0 {
			t.Error("a refused delete still published an event")
		}
	})

	t.Run("a Team-section moderator", func(t *testing.T) {
		// The refusal most likely to be "fixed" by a well-meaning change. Moderators hide;
		// only the author destroys. That asymmetry is what makes hiding cheap enough to be
		// the default response to a report.
		moderator := person.Person{
			PersonID: "mock-spejder-1", AppRole: person.RoleSpejder,
			SectionSlug: person.SectionTeam,
		}
		app, store, pub := glimtApp(t, nil, moderator)
		row := mediaGlimt(t, app, "g-1", "someone-else", "spejder", glimt.AudienceGroup)
		store.rows = []glimt.Glimt{row}

		srv := httptest.NewServer(app.routes())
		defer srv.Close()
		cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

		resp := deleteWithCookies(t, itemURL(srv.URL, "g-1"), cookies)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("status = %d, want 403 — moderators hide, authors delete", resp.StatusCode)
		}
		if len(pub.Subjects()) != 0 {
			t.Error("a moderator's delete published an event")
		}
		// And the bytes are untouched, so an unhide would still work.
		if ok, _ := app.blobs.Exists(t.Context(), blob.Ref(row.Media[0].Ref)); !ok {
			t.Error("a refused delete removed the media")
		}
	})

	t.Run("unauthenticated", func(t *testing.T) {
		app, store, _ := glimtApp(t, nil, spejderPerson())
		store.rows = []glimt.Glimt{mediaGlimt(t, app, "g-1", "mock-spejder-1", "spejder", glimt.AudienceGroup)}

		srv := httptest.NewServer(app.routes())
		defer srv.Close()

		resp := deleteWithCookies(t, itemURL(srv.URL, "g-1"), nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", resp.StatusCode)
		}
	})
}

// TestDeleteGlimt_KeepsBytesSharedWithAnotherGlimt is the content-addressing trap.
//
// Identical bytes are one object with one ref, so deleting it for one glimt would blank the media of
// every other glimt referencing it. Two members of a patrulje posting the same photo — one AirDropped
// it to the other — produce exactly this, and so does one member posting the same picture twice.
// Without the reference check the newer delete would silently break the older post, and the only
// evidence would be a grey box in somebody else's feed.
func TestDeleteGlimt_KeepsBytesSharedWithAnotherGlimt(t *testing.T) {
	app, store, _ := glimtApp(t, nil, spejderPerson())

	mine := mediaGlimt(t, app, "g-mine", "mock-spejder-1", "spejder", glimt.AudienceGroup)
	// A different glimt, by a different member, referencing the *same objects* — which is
	// what posting the same photo produces under content addressing.
	theirs := mediaGlimt(t, app, "g-theirs", "other-spejder", "spejder", glimt.AudienceGroup)
	theirs.Media[0].Ref = mine.Media[0].Ref
	theirs.Media[0].ThumbRef = mine.Media[0].ThumbRef
	store.rows = []glimt.Glimt{mine, theirs}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp := deleteWithCookies(t, itemURL(srv.URL, "g-mine"), cookies)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}

	for name, ref := range map[string]string{"full": mine.Media[0].Ref, "thumb": mine.Media[0].ThumbRef} {
		if ok, _ := app.blobs.Exists(t.Context(), blob.Ref(ref)); !ok {
			t.Errorf("%s bytes were deleted although another glimt still references them", name)
		}
	}
}

// TestDeleteGlimt_LeavesBytesWhenSharingCannotBeChecked is the safe direction.
//
// If we cannot tell whether an object is shared, we keep it. Leaking disk space is recoverable and
// invisible; blanking a photo in another member's feed is neither.
func TestDeleteGlimt_LeavesBytesWhenSharingCannotBeChecked(t *testing.T) {
	app, store, pub := glimtApp(t, nil, spejderPerson())
	row := mediaGlimt(t, app, "g-1", "mock-spejder-1", "spejder", glimt.AudienceGroup)
	store.rows = []glimt.Glimt{row}
	store.refsErr = errRefCheckFailed

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp := deleteWithCookies(t, itemURL(srv.URL, "g-1"), cookies)
	// The delete still succeeds — the glimt is gone from every view, which is what the member
	// asked for and what was published.
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	if len(pub.Subjects()) != 1 {
		t.Fatalf("subjects = %v", pub.Subjects())
	}
	if ok, _ := app.blobs.Exists(t.Context(), blob.Ref(row.Media[0].Ref)); !ok {
		t.Error("bytes were deleted although sharing could not be checked")
	}
}

// TestDeleteGlimt_PublishesBeforeDeletingBytes pins the ordering.
//
// The opposite of the retention sweep's order, deliberately: a member is watching this one. If the
// bytes went first and the publish failed, they would be told the delete failed while their glimt sat
// in everyone's feed with grey boxes. Publishing first means the worst case is unreferenced bytes,
// which no URL can reach because the media handler needs a row to find them.
func TestDeleteGlimt_PublishesBeforeDeletingBytes(t *testing.T) {
	app, store, _ := glimtApp(t, nil, spejderPerson())
	row := mediaGlimt(t, app, "g-1", "mock-spejder-1", "spejder", glimt.AudienceGroup)
	store.rows = []glimt.Glimt{row}
	// No publisher, so the publish fails.
	app.commands = commandsWithNoPublisher()

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp := deleteWithCookies(t, itemURL(srv.URL, "g-1"), cookies)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
	// The media survives, so the glimt is still whole and the member can retry rather than
	// being left with a post full of grey boxes.
	for name, ref := range map[string]string{"full": row.Media[0].Ref, "thumb": row.Media[0].ThumbRef} {
		if ok, _ := app.blobs.Exists(t.Context(), blob.Ref(ref)); !ok {
			t.Errorf("%s bytes were deleted although the deletion was never published", name)
		}
	}
}

func TestDeleteGlimt_UnknownIs404(t *testing.T) {
	app, _, _ := glimtApp(t, nil, spejderPerson())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp := deleteWithCookies(t, itemURL(srv.URL, "never-existed"), cookies)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

// TestDeleteGlimt_OwnershipSurvivesAProfileSwitch records why ownership is the stored person id
// rather than anything session-shaped.
func TestDeleteGlimt_OwnershipSurvivesAProfileSwitch(t *testing.T) {
	app, store, pub := glimtApp(t, nil, spejderPerson())
	row := mediaGlimt(t, app, "g-1", "mock-spejder-1", "spejder", glimt.AudienceGroup)
	// Created a while ago, under a session that no longer exists.
	row.CreatedAt = time.Date(2026, 9, 16, 20, 0, 0, 0, time.UTC)
	store.rows = []glimt.Glimt{row}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()
	// A fresh login for the same person.
	cookies := authedCookies(t, app, srv, "30000001", "+4530000001")

	resp := deleteWithCookies(t, itemURL(srv.URL, "g-1"), cookies)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204 — ownership is the person, not the session", resp.StatusCode)
	}
	if len(pub.Subjects()) != 1 {
		t.Errorf("subjects = %v", pub.Subjects())
	}
}

func TestGlimtRefsOfCollectsBothVariants(t *testing.T) {
	g := glimt.Glimt{Media: []glimt.Media{
		{Ordinal: 0, Ref: "full-a", ThumbRef: "thumb-a"},
		{Ordinal: 1, Ref: "full-b"}, // no thumbnail — a real state (task 303)
	}}
	got := glimtRefsOf(g)
	if len(got) != 3 {
		t.Errorf("refs = %v, want three (two full, one thumb)", got)
	}
}
