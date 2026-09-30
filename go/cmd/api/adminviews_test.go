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
	// Not the year-wide counts, which would read as this album's.
	//
	// **Two things have come off this list, and both were the same mistake.** The filter row (task 460) was excluded
	// because the album *was* the filter — which missed that the album is where the work happens, and the page was
	// already offering "Vælg alle der matcher filteret" for a filter it did not have. The uploader (task 474) was
	// excluded because uploading was the library's business — which missed that a curator assembling an album has a
	// card to put in it, and the alternative is upload, navigate back, select, file.
	//
	// What remains on the list is the thing that is genuinely about the year rather than the album.
	for _, unwanted := range []string{`id="counts"`} {
		if strings.Contains(body, unwanted) {
			t.Errorf("the album view should not carry %s", unwanted)
		}
	}
}

// The album view has the filter row, and every preset it shows composes with the album (task 460).
//
// # Why this is the whole point of the task
//
// The album view is where a curator actually works, and an album is 100–300 photographs. "Is anything missing a
// position or a caption" is a question about the album, asked of the whole album, and sometimes the answer is
// "yes, deliberately" — so it has to be *visible* rather than inferred. Before this the view had no row at all and
// yet still offered "Vælg alle der matcher filteret": a button naming a filter the page did not have.
//
// What must hold is the composition. A preset that replaced the query rather than narrowing it would silently show
// the whole year's uncaptioned pile on a page titled with one album's name — with the action bar, and "select all
// matching", acting on it.
func TestTheAlbumViewFiltersWithinTheAlbum(t *testing.T) {
	srv := albumViewApp(t)

	body := adminBody(t, getAdmin(t, srv, "/2026/album/natten/edit?caption=no", testAdminUser, testAdminPass))

	for _, want := range []struct{ needle, why string }{
		{`id="filters"`, "the row is on this view now"},
		{`data-q="caption=no"`, "the preset the task asked for: which of these has nobody written a line for"},
		{`data-q="location=no"`, "and the other half of the question, which already existed"},
		{`data-query="album=al-1&amp;caption=no"`,
			"the grid reads the album **narrowed** by the preset; a preset that replaced the query would show " +
				"the whole year on a page named for one album"},
		{`data-base="album=al-1"`,
			"the browser needs the album separately, to recompose when another preset is clicked"},
		// The literal `&` in the template stays literal while the one inside `.Query` is escaped, so the attribute
		// reads `…&album=al-1&amp;caption=no`. Asserted as it is actually written rather than as it looks like it
		// should be — the first version of this needle was wrong in exactly that way.
		{`hx-get="/admin/fragments/photos?limit=120&album=al-1&amp;caption=no"`,
			"the first request is already the narrowed set: loading 200 thumbnails and then taking them away is " +
				"the version of this that feels broken"},
		{`data-path="/2026/album/natten/edit"`,
			"a choice is written to the album's own address, by slug — navigating to /photos would drop the album"},
		{`class="f on" data-q="caption=no"`, "and the button the URL names is the one lit"},
	} {
		if !strings.Contains(body, want.needle) {
			t.Errorf("the album view is missing %s: %s", want.needle, want.why)
		}
	}

	// **"Uden album" is not offered here**, because every photograph on this page is in one. A preset that can only
	// ever return nothing is worse than a missing one: a curator who clicks it learns that the filters lie.
	if strings.Contains(body, `data-q="album=none"`) {
		t.Error("the album view must not offer `album=none`: inside an album it can only return an empty grid")
	}
	// It is still on the all-photos view, which is where it answers the question it was built for.
	if photos := adminBody(t, getAdmin(t, srv, "/2026/photos", testAdminUser, testAdminPass)); !strings.Contains(
		photos, `data-q="album=none"`) {
		t.Error("`album=none` is the all-photos view's most-used filter and must stay there")
	}
}

// The two rows are one list, so a filter cannot exist in one view and be missing from the other by accident.
func TestTheAlbumRowIsTheLibraryRowMinusWhatCannotApply(t *testing.T) {
	inAlbum, _ := adminFiltersInAlbum("")
	inLibrary, _ := adminFiltersFor("")

	// Asserted against the Go values rather than the page, because that is what both views render *from* — and
	// `adminFilters` is not in the template's text at all (see TestTheFilterRowOffersUncreditedPhotographs).
	allowed := map[string]bool{}
	for _, f := range inLibrary {
		allowed[f.Q] = true
	}
	for _, f := range inAlbum {
		if !allowed[f.Q] {
			t.Errorf("the album row offers %q, which the library row does not: the two must be one list, or a "+
				"curator learns a filter in one view and cannot find it in the other", f.Q)
		}
	}
	if len(inAlbum) != len(inLibrary)-1 {
		t.Errorf("the album row has %d presets and the library row %d; exactly one — `album=none` — cannot apply "+
			"inside an album, so any other difference is a filter that went missing", len(inAlbum), len(inLibrary))
	}

	// Both rows always have something lit, or the grid is narrowed with nothing on screen saying so.
	for name, row := range map[string][]adminFilterView{"album": inAlbum, "photos": inLibrary} {
		on := 0
		for _, f := range row {
			if f.On {
				on++
			}
		}
		if on != 1 {
			t.Errorf("%s row has %d presets switched on, want exactly 1", name, on)
		}
	}
}

// A preset the album view does not offer must not be honoured by its URL either.
//
// `adminFiltersFor`'s rule, on the other view: only an exact preset is honoured, because a grid narrowed by a query
// no button represents is a curator looking at a subset with nothing saying so. `album=none` typed into the album
// view's URL would be exactly that — and would compose to `album=al-1&album=none`, where the second wins in the
// parser and the page would show the unsorted pile under an album's title.
func TestTheAlbumViewIgnoresAPresetItDoesNotOffer(t *testing.T) {
	srv := albumViewApp(t)

	body := adminBody(t, getAdmin(t, srv, "/2026/album/natten/edit?album=none", testAdminUser, testAdminPass))

	if !strings.Contains(body, `data-query="album=al-1"`) {
		t.Error("an unoffered preset must fall back to the album alone, not compose with it")
	}
	if strings.Contains(body, "album=none") {
		t.Error("`album=none` must not reach the album view's markup at all")
	}
}

// The browser recomposes rather than reloading the page, and it keeps the album while doing it (task 460).
//
// Source-read, because there is no JavaScript runtime in this suite. It is worth pinning for one reason: a filter
// click that dropped `base` would leave the *page* saying one album and the *grid* showing the year, and both
// "select all matching" and every action in the bar read the same `query`. The bug would not be a wrong screen, it
// would be a bulk edit applied to photographs the curator never saw.
func TestTheFilterRowKeepsTheAlbumWhenItChangesThePreset(t *testing.T) {
	js := stripJSLineComments(adminAsset(t, "contactsheet.js"))

	for _, want := range []struct{ needle, why string }{
		{"const base = sheet.dataset.base || '';",
			"the album arrives separately from the preset, because the preset is what a click replaces"},
		{"const compose = (preset) => [base, preset].filter(Boolean).join('&');",
			"composed in one place; two call sites building the query by hand is how one of them loses the album"},
		{"query = compose(preset);",
			"the filter click must compose rather than assign — assigning is the bug this guards"},
		{"history.replaceState(null, '', filters.dataset.path + (preset ? '?' + preset : ''));",
			"the URL gets the preset and the view's own path: the album is already in the path, and writing its " +
				"id beside its slug would be two answers to which album this is"},
	} {
		if !strings.Contains(js, want.needle) {
			t.Errorf("contactsheet.js no longer has %q: %s", want.needle, want.why)
		}
	}

	// And nothing assigns the bare preset to `query`, which is what the code did before the album view had a row.
	if strings.Contains(js, "query = b.dataset.q;") {
		t.Error("assigning the preset straight to `query` drops the album on the album view — compose() exists " +
			"for exactly this line")
	}
}

// Dragging is refused while the grid is filtered, and the two files agree on how that is known (task 460).
//
// # Why the filter row had to bring this with it
//
// A move says "these, before that one" and the server rebuilds the album's **whole** order, because the browser may
// hold 120 of 200 (moveAdminAlbumItemsHandler). That is right when the grid is the album. With a filter on it is a
// trap: dropping a photograph before the twelfth *uncaptioned* one puts it before the twelfth photograph of the
// album, off screen — and the filtered grid can look unchanged afterwards, which reads as the drag having failed.
// The reorder also rewrites every ordinal in the album, so "it looked like nothing happened" is not harmless.
func TestReorderingIsRefusedWhileTheAlbumGridIsFiltered(t *testing.T) {
	sheetJS := stripJSLineComments(adminAsset(t, "contactsheet.js"))
	orderJS := stripJSLineComments(adminAsset(t, "albumorder.js"))

	// One side publishes the fact, the other reads it. Asserted as a pair, because the failure mode is a rename on
	// one side: the flag would simply never be set, and dragging in a filtered album would silently come back.
	if !strings.Contains(sheetJS, "sheet.dataset.filtered = query === base ? '' : '1';") {
		t.Error("contactsheet.js must publish whether the grid shows a subset: it owns `query`, and a filter click " +
			"does not reload the page, so the server cannot say")
	}
	if !strings.Contains(orderJS, "if (sheet.dataset.filtered) {") {
		t.Error("albumorder.js must refuse to start a drag while the grid is filtered")
	}

	// Refused at the **start** of the gesture. The sort-mode confirmation deliberately waits for the end, because
	// there is a decision to make and the curator may drop the photograph back where it came from; here there is no
	// decision, and opening the gap would promise a move that is not going to happen.
	before := strings.Index(orderJS, "if (sheet.dataset.filtered) {")
	press := strings.Index(orderJS, "press = { id: cell.dataset.id")
	if before < 0 || press < 0 || before > press {
		t.Error("the refusal must come before the press is recorded, or the gap opens and the stack lifts for a " +
			"move that will not be sent")
	}
	// And it is said where an action's outcome is said, which is `aria-live`.
	if !strings.Contains(orderJS, "ctx.actionNote.textContent = 'Ryd filteret for at flytte billeder.") {
		t.Error("the refusal has to explain itself in Danish, in the action line: a drag that silently does " +
			"nothing is indistinguishable from a broken tool")
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

// **No public page links to the curator's tool** (task 396). The curator pages sit under the same year prefix
// as the public ones, so a link is one careless template edit away — and it would be an invitation: a visitor who
// finds `/2026/photos` gets a credential prompt for the whole archive.
//
// Walked with the privacy walk's own fixture and enumeration, so every public page is covered, including ones
// added later. The needles come from the route table: every admin page under the year, with its parameters
// filled, plus the `/admin` namespace.
func TestNoPublicPageLinksToTheCuratorsTool(t *testing.T) {
	_, srv := leakTestApp(t)

	needles := []string{`href="/admin`, `hx-get="/admin`, `action="/admin`}
	for _, r := range allRegisteredRoutes(t) {
		if r.admin && looksLikeYearPrefix(r.path) {
			needles = append(needles, concreteURL(r.path))
		}
	}
	if len(needles) < 6 {
		t.Fatalf("expected the curator's pages among the needles, got %v", needles)
	}

	for _, route := range publicRoutePaths(t) {
		_, body := getPublic(t, srv.URL+concreteURL(route.path), nil)
		for _, needle := range needles {
			if strings.Contains(string(body), needle) {
				t.Errorf("%s (routes.go:%d) links to the curator's tool: %q", route.path, route.line, needle)
			}
		}
	}
}

// The sheet keeps its clicks (task 406, PRD 022 §7).
//
// # The one thing this wiring must not break
//
// A click on a cell **selects**, and only selects. The whole tool is built on that: a curator sorting three
// hundred photographs clicks constantly, and a gesture that sometimes opened an overlay instead would make the
// single most-used interaction in the tool ambiguous.
//
// The cells now carry the viewer's data attributes, which is exactly the arrangement that could break it —
// `viewer.js` opens on a click on `[data-viewer-item]` by default. `data-viewer-click="none"` is what switches
// that off, so this asserts the two halves together: the sheet is a viewer container, and it is opted out of
// click-to-open.
//
// It also records why there is no expand control in the cell's corner, which is the design task 406 was written
// with: the cell is a `<button>` with `role="option"` inside a `role="listbox"`, so a control in it would be a
// button inside a button — invalid markup — and an `option` may not hold interactive descendants in any case.
func TestTheContactSheetKeepsItsClicksAfterWiringTheViewer(t *testing.T) {
	src := adminPageSource(t)

	if !strings.Contains(src, `data-viewer data-viewer-actions="caption,credit,fullscreen" data-viewer-click="none"`) {
		t.Error(`the sheet must declare data-viewer-click="none": with the cells carrying data-viewer-item, the ` +
			"viewer would otherwise open on a click that is supposed to select (PRD 022 §7)")
	}
	// The viewer is opened from the action bar instead, like every other action in this tool.
	if !strings.Contains(src, `<button type="button" data-act="view">Vis stort</button>`) {
		t.Error("want a Vis stort action on the bar, which is how the viewer is reached without touching the " +
			"selection gesture")
	}

	// No interactive control inside a cell, which is the thing that cannot be built here.
	fragments := adminSource(t, "adminui/fragments.html")
	cell := fragments[strings.Index(fragments, `<button type="button" class="cell`):]
	cell = cell[:strings.Index(cell, "</button>")]
	if strings.Count(cell, "<button") > 1 {
		t.Error("a cell must not contain a button: the cell is itself a button with role=option, so a nested " +
			"one is invalid markup and undescribable to a screen reader")
	}

	// The public page's share control must not leak onto this surface: it shares a public album URL, which an
	// unpublished photograph does not have.
	if strings.Contains(src, `data-viewer-actions="share`) {
		t.Error("the admin tool must not offer the public share control")
	}
}

// The admin cells hand the viewer the same things the public tiles do (task 406, PRD 023 §7.4).
//
// Through the **admin** media route, not the public one, for the reason the album list fragment already records:
// this sheet shows unpublished photographs, and the public route would correctly refuse them. A viewer full of
// broken images is a viewer a curator stops opening.
func TestTheAdminCellsCarryTheViewerContract(t *testing.T) {
	fragments := adminSource(t, "adminui/fragments.html")

	for _, want := range []struct{ needle, why string }{
		{`data-viewer-item data-viewer-id="{{.ID}}"`, "the viewer identifies a photograph by id here"},
		{`data-full="/api/admin/photos/{{.ID}}/media?year={{$.Year}}"`,
			"the display image through the admin route, because the sheet shows unpublished photographs"},
		{`data-thumb="/api/admin/photos/{{.ID}}/media?variant=thumb&amp;year={{$.Year}}"`,
			"and the thumbnail for the filmstrip"},
		{`{{if .Caption}}data-caption="{{.Caption}}"{{end}}`, "the caption, when there is one"},
		{`{{if .Credit}}data-credit="{{.Credit}}"{{end}}`, "and the credit"},
		{`{{if .Deleted}}data-viewer-deleted="true"{{end}}`,
			"a deleted photograph stays visibly deleted in the viewer rather than being presented as live, " +
				"which is how something taken down on purpose gets republished"},
	} {
		if !strings.Contains(fragments, want.needle) {
			t.Errorf("the contact sheet's cell is missing %s — %s", want.needle, want.why)
		}
	}
}

// The caption and the credit are editable in the viewer, and they are **two controls** (tasks 407, 408).
//
// # Why the separateness is the thing asserted
//
// They look like the same widget and are not the same kind of thing:
//
//   - One form writing both fields would let a curator fixing a typo in a caption **blank a credit** by leaving
//     it alone, which is a loss nobody notices until a photographer asks why their name is gone.
//   - The credit is the one field in this tool that publishes a person's name (task 393, PRD 011's single
//     documented exception), so clearing it is its own act rather than "save an empty field" — the rule
//     `creditaction.js` already applies, because removing an attribution should not be something a stray
//     select-all-and-delete does on its way past.
func TestTheViewerEditsCaptionAndCreditSeparately(t *testing.T) {
	// **Comments stripped before searching.** This test caught its own explanatory prose twice while being
	// written — once on `panel.hidden`, once on `lastCredit` — which is the sixth time a guard in this repo has
	// matched the comment that explains it. `withoutComments` (viewer_test.go) is the standing answer.
	src := withoutComments(adminSource(t, "adminui/vieweredit.js"))

	// Two registered actions, not one combined editor.
	for _, name := range []string{"caption:", "credit:"} {
		if !strings.Contains(src, name) {
			t.Errorf("want a %s field spec; the two must be separate controls", name)
		}
	}
	// One request per field, carrying only that field. `body[open.name] = value` is what makes that true for both
	// without two copies of the request.
	if !strings.Contains(src, "body[open.name] = value") {
		t.Error("each save must send only the field being edited, or a caption edit can blank a credit")
	}
	// Clearing is its own button, and only the credit has one.
	if !strings.Contains(src, `clear: 'Fjern fotokredit'`) {
		t.Error("clearing a credit must be its own act, not saving an empty field")
	}
	if !strings.Contains(src, "clear: null") {
		t.Error("the caption has no separate clear button; only the credit's removal is a deliberate act")
	}

	// The credit's warning, and the public form shown while it is typed.
	if !strings.Contains(src, "vises offentligt sammen med billedet") {
		t.Error("the credit field must say that it publishes, in Danish")
	}
	if !strings.Contains(src, "'Offentligt: '") {
		t.Error("a curator writing a colleague's name onto a public page should see the public form as they type")
	}

	// Prefilled from the photograph, never from the sheet's remembered credit. That key exists because a *batch*
	// has no single current value; with one photograph in front of you the honest prefill is its own text.
	for _, forbidden := range []string{"localStorage", "lastCredit"} {
		if strings.Contains(src, forbidden) {
			t.Errorf("the viewer's editor must prefill from the photograph, not from %s", forbidden)
		}
	}

	// Through ctx.fetch, which is what stamps the working year. A direct window.fetch would be a silent
	// wrong-year write, and TestTheAdminScriptsFetchOnlyThroughTheYear covers this file for free because it lives
	// on this side of the boundary rather than inside the shared viewer.
	if !strings.Contains(src, "ctx.fetch('/api/admin/photos'") {
		t.Error("the save must go through ctx.fetch, which carries the year the API refuses a request without")
	}

	// A failed save keeps what was typed. Asserted as the absence of a reset on the error path, which is the only
	// way this can be checked without executing it.
	errPath := src[strings.Index(src, "if (!res.ok)"):]
	errPath = errPath[:strings.Index(errPath, "return;")]
	if strings.Contains(errPath, ".value = ") {
		t.Error("a failed save must keep the typed text: a caption is a sentence somebody composed, and losing " +
			"it to a dropped hotel connection is what makes a curator stop trusting the tool")
	}
}

// The editors are registered from the admin side, and the shared viewer stays free of admin concepts.
//
// This is the boundary `TestTheViewerKnowsNothingAboutItsSurfaces` protects, seen from the other side: the write
// is behind the admin credential and needs the working year, so it belongs to the tool — and keeping it here is
// what lets the public album page load the same viewer file without loading the code for an editing control.
func TestTheViewerEditorsLiveOnTheAdminSide(t *testing.T) {
	if src := withoutComments(viewerAsset(t, "viewer.js")); strings.Contains(src, "photoIds") {
		t.Error("the shared viewer must not know the admin write's shape; the editors register from the tool")
	}
	src := withoutComments(adminSource(t, "adminui/vieweredit.js"))
	if !strings.Contains(src, "window.hejViewer.register(") {
		t.Error("the editors must plug into the viewer through its registry rather than being built into it")
	}
	// The sheet is reloaded once, on close, rather than after every save — swapping 120 thumbnails out from under
	// somebody still looking at one, to change an attribute nobody can see on a cell, is the wrong trade.
	if !strings.Contains(src, "ctx.reloadSheet()") {
		t.Error("a saved edit must reconcile with the server through the sheet's existing refresh")
	}
	if !strings.Contains(src, "hv:close") {
		t.Error("the refresh belongs on the viewer closing, not on each save")
	}
}
