package main

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// Uploading straight into an album (task 474).
//
// # What is worth guarding
//
// The feature is a placement plus a batch step, and both of its interesting decisions are invisible from the screen:
// **which** photographs get filed, and **when**. Getting either wrong produces a tool that looks like it worked.
//
//   - File only the newly stored ones and half a card silently stays out of the album, because half a card is
//     routinely already in the library — a colleague's dump, a retry, a second pass.
//   - File the previously-deleted ones and a photograph somebody took down is resurrected into public view.
//   - File before the projection has the new ids and the endpoint refuses photographs that are perfectly fine.
//   - File one at a time and a non-manual album publishes one reorder per photograph, each rewriting every ordinal
//     (PRD 024).
//
// Source-read, because there is no JavaScript runtime in this suite. The endpoint being reused is covered by its own
// tests; what these hold is the client's half of the contract.

// The uploader is on the album view, and it says where the photographs will land.
func TestTheAlbumViewCanUploadIntoTheAlbum(t *testing.T) {
	srv := albumViewApp(t)

	body := adminBody(t, getAdmin(t, srv, "/2026/album/natten/edit", testAdminUser, testAdminPass))

	for _, want := range []struct{ needle, why string }{
		{`id="drop"`, "the uploader is on this view now"},
		{`id="files"`, "including its file input"},
		{`id="uploadnote"`, "and the line the batch's outcome is written to"},
		{"Læg billeder op i albummet", "the heading says what this uploader does that the library's does not"},
		{"Billederne bliver lagt i <strong>Natten</strong>",
			"the hint names the album, because an uploader that silently files somewhere is worse than one that " +
				"does not file at all"},
		{"bliver de lagt i albummet uden at blive uploadet igen",
			"and it says what happens to files that are already in the library, which is most of a second pass"},
	} {
		if !strings.Contains(body, want.needle) {
			t.Errorf("the album view is missing %q: %s", want.needle, want.why)
		}
	}
}

// One uploader, rendered by one template.
//
// Two copies of that markup would be two sets of ids — `drop`, `files`, `prog`, `bar`, `proglabel`, `rows`,
// `uploadnote` — every one of which is looked up by name, so a divergence would be a feature that works on one view
// and silently does nothing on the other.
func TestBothViewsRenderTheSameUploader(t *testing.T) {
	srv := albumViewApp(t)

	album := adminBody(t, getAdmin(t, srv, "/2026/album/natten/edit", testAdminUser, testAdminPass))
	photos := adminBody(t, getAdmin(t, srv, "/2026/photos", testAdminUser, testAdminPass))

	for _, id := range []string{`id="drop"`, `id="files"`, `id="prog"`, `id="bar"`, `id="proglabel"`,
		`id="rows"`, `id="uploadnote"`} {
		if got := strings.Count(album, id); got != 1 {
			t.Errorf("the album view has %d of %s, want exactly 1", got, id)
		}
		if got := strings.Count(photos, id); got != 1 {
			t.Errorf("the all-photos view has %d of %s, want exactly 1", got, id)
		}
	}

	// The template is one definition; the difference between the views is the sentence about the album.
	if strings.Contains(photos, "bliver lagt i <strong>") {
		t.Error("the all-photos view must not claim the photographs are filed anywhere: nothing files them there")
	}
	if !strings.Contains(adminAsset(t, "page.html"), `{{define "uploader"}}`) {
		t.Error("the uploader must be one template used by both views")
	}
}

// **Which photographs are filed, and which are not.**
//
// The two decisions this feature turns on, asserted against the code that makes them.
func TestTheBatchFilesTheLivePhotographsAndNotTheDeletedOnes(t *testing.T) {
	js := stripJSLineComments(adminAsset(t, "upload.js"))

	for _, want := range []struct{ needle, why string }{
		{"if (id && (what === 'stored' || what === 'already')) batch.file.push(id);",
			"both outcomes that leave a live photograph in the library are filed — skipping `already` would file " +
				"half a card and silently leave the rest out"},
		{"tally('already', out.photoId);",
			"so the already-uploaded ones have an id to file at all"},
	} {
		if !strings.Contains(js, want.needle) {
			t.Errorf("upload.js no longer has %q: %s", want.needle, want.why)
		}
	}

	// **`gone` must never be filed.** A photograph a curator deleted is not re-uploaded (PRD 022 §8.5), and filing it
	// into an album would put it back on a public page — the one outcome in this feature that is not merely untidy.
	if strings.Contains(js, "what === 'gone'") && strings.Contains(js, "batch.file.push") {
		lines := strings.Split(js, "\n")
		for i, line := range lines {
			if strings.Contains(line, "batch.file.push") && strings.Contains(line, "gone") {
				t.Errorf("line %d files a previously-deleted photograph into an album: %s", i+1, strings.TrimSpace(line))
			}
		}
	}
}

// The order of the two waits, which is the race this would otherwise reintroduce.
//
// `/api/admin/albums/items` validates photographs against the library projection, so filing an id the fold has not
// reached is a refusal for a photograph that is perfectly fine — task 437's race in a new place. The wait must come
// first, and the filing second.
func TestFilingWaitsForTheProjectionFirst(t *testing.T) {
	js := stripJSLineComments(adminAsset(t, "upload.js"))

	settled := strings.Index(js, "const caught = await ctx.settled(b.stored);")
	filed := strings.Index(js, "filed = await fileIntoAlbum(b.file);")
	if settled < 0 || filed < 0 {
		t.Fatal("upload.js must both wait for the projection and file into the album")
	}
	if settled > filed {
		t.Error("the projection wait must come before the filing, or the endpoint refuses ids the fold has not " +
			"reached yet")
	}

	// One request for the whole batch. One per photograph would publish one reorder per photograph in a non-manual
	// album, each rewriting every ordinal (PRD 024).
	if !strings.Contains(js, "body: JSON.stringify({ photoIds: ids, albumIds: [intoAlbum] }),") {
		t.Error("the batch must be filed in one request, so a non-manual album is re-sorted once")
	}
	// The same endpoint the action bar uses, so a dragged card and a filed selection cannot end up ordered
	// differently.
	if !strings.Contains(js, "'/api/admin/albums/items'") {
		t.Error("filing must reuse the add-to-album endpoint rather than a second path into the same write")
	}
	// And the grid is only reloaded once the album read agrees, or it shows the album without the new photographs.
	if !strings.Contains(js, "await ctx.settledFilter(ids, '&album=' + encodeURIComponent(intoAlbum), true);") {
		t.Error("the filing must wait for the album read, like every other write in this tool (task 457)")
	}
}

// A failure to file says the photographs are up, and names the recovery that works.
//
// The distinction matters to somebody holding a card: the upload succeeded, so "try again" is the wrong advice and
// would risk a second pass over three hundred files. The photographs are in the library and can be filed from the
// action bar.
func TestAFailureToFileDoesNotClaimTheUploadFailed(t *testing.T) {
	js := adminAsset(t, "upload.js")

	if !strings.Contains(js, "Billederne blev lagt op, men kunne ikke lægges i albummet") {
		t.Error("a filing failure must say the upload itself succeeded")
	}
	if !strings.Contains(js, "Vælg dem og brug Tilføj til album.") {
		t.Error("and it must name the recovery: the photographs are in the library, so the action bar files them")
	}
}

// The uploader only files when it is on an album's page.
func TestTheAllPhotosUploaderFilesNothing(t *testing.T) {
	js := stripJSLineComments(adminAsset(t, "upload.js"))

	if !strings.Contains(js, "const intoAlbum = editor ? (editor.dataset.album || '') : '';") {
		t.Error("the target album must come from the editor card, which is the one element that states which " +
			"album a page is about")
	}
	if !strings.Contains(js, "if (intoAlbum && b.file.length) {") {
		t.Error("filing must be conditional on there being an album: on the all-photos view an upload goes to the " +
			"library and nowhere else")
	}
}

// The filename in the viewer's info panel, below the credit (task 475).
//
// # Why the public boundary is the whole test
//
// A filename is the one field in the library whose exception rests entirely on *where it is allowed to be*. Task 448
// admitted it on the maintainer's bound — **"protected by authentication"** — and made that structural: the public
// guard keeps flagging the word, so a public read that grows one fails.
//
// Putting it in the viewer moves it one step closer to a page, and the viewer is a **shared** component: the same
// `viewer.js` runs on the public album page. So what needs asserting is not that the admin viewer shows it — that is
// one attribute and one span — but that the public page never provides it.
func TestTheViewerShowsTheFilenameBelowTheCredit(t *testing.T) {
	js := stripJSLineComments(viewerAsset(t, "viewer.js"))

	for _, want := range []struct{ needle, why string }{
		{"filename: node.getAttribute('data-filename') || '',",
			"read from the tile, so only a host page that provides it has one"},
		{"filename.className = 'hv-filename';", "its own element, so it can be styled and found"},
		{"filename.textContent = item.filename;",
			"rendered as the filename it is: no prefix, nothing that would read as an attribution (task 448's " +
				"second bound)"},
	} {
		if !strings.Contains(js, want.needle) {
			t.Errorf("viewer.js no longer has %q: %s", want.needle, want.why)
		}
	}

	// Below the credit, which is what was asked for — so the credit's block must come first in the panel.
	credit := strings.Index(js, "credit.className = 'hv-credit';")
	filename := strings.Index(js, "filename.className = 'hv-filename';")
	if credit < 0 || filename < 0 || credit > filename {
		t.Error("the filename must be appended after the credit: the info panel renders in append order")
	}

	if !strings.Contains(viewerAsset(t, "viewer.css"), ".hv-filename {") {
		t.Error("the filename needs its own rule, or it renders as the caption's body text")
	}
}

// The curator's tile provides it; **the public album page never does.**
func TestOnlyTheCuratorsTileCarriesAFilename(t *testing.T) {
	// The admin fragment: the attribute is there when the photograph has a filename.
	fragment := adminAsset(t, "fragments.html")
	if !strings.Contains(fragment, `{{if .FileName}}data-filename="{{.FileName}}"{{end}}`) {
		t.Error("the curator's tile must carry the filename for the viewer to read")
	}

	// And the public album page's own template does not mention it at all — not as an attribute, not as a field.
	// Asserted against the template source rather than one rendered page, so an item that happens to have no
	// filename in a fixture cannot make this pass.
	src, err := os.ReadFile("publicsite.go")
	if err != nil {
		t.Fatalf("reading publicsite.go: %v", err)
	}
	// Comments stripped: this file's prose discusses filenames in several places, and a guard that greps for a word
	// finds the paragraph forbidding it. The same trap task 439 records.
	public := stripGoComments(string(src))
	for _, forbidden := range []string{"data-filename", ".FileName", "fileName"} {
		if strings.Contains(public, forbidden) {
			t.Errorf("the public site's template mentions %q: a filename is admin-only (task 448 — the bound is "+
				"that it sits behind the credential)", forbidden)
		}
	}
}

// And rendered, the public album page carries none — the other direction, against a fixture that has one.
func TestThePublicAlbumPageCarriesNoFilename(t *testing.T) {
	app, store := albumApp(t)
	// The public read model has no filename field at all, which is the real guarantee; this sets the *library's*
	// value to something unmistakable so that any future path from one to the other shows up here.
	store.albums[0].items[0].Caption = "Ved målstregen"
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, body := getPublic(t, srv.URL+"/2026/album/loerdag-morgen", nil)
	if strings.Contains(string(body), "data-filename") {
		t.Error("the public album page must not carry a filename")
	}

	// The type itself cannot hold one, which is what makes the above true by construction rather than by omission.
	for _, field := range structFieldNames(publicAlbumItem{}) {
		if strings.Contains(strings.ToLower(field), "filename") {
			t.Errorf("publicAlbumItem gained %q: the public read model must have nowhere to put a filename", field)
		}
	}
}
