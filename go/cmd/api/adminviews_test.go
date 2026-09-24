package main

import (
	"net/http"
	"strings"
	"testing"
)

// Task 396: the tool is three pages under the year, beside the public ones, instead of one page at `/admin`.

// noRedirect is a client that reports a redirect instead of following it, so the Location can be asserted.
func noRedirect(t *testing.T, srvURL, path string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, srvURL+path, nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.SetBasicAuth(testAdminUser, testAdminPass)
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// The old addresses keep working: a bookmark, or task 385's half-page, lands on the page it used to show.
func TestTheOldAdminAddressesRedirect(t *testing.T) {
	_, srv := adminApp(t)

	for path, want := range map[string]string{
		"/admin":              "/2026/albums",
		"/admin/album/natten": "/2026/album/natten/edit",
	} {
		resp := noRedirect(t, srv.URL, path)
		if resp.StatusCode != http.StatusFound {
			t.Errorf("%s: want 302, got %d", path, resp.StatusCode)
		}
		if got := resp.Header.Get("Location"); got != want {
			t.Errorf("%s: want a redirect to %s, got %q", path, want, got)
		}
	}
}

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
