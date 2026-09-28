package main

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"nathejk.dk/nathejk/table/album"
)

// Dragging in an album that sorts itself (task 445, PRD 024 §6 R8), and the control that chooses the mode
// (task 446).

// The server refuses the move, and that is what makes the rule true.
//
// The tool asks the curator first and switches the mode for them, but a rule enforced only by the client is not
// a rule: this endpoint is reachable with `curl` and the admin credential, and an arrangement made in an album
// that still sorts itself is work the next addition destroys.
func TestAdminMoveIsRefusedInAnAlbumThatSortsItself(t *testing.T) {
	for _, mode := range []string{album.SortModeTimeAsc, album.SortModeTimeDesc,
		album.SortModeFilenameAsc, album.SortModeFilenameDesc} {

		curator := newAlbumCurator(&curatedAlbum{
			a: album.CuratorAlbum{ID: "al-1", Title: "Natten", SortMode: mode},
			items: []album.CuratorItem{
				{Ordinal: 0, PhotoID: photoID("a")},
				{Ordinal: 1, PhotoID: photoID("b")},
			},
		})
		_, srv, pub := albumWriteApp(t, curator)

		body := `{"photoIds":["` + photoID("b") + `"],"beforePhotoId":"` + photoID("a") + `"}`
		resp := moveAdmin(t, srv, "/api/admin/albums/al-1/move", body)
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("%s: want 409, got %d", mode, resp.StatusCode)
		}
		// The message has to name the mode: "switch it to manual" is only actionable if the curator knows what
		// the album is currently doing.
		if body := adminBody(t, resp); !strings.Contains(body, mode) {
			t.Errorf("%s: the refusal must name the mode, got %s", mode, body)
		}
		if len(pub.Messages) != 0 {
			t.Errorf("%s: a refused move must publish nothing, got %d events", mode, len(pub.Messages))
		}
	}
}

// And a manual album still moves, which is the case that must not have been broken by adding the check.
func TestAdminMoveStillWorksInAManualAlbum(t *testing.T) {
	curator := newAlbumCurator(&curatedAlbum{
		a: album.CuratorAlbum{ID: "al-1", SortMode: album.SortModeManual},
		items: []album.CuratorItem{
			{Ordinal: 0, PhotoID: photoID("a")},
			{Ordinal: 1, PhotoID: photoID("b")},
		},
	})
	_, srv, pub := albumWriteApp(t, curator)

	body := `{"photoIds":["` + photoID("b") + `"],"beforePhotoId":"` + photoID("a") + `"}`
	if resp := moveAdmin(t, srv, "/api/admin/albums/al-1/move", body); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d: %s", resp.StatusCode, adminBody(t, resp))
	}
	if n := reorderCount(pub); n != 1 {
		t.Errorf("want one reorder, got %d", n)
	}
}

// The five labels exist twice — in the `<select>` a curator chooses from, and in `ctx.sortModeName`, which is
// what the tool says back to them afterwards.
//
// **Deliberately duplicated, and therefore worth a guard.** The markup is what a curator reads while choosing;
// the JavaScript is what they read in the confirmation before a hand move and in the note after a save. They
// cannot share a definition: there is no build step on this surface, so a Go template cannot be called from
// JavaScript. What can be checked is that the two lists say the same thing — and the one that would drift is
// the warning, which is where it matters most.
func TestTheSortModeLabelsAgree(t *testing.T) {
	markup := adminAsset(t, "fragments.html")
	js := stripJSLineComments(adminAsset(t, "main.js"))

	// From `<option value="time-asc"…>Tid, ældste først</option>`, allowing the `selected` conditional between.
	option := regexp.MustCompile(`<option value="([a-z-]+)"[^>]*>([^<]+)</option>`)
	found := map[string]string{}
	for _, m := range option.FindAllStringSubmatch(markup, -1) {
		found[m[1]] = strings.TrimSpace(m[2])
	}

	for _, mode := range album.SortModes() {
		label, ok := found[mode]
		if !ok {
			t.Errorf("the sort control has no option for %q: a mode a curator cannot choose is a mode that "+
				"exists only in the API", mode)
			continue
		}
		// The same string, in the same words, in `ctx.sortModeName`.
		want := `'` + mode + `': '` + label + `',`
		if !strings.Contains(js, want) {
			t.Errorf("main.js does not render %q as %q. The two lists must agree: the curator chooses in the "+
				"markup's words and is warned in these", mode, label)
		}
	}

	// And the map is not larger than the modes, which would mean a label for something unchoosable.
	names := regexp.MustCompile(`'([a-z-]+)': '[^']+',`)
	block := js[strings.Index(js, "ctx.sortModeName = (mode) =>"):]
	block = block[:strings.Index(block, "}[mode]")]
	for _, m := range names.FindAllStringSubmatch(block, -1) {
		if !album.ValidSortMode(m[1]) {
			t.Errorf("main.js names a mode %q that the album package does not have", m[1])
		}
	}
}

// The confirmation is asked at the **end** of the gesture, and the mode is switched **before** the move.
//
// Both are decisions rather than details, and both are invisible in a diff:
//
//   - asking at the start would mean interrupting a pointer drag with a dialog, which is how a drag gets
//     abandoned by accident — and the curator may drop the photograph back where it started, in which case
//     there was nothing to ask;
//   - moving before switching would leave an arrangement in an album that still claims to sort itself, which
//     the next addition silently destroys, after the curator was told the move succeeded.
func TestTheDragConfirmsBeforeSwitchingAnAlbumToManual(t *testing.T) {
	js := stripJSLineComments(adminAsset(t, "albumorder.js"))

	// Asked from `pointerup`, after the drag has ended: the confirmation replaces the request that would
	// otherwise have gone out there.
	up := js[strings.Index(js, "window.addEventListener('pointerup'"):]
	up = up[:strings.Index(up, "window.addEventListener('pointercancel'")]
	for _, want := range []struct{ needle, why string }{
		{"const mode = editor.dataset.sortMode || 'manual';", "the mode is read at the end of the gesture, " +
			"not cached at the start: the curator may have changed it in the editor card since the last drag"},
		{"ctx.openSheet(manualPanel);", "a non-manual album asks before anything is written"},
		{"send(d);", "a manual album is unchanged: it moves straight away"},
	} {
		if !strings.Contains(up, want.needle) {
			t.Errorf("the pointerup handler no longer has %q: %s", want.needle, want.why)
		}
	}
	// Nothing may be written from the gesture itself in a sorted album.
	if strings.Contains(up, "sortMode: 'manual'") {
		t.Error("the mode must not be switched from the pointer handler: the curator has not been asked yet")
	}

	// In the confirm handler, the switch precedes the move, and a failed switch returns before it.
	confirm := js[strings.Index(js, "document.getElementById('domanual')"):]
	confirm = confirm[:strings.Index(confirm, "document.getElementById('closemanual')")]
	patch := strings.Index(confirm, `body: JSON.stringify({ sortMode: 'manual' })`)
	move := strings.Index(confirm, "send(d);")
	if patch < 0 || move < 0 {
		t.Fatal("could not find both the switch and the move in the confirm handler")
	}
	if patch > move {
		t.Error("the album must be switched to manual before the photographs are moved, or an arrangement " +
			"outlives the mode that protects it")
	}
	for _, want := range []string{
		"'Kunne ikke skifte til manuel rækkefølge (fejl '",
		"'Kunne ikke skifte til manuel rækkefølge. Billederne blev ikke flyttet.'",
	} {
		if !strings.Contains(confirm, want) {
			t.Errorf("a failed switch must say the photographs were not moved (%q missing)", want)
		}
	}

	// Declining writes nothing at all — there is nothing to revert, because nothing was sent. Bounded by the
	// dragstart handler rather than by the comment above it, since the comments have been stripped.
	cancel := js[strings.Index(js, "document.getElementById('closemanual')"):]
	cancel = cancel[:strings.Index(cancel, "sheet.addEventListener('dragstart'")]
	if strings.Contains(cancel, "ctx.fetch") {
		t.Error("cancelling must not send anything")
	}
	if !strings.Contains(cancel, "pending = null;") {
		t.Error("cancelling must forget the drop, or a later confirmation would apply it")
	}
}

// The control saves on change, and waits for the **order** rather than for the photographs.
//
// `ctx.settled` is the wrong tool here and reaching for it is the obvious mistake: it waits for photographs to
// exist, and after a re-sort they already did. Their presence says nothing about their positions, so a grid
// reloaded on that signal can still show the old order — which is the complaint tasks 437 and 438 fixed for the
// uploader, in a new place.
func TestTheSortControlSavesAndWaitsForTheNewOrder(t *testing.T) {
	js := stripJSLineComments(adminAsset(t, "albumeditor.js"))

	for _, want := range []struct{ needle, why string }{
		{"sortMode.addEventListener('change'", "the control takes effect immediately, so it saves immediately: " +
			"a visible effect with an unsaved value is the combination nobody can reason about"},
		{"const settled = await ctx.settledOrder(albumId, out.resortedOrder);",
			"the reload must wait for the positions the server published, not for the photographs to exist"},
		{"ctx.reloadSheet();", "and then the grid is refreshed"},
		{"editor.dataset.sortMode = mode;", "the card must remember the new mode, or the next drag asks the " +
			"wrong question — or none"},
	} {
		if !strings.Contains(js, want.needle) {
			t.Errorf("albumeditor.js no longer has %q: %s", want.needle, want.why)
		}
	}

	if strings.Contains(js, "ctx.settled(") {
		t.Error("the sort control must not wait on ctx.settled: the photographs already exist, so it would " +
			"return at once and the grid could show the old order")
	}
}

// An album with no stated mode behaves as `manual`, in every reader.
//
// **The direction of this failure is what makes it worth a test.** The obvious spelling of the rule is
// `a.SortMode != album.SortModeManual`, which reads correctly and makes `""` mean "sorts itself" — so an album
// with nothing stated would refuse a hand move and try to re-sort itself on every addition. That is the safest
// possible input producing the least safe behaviour, which is the worst place to find a bug.
//
// It is not hypothetical: `CuratorAlbum` is built by hand in several places, a projection row written before task
// 442 has the column's default rather than a value from an event, and `album.SortModeOr` is the one answer all of
// them now go through.
func TestAnAlbumWithNoStatedModeIsManual(t *testing.T) {
	if got := album.SortModeOr(""); got != album.SortModeManual {
		t.Errorf(`SortModeOr("") = %q, want manual: "nothing said so" cannot mean "sorts itself"`, got)
	}
	if got := album.SortModeOr("time-by-vibes"); got != album.SortModeManual {
		t.Errorf("SortModeOr of an unknown mode = %q, want manual — a value a newer binary wrote must not "+
			"make this one rearrange an album", got)
	}
	for _, mode := range album.SortModes() {
		if got := album.SortModeOr(mode); got != mode {
			t.Errorf("SortModeOr(%q) = %q, want it unchanged", mode, got)
		}
	}

	// And the move handler agrees: an album with no mode is movable, because it is manual.
	curator := newAlbumCurator(&curatedAlbum{
		a: album.CuratorAlbum{ID: "al-1"}, // no SortMode at all
		items: []album.CuratorItem{
			{Ordinal: 0, PhotoID: photoID("a")},
			{Ordinal: 1, PhotoID: photoID("b")},
		},
	})
	_, srv, _ := albumWriteApp(t, curator)

	body := `{"photoIds":["` + photoID("b") + `"],"beforePhotoId":"` + photoID("a") + `"}`
	if resp := moveAdmin(t, srv, "/api/admin/albums/al-1/move", body); resp.StatusCode != http.StatusNoContent {
		t.Errorf("want 204, got %d: %s", resp.StatusCode, adminBody(t, resp))
	}
}
