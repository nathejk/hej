package main

import (
	"net/http"
	"strings"
	"testing"

	"nathejk.dk/nathejk/table/album"
	"nathejk.dk/nathejk/table/photo"
)

// The chosen cover (task 396): any live photograph in the album, set through the album edit.

func coverApp(t *testing.T) (*albumCurator, func(body string) *http.Response, func() []album.Updated) {
	t.Helper()
	items := itemsOf(photoID("a"), photoID("b"), photoID("c"), photoID("d"))
	items[2].Removed = true
	items[3].PhotoDeleted = true
	curator := newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{ID: "al-1", Slug: "s", Title: "S"}, items: items})
	_, srv, pub := albumWriteApp(t, curator)

	patch := func(body string) *http.Response { return moveAdmin(t, srv, "/api/admin/albums/al-1", body) }
	events := func() []album.Updated {
		var out []album.Updated
		for _, m := range pub.Messages {
			var u album.Updated
			if err := m.Body(&u); err != nil {
				t.Fatal(err)
			}
			out = append(out, u)
		}
		return out
	}
	return curator, patch, events
}

func TestTheCoverCanBeAnyLivePhotographInTheAlbum(t *testing.T) {
	_, patch, events := coverApp(t)

	if got := patch(`{"coverPhotoId":"` + photoID("b") + `"}`).StatusCode; got != http.StatusOK {
		t.Fatalf("want 200, got %d", got)
	}
	ev := events()
	if len(ev) != 1 || ev[0].CoverPhotoID == nil || *ev[0].CoverPhotoID != photoID("b") {
		t.Fatalf("want one update choosing b, got %+v", ev)
	}
	if ev[0].Title != nil || ev[0].Published != nil {
		t.Error("choosing a cover must carry nothing else")
	}
}

func TestClearingTheCoverIsAllowed(t *testing.T) {
	_, patch, events := coverApp(t)

	if got := patch(`{"coverPhotoId":""}`).StatusCode; got != http.StatusOK {
		t.Fatalf("want 200, got %d", got)
	}
	if ev := events(); len(ev) != 1 || ev[0].CoverPhotoID == nil || *ev[0].CoverPhotoID != "" {
		t.Errorf("want one update clearing the cover, got %+v", ev)
	}
}

// A cover the album does not show would be stored and then ignored by the cover rule — which reads as a button that
// does not work. Refused instead: not in the album, removed from it, or deleted from the library.
func TestTheCoverMustBeAPhotographTheAlbumShows(t *testing.T) {
	_, patch, events := coverApp(t)

	for name, id := range map[string]string{
		"not in the album":         photoID("z"),
		"removed from the album":   photoID("c"),
		"deleted from the library": photoID("d"),
	} {
		if got := patch(`{"coverPhotoId":"` + id + `"}`).StatusCode; got != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", name, got)
		}
	}
	if ev := events(); len(ev) != 0 {
		t.Errorf("a refused cover must publish nothing, got %d", len(ev))
	}
}

// The album view's grid marks the album's cover, and only there.
func TestTheAlbumGridMarksTheCover(t *testing.T) {
	lib := &libraryCurator{rows: []photo.LibraryPhoto{
		{ID: photoID("a"), BoundsVerdict: "none"}, {ID: photoID("b"), BoundsVerdict: "none"},
	}}
	app, srv := libraryApp(t, lib)
	app.models.AlbumCurator = newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{
		ID: "al-1", Slug: "s", CoverPhotoID: photoID("b"),
	}})

	body := adminBody(t, getAdmin(t, srv, "/admin/fragments/photos?album=al-1", testAdminUser, testAdminPass))
	if strings.Count(body, "forsidebillede") != 1 {
		t.Fatalf("want exactly one cover mark\n%s", body)
	}
	b := body[strings.Index(body, `data-id="`+photoID("b")+`"`):]
	if !strings.Contains(b[:strings.Index(b, "</button>")], "forsidebillede") {
		t.Error("the mark must be on the cover's cell")
	}

	body = adminBody(t, getAdmin(t, srv, "/admin/fragments/photos", testAdminUser, testAdminPass))
	if strings.Contains(body, "forsidebillede") {
		t.Error("the library grid has no album, so no cover mark")
	}
}
