package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"nathejk.dk/nathejk/table/album"
)

// Drag-and-drop in the album view (task 396): the request says what moved and where to, and the server builds the
// album's whole order — because the browser, scrolling in pages, may not hold all of it.

// moveAdmin sends a move request with the credential.
func moveAdmin(t *testing.T, srv *httptest.Server, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPatch, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	// The year the page would stamp on the request (task 392).
	req.Header.Set(adminYearHeader, "2026")
	req.SetBasicAuth(testAdminUser, testAdminPass)
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("PATCH %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func itemsOf(ids ...string) []album.CuratorItem {
	out := make([]album.CuratorItem, len(ids))
	for i, id := range ids {
		out[i] = album.CuratorItem{Ordinal: i, PhotoID: id}
	}
	return out
}

func TestMoveAlbumOrder(t *testing.T) {
	items := itemsOf("a", "b", "c", "d", "e")

	for name, tc := range map[string]struct {
		moving []string
		target string
		after  bool
		want   []string
	}{
		"one to the front":               {[]string{"d"}, "a", false, []string{"d", "a", "b", "c", "e"}},
		"one to the end":                 {[]string{"b"}, "e", true, []string{"a", "c", "d", "e", "b"}},
		"several keep their album order": {[]string{"e", "b"}, "c", false, []string{"a", "b", "e", "c", "d"}},
		"after a neighbour":              {[]string{"a"}, "b", true, []string{"b", "a", "c", "d", "e"}},
	} {
		got, err := moveAlbumOrder(items, tc.moving, tc.target, tc.after)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v, want %v", name, got, tc.want)
		}
	}
}

// Removed items have no place in the order; a photograph deleted from the library keeps its place, which is the
// full reorder's definition of live.
func TestMoveAlbumOrderSkipsRemovedItems(t *testing.T) {
	items := itemsOf("a", "b", "c")
	items[1].Removed = true
	items[2].PhotoDeleted = true

	got, err := moveAlbumOrder(items, []string{"c"}, "a", false)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"c", "a"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if _, err := moveAlbumOrder(items, []string{"b"}, "a", false); err == nil {
		t.Error("a removed item cannot be moved")
	}
}

func TestMoveAlbumOrderRefusesBadMoves(t *testing.T) {
	items := itemsOf("a", "b", "c")
	for name, tc := range map[string]struct {
		moving []string
		target string
	}{
		"target is moving":   {[]string{"a", "b"}, "b"},
		"unknown photograph": {[]string{"x"}, "a"},
		"unknown target":     {[]string{"a"}, "x"},
	} {
		if _, err := moveAlbumOrder(items, tc.moving, tc.target, false); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

// The case the task names: an album of 200, the last twenty dragged to the front, with only the first 120 ever
// loaded in the browser. The event carries all 200, in the new order.
func TestMovingFromTheEndOfALargeAlbumPublishesTheWholeOrder(t *testing.T) {
	ids := make([]string, 200)
	for i := range ids {
		ids[i] = fmt.Sprintf("p%03d", i)
	}
	_, srv, pub := albumWriteApp(t, newAlbumCurator(&curatedAlbum{
		a:     album.CuratorAlbum{ID: "al-1", Slug: "stort", Title: "Stort"},
		items: itemsOf(ids...),
	}))

	moving := `"` + strings.Join(ids[180:], `","`) + `"`
	resp := moveAdmin(t, srv, "/api/admin/albums/al-1/move", `{"photoIds":[`+moving+`],"beforePhotoId":"p000"}`)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d: %s", resp.StatusCode, adminBody(t, resp))
	}
	if len(pub.Messages) != 1 || !strings.Contains(pub.Subjects()[0], ".album.al-1.itemsreordered") {
		t.Fatalf("want one reorder event, got %v", pub.Subjects())
	}
	var ev album.ItemsReordered
	if err := pub.Messages[0].Body(&ev); err != nil {
		t.Fatal(err)
	}
	want := append(append([]string{}, ids[180:]...), ids[:180]...)
	if !reflect.DeepEqual(ev.PhotoIDs, want) {
		t.Errorf("the event must carry the whole album in the new order; got %d ids starting %v", len(ev.PhotoIDs), ev.PhotoIDs[:3])
	}
}

func TestTheMoveRequestNeedsExactlyOneTarget(t *testing.T) {
	_, srv, pub := albumWriteApp(t, newAlbumCurator(&curatedAlbum{
		a: album.CuratorAlbum{ID: "al-1", Slug: "s"}, items: itemsOf("a", "b"),
	}))
	for _, body := range []string{
		`{"photoIds":["a"]}`,
		`{"photoIds":["a"],"beforePhotoId":"b","afterPhotoId":"b"}`,
		`{"photoIds":[],"beforePhotoId":"b"}`,
	} {
		if got := moveAdmin(t, srv, "/api/admin/albums/al-1/move", body).StatusCode; got != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", body, got)
		}
	}
	if got := moveAdmin(t, srv, "/api/admin/albums/al-nope/move", `{"photoIds":["a"],"beforePhotoId":"b"}`).StatusCode; got != http.StatusNotFound {
		t.Errorf("unknown album: want 404, got %d", got)
	}
	if len(pub.Messages) != 0 {
		t.Errorf("a refused move must publish nothing, got %d", len(pub.Messages))
	}
}

// The landing place is a gap in the grid, not a bar on a cell's edge (task 435).
//
// Source-read rather than behavioural, because there is no way to execute a pointer gesture from a Go test. What it
// can pin is that the mechanism is still the one the decision chose — and in particular the two guards against the
// indicator oscillating, which are the non-obvious part and the part a later simplification would remove first.
func TestTheDropIndicatorOpensAGapInTheGrid(t *testing.T) {
	// Comments stripped: this file's prose necessarily names `dropslot` and the classes it replaced, and a needle
	// that matches the paragraph explaining a rule rather than the code implementing it is a mistake this package
	// has made several times.
	js := stripJSLineComments(adminAsset(t, "albumorder.js"))
	css := stripCSSComments(adminAsset(t, "page.css"))

	for _, want := range []struct{ needle, why string }{
		{"slot.className = 'dropslot'", "the gap must be real elements in the grid, or the cells after it do not move"},
		{"if (after) cell.after(frames); else cell.before(frames);", "the frames go where the photographs will land"},
		{"makeSlots(sheet.querySelectorAll('.cell.dragging').length)", "one frame per cell the drag took out of " +
			"the grid is what keeps the album's length unchanged — counted from the grid, not from the selection, " +
			"which may name photographs that are not loaded and so vacated nothing"},
		{"if (gapIsOpen(cell, after)) return;", "without this the gap oscillates: it takes up room, which moves " +
			"the cell under the pointer, which asks for the next gap over, every pointermove"},
		{"if (onSlot(e.clientX, e.clientY)) return;", "the pointer spends most of a drag over the gap it opened, " +
			"and that is not a reason to close it again"},
		// Task 436: the selection leaves the grid and rides the pointer.
		{"const copy = img.cloneNode(false);", "the pointer carries the photographs themselves; clones, so the " +
			"cells they came from cannot be disturbed"},
		{"if (stack.childElementCount >= MAXCARRIED) break;", "the stack is a handful, not an inventory"},
		{"const first = [press.id].concat(moving.filter((id) => id !== press.id));", "the photograph under the " +
			"pointer must be the one the curator took hold of"},
	} {
		if !strings.Contains(js, want.needle) {
			t.Errorf("albumorder.js no longer has %q: %s", want.needle, want.why)
		}
	}

	// The moved photographs leave the layout rather than being dimmed in it (task 436). A dimmed cell still holds its
	// place, so the grid grew by the gap while still showing everything and the arrangement under the pointer was one
	// that could never exist.
	if dragging := ruleFor(t, css, "#sheet .cell.dragging {"); !strings.Contains(dragging, "display: none") {
		t.Errorf("#sheet .cell.dragging must take the cell out of the grid, got %q", dragging)
	}

	// The frames are a thumbnail's shape, so the gap is the size of the hole the move will make rather than a hint
	// beside it, and dashed so an empty frame cannot be read as a photograph that failed to load.
	rule := ruleFor(t, css, "#sheet .dropslot {")
	for _, want := range []string{"aspect-ratio: 4 / 3", "dashed"} {
		if !strings.Contains(rule, want) {
			t.Errorf("#sheet .dropslot should declare %q, got %q", want, rule)
		}
	}

	// And the edge bar is gone from both languages. A leftover rule would be dead CSS; a leftover class would be a
	// second indicator drawn beside the first.
	for name, src := range map[string]string{"albumorder.js": js, "page.css": css} {
		for _, gone := range []string{"drop-before", "drop-after"} {
			if strings.Contains(src, gone) {
				t.Errorf("%s still carries %q, which the gap replaced", name, gone)
			}
		}
	}
}
