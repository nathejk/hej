package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nathejk.dk/nathejk/table/album"
)

// Task 396: the tool is three pages under the year, beside the public ones, instead of one page at `/admin`.

// **Under the public prefix is not public.** The pages beside `/2026/album/:slug` must still refuse anyone without
// the credential — the whole risk of moving them there, and the one thing the path no longer says.
func TestTheCuratorPagesUnderTheYearNeedTheCredential(t *testing.T) {
	_, srv := adminApp(t)

	for _, path := range []string{"/2026/albums", "/2026/photos", "/2026/album/natten/edit"} {
		for _, creds := range [][2]string{{"", ""}, {"nobody", "guess"}} {
			resp := getAdmin(t, srv, path, creds[0], creds[1])
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("%s with %q: want 401, got %d", path, creds[0], resp.StatusCode)
			}
			if strings.Contains(adminBody(t, resp), adminPageMarker) {
				t.Errorf("%s served the tool without the right credential", path)
			}
		}
	}
}

// The landing page is the album list and the counts, and nothing that needs the contact sheet's script.
func TestTheAlbumsPageIsTheListAndNotTheSheet(t *testing.T) {
	app, srv := adminApp(t)
	app.models.PhotoCurator = &libraryCurator{}
	app.models.PhotoCurator.(*libraryCurator).counts.InNoAlbum = 3

	body := adminBody(t, getAdmin(t, srv, "/2026/albums", testAdminUser, testAdminPass))

	for _, want := range []string{
		`hx-get="/admin/fragments/albums"`,
		`id="counts"`,
		`href="/2026/photos?album=none">3 billeder uden album</a>`,
		`href="/2026/albums" aria-current="page"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the albums page should contain %s", want)
		}
	}
	for _, unwanted := range []string{`id="sheet"`, `id="drop"`, `initAdminTool`, `leaflet.js`} {
		if strings.Contains(body, unwanted) {
			t.Errorf("the albums page should not carry %s; that belongs to the photos page", unwanted)
		}
	}
}

// The all-photos page lights the filter its URL names and asks for the first page already filtered, so the album
// list's "uden album" link lands on exactly that — and a reload keeps it.
func TestThePhotosPageHonoursTheFilterInItsURL(t *testing.T) {
	app, srv := adminApp(t)
	app.models.PhotoCurator = &libraryCurator{}

	body := adminBody(t, getAdmin(t, srv, "/2026/photos?album=none", testAdminUser, testAdminPass))

	if !strings.Contains(body, `class="f on" data-q="album=none"`) {
		t.Error("the filter in the URL should be the lit one")
	}
	if !strings.Contains(body, `hx-get="/admin/fragments/photos?limit=120&album=none"`) {
		t.Error("the first page of thumbnails should be requested with the URL's filter")
	}
	if strings.Count(body, `class="f on"`) != 1 {
		t.Error("exactly one filter is lit")
	}
}

// A query no preset represents falls back to "Alle", rather than narrowing the grid with nothing on screen saying so.
func TestThePhotosPageIgnoresAQueryNoPresetNames(t *testing.T) {
	app, srv := adminApp(t)
	app.models.PhotoCurator = &libraryCurator{}

	body := adminBody(t, getAdmin(t, srv, "/2026/photos?verdict=insid", testAdminUser, testAdminPass))

	if !strings.Contains(body, `class="f on" data-q=""`) {
		t.Error("an unknown query should light Alle")
	}
	if !strings.Contains(body, `hx-get="/admin/fragments/photos?limit=120"`) {
		t.Error("an unknown query must not reach the fragment request")
	}
}

// The public album page is untouched by its editor now living one segment below it.
func TestThePublicAlbumPageStaysPublicBesideItsEditor(t *testing.T) {
	_, srv := adminApp(t)

	resp, err := srv.Client().Get(srv.URL + "/2026/album/noget")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		t.Error("the public album page must not ask for the admin credential")
	}
}

// albumViewApp has one album, "natten", with an id that needs no escaping, and a library to read counts from.
func albumViewApp(t *testing.T) *httptest.Server {
	t.Helper()
	app, srv := adminApp(t)
	app.models.PhotoCurator = &libraryCurator{}
	app.models.AlbumCurator = newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{
		ID: "al-1", Slug: "natten", Title: "Natten", ItemCount: 4,
	}})
	return srv
}

// The album view is the editor card over the shared contact sheet, narrowed to the album — the same grid, the
// same action bar and the same sheets as the library, so sorting an album needs no trip back to it.
func TestTheAlbumViewIsTheEditorOverTheSharedSheet(t *testing.T) {
	srv := albumViewApp(t)

	resp := getAdmin(t, srv, "/2026/album/natten/edit", testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	body := adminBody(t, resp)

	for _, want := range []string{
		`id="albumeditor" data-album="al-1"`,
		`data-query="album=al-1"`,
		`hx-get="/admin/fragments/photos?limit=120&album=al-1"`,
		`4 billeder i albummet`,
		`data-act="caption"`, `data-act="album"`, `data-act="position"`, `data-act="patrol"`,
		`data-act="credit"`, `data-act="delete"`,
		`id="captionpanel"`, `id="delpanel"`,
		`initAlbumEditor`,
		`<title>Billedarkiv 2026 — Natten`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the album view should contain %s", want)
		}
	}
	// Not the library's own parts: no upload and no filters, and no year-wide counts that would read as the album's.
	for _, unwanted := range []string{`id="drop"`, `id="filters"`, `id="counts"`} {
		if strings.Contains(body, unwanted) {
			t.Errorf("the album view should not carry %s", unwanted)
		}
	}
}

func TestTheAlbumViewOfAnUnknownSlugIs404(t *testing.T) {
	srv := albumViewApp(t)

	if got := getAdmin(t, srv, "/2026/album/findes-ikke/edit", testAdminUser, testAdminPass).StatusCode; got != http.StatusNotFound {
		t.Errorf("want 404, got %d", got)
	}
}

// On the album view, "Fjern fra et album" defaults to that album. Asked for by the sheet with `?album=`.
func TestTheRemovePickerCanPreselectAnAlbum(t *testing.T) {
	srv := albumViewApp(t)

	body := adminBody(t, getAdmin(t, srv, "/admin/fragments/delalbumpicker?album=al-1", testAdminUser, testAdminPass))
	if !strings.Contains(body, `<option value="al-1" selected>`) {
		t.Errorf("the album asked for should be selected\n%s", body)
	}
	body = adminBody(t, getAdmin(t, srv, "/admin/fragments/delalbumpicker", testAdminUser, testAdminPass))
	if strings.Contains(body, "selected") {
		t.Errorf("with no album asked for, nothing is preselected\n%s", body)
	}
}
