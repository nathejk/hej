package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jrgensen/cqrs/cqrstest"
	"nathejk.dk/nathejk/table/album"
	"nathejk.dk/nathejk/table/photo"
)

// Applying the sort (task 444, PRD 024 §6 R2/R7).
//
// Two moments, and no others: a mode change, and an add. Nothing else can invalidate a sorted order — the keys
// are immutable once a photograph is in the library, and a removal does not change the relative order of what
// is left.

// sortableAlbumApp wires an album curator and a library, which is what applying a sort needs: the album's
// items come from one and the newly added photographs' sort keys from the other.
func sortableAlbumApp(t *testing.T, curator *albumCurator, rows []photo.LibraryPhoto) (*application, *httptest.Server, *cqrstest.Publisher) {
	t.Helper()
	app, srv, pub := albumWriteApp(t, curator)
	app.models.PhotoCurator = &libraryCurator{rows: rows}
	return app, srv, pub
}

// A mode change re-sorts the album straight away, as its own event.
func TestAdminSortModeChangeReordersTheAlbum(t *testing.T) {
	curator := newAlbumCurator(&curatedAlbum{
		a: album.CuratorAlbum{ID: "al-1", Title: "Natten", SortMode: album.SortModeManual},
		items: []album.CuratorItem{
			{Ordinal: 0, PhotoID: photoID("c"), SortShotAt: "2026-09-12 23:00:00"},
			{Ordinal: 1, PhotoID: photoID("a"), SortShotAt: "2026-09-12 21:00:00"},
			{Ordinal: 2, PhotoID: photoID("b"), SortShotAt: "2026-09-12 22:00:00"},
		},
	})
	_, srv, pub := sortableAlbumApp(t, curator, nil)

	resp := moveAdmin(t, srv, "/api/admin/albums/al-1", `{"sortMode":"time-asc"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, adminBody(t, resp))
	}

	var out adminAlbumSummary
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if !out.Resorted {
		t.Error("the response must say the album was re-sorted, or the tool cannot tell the curator why the " +
			"grid moved")
	}

	order := reorderedOrder(t, pub)
	want := []string{photoID("a"), photoID("b"), photoID("c")}
	if !equalOrder(order, want) {
		t.Errorf("published order = %v, want capture order %v", order, want)
	}
}

// An album already in the order the mode asks for publishes **nothing**.
//
// Not an optimisation: an event that changes no position is noise on a log that is never rewritten, it makes
// the `resorted` flag a lie, and it costs a projection wait in the tool for no visible change.
func TestAdminSortModeChangePublishesNothingWhenTheOrderIsAlreadyRight(t *testing.T) {
	curator := newAlbumCurator(&curatedAlbum{
		a: album.CuratorAlbum{ID: "al-1", SortMode: album.SortModeManual},
		items: []album.CuratorItem{
			{Ordinal: 0, PhotoID: photoID("a"), SortShotAt: "2026-09-12 21:00:00"},
			{Ordinal: 1, PhotoID: photoID("b"), SortShotAt: "2026-09-12 22:00:00"},
		},
	})
	_, srv, pub := sortableAlbumApp(t, curator, nil)

	resp := moveAdmin(t, srv, "/api/admin/albums/al-1", `{"sortMode":"time-asc"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var out adminAlbumSummary
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if out.Resorted {
		t.Error("nothing moved, so the response must not claim a re-sort")
	}
	if n := reorderCount(pub); n != 0 {
		t.Errorf("an album already in the right order must not publish a reorder, got %d", n)
	}
}

// Switching **to** manual leaves the order exactly as it is. That is what makes manual free, and it is what
// task 445's confirm-then-switch depends on: the hand arrangement the curator is about to make must not be
// pre-empted by a re-sort on the way in.
func TestAdminSwitchingToManualChangesNoOrder(t *testing.T) {
	curator := newAlbumCurator(&curatedAlbum{
		a: album.CuratorAlbum{ID: "al-1", SortMode: album.SortModeTimeAsc},
		items: []album.CuratorItem{
			{Ordinal: 0, PhotoID: photoID("b"), SortShotAt: "2026-09-12 22:00:00"},
			{Ordinal: 1, PhotoID: photoID("a"), SortShotAt: "2026-09-12 21:00:00"},
		},
	})
	_, srv, pub := sortableAlbumApp(t, curator, nil)

	if resp := moveAdmin(t, srv, "/api/admin/albums/al-1", `{"sortMode":"manual"}`); resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if n := reorderCount(pub); n != 0 {
		t.Errorf("switching to manual must not reorder anything: the arrangement is the order (got %d)", n)
	}
}

// Adding to a sorted album places the addition in order rather than at the end (R7).
//
// The load-bearing part is that the newly added photographs are sorted **with** the ones already there, using
// sort keys that cannot come from the album read — the adds were published a moment ago and the projection is
// downstream of the log.
func TestAdminAddingToASortedAlbumInterleavesTheAddition(t *testing.T) {
	curator := newAlbumCurator(&curatedAlbum{
		a: album.CuratorAlbum{ID: "al-1", SortMode: album.SortModeTimeAsc},
		items: []album.CuratorItem{
			{Ordinal: 0, PhotoID: photoID("early"), SortShotAt: "2026-09-12 21:00:00"},
			{Ordinal: 1, PhotoID: photoID("late"), SortShotAt: "2026-09-12 23:00:00"},
		},
	})
	// The photograph being added was taken between the two, so it must land between them.
	rows := []photo.LibraryPhoto{{ID: photoID("middle"), ShotAt: "2026-09-12 22:00:00"}}
	_, srv, pub := sortableAlbumApp(t, curator, rows)

	body := `{"photoIds":["` + photoID("middle") + `"],"albumIds":["al-1"]}`
	resp := postAdmin(t, srv, "/api/admin/albums/items", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, adminBody(t, resp))
	}

	var out addAdminAlbumItemsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(out.Albums) != 1 || !out.Albums[0].Resorted {
		t.Errorf("the response must report the re-sort per album, got %+v", out.Albums)
	}

	order := reorderedOrder(t, pub)
	want := []string{photoID("early"), photoID("middle"), photoID("late")}
	if !equalOrder(order, want) {
		t.Errorf("published order = %v, want %v — the addition belongs between the two, not at the end",
			order, want)
	}
}

// A manual album still appends, exactly as before this feature existed.
func TestAdminAddingToAManualAlbumStillAppends(t *testing.T) {
	curator := newAlbumCurator(&curatedAlbum{
		a: album.CuratorAlbum{ID: "al-1", SortMode: album.SortModeManual},
		items: []album.CuratorItem{
			{Ordinal: 0, PhotoID: photoID("a"), SortShotAt: "2026-09-12 23:00:00"},
		},
	})
	rows := []photo.LibraryPhoto{{ID: photoID("b"), ShotAt: "2026-09-12 21:00:00"}}
	_, srv, pub := sortableAlbumApp(t, curator, rows)

	body := `{"photoIds":["` + photoID("b") + `"],"albumIds":["al-1"]}`
	if resp := postAdmin(t, srv, "/api/admin/albums/items", body); resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	if n := reorderCount(pub); n != 0 {
		t.Errorf("a manual album must not be re-sorted by an addition: the arrangement is the curator's (got %d)", n)
	}
}

// The library being unreadable must not lose the addition, and must not publish a partial order.
//
// The adds are already on the log when the sort runs, so failing the request would tell the curator nothing
// happened when something did. And sorting the list *without* the new photographs would publish an order that
// does not name them — the fold vacates every position before placing the named ones, so they would land in
// the offset range at the end of the album in an order nobody chose. So: skip, and say `resorted: false`.
func TestAdminAddingToASortedAlbumSurvivesAnUnreadableLibrary(t *testing.T) {
	curator := newAlbumCurator(&curatedAlbum{
		a: album.CuratorAlbum{ID: "al-1", SortMode: album.SortModeTimeAsc},
		items: []album.CuratorItem{
			{Ordinal: 0, PhotoID: photoID("a"), SortShotAt: "2026-09-12 21:00:00"},
		},
	})
	app, srv, pub := sortableAlbumApp(t, curator, nil)
	app.models.PhotoCurator = nil // the library is down

	body := `{"photoIds":["` + photoID("b") + `"],"albumIds":["al-1"]}`
	resp := postAdmin(t, srv, "/api/admin/albums/items", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("the addition must still succeed, got %d: %s", resp.StatusCode, adminBody(t, resp))
	}

	var out addAdminAlbumItemsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(out.Albums) != 1 || out.Albums[0].Added != 1 {
		t.Errorf("the photograph must be filed, got %+v", out.Albums)
	}
	if out.Albums[0].Resorted {
		t.Error("nothing was re-sorted, so the response must not say it was")
	}
	if n := reorderCount(pub); n != 0 {
		t.Error("no order may be published without the keys to compute it: an order that omits the new " +
			"photographs would strand them at the end of the album")
	}
}

// reorderedOrder returns the photo ids from the one items-reordered event, failing if there is not exactly one.
func reorderedOrder(t *testing.T, pub *cqrstest.Publisher) []string {
	t.Helper()

	if n := reorderCount(pub); n != 1 {
		t.Fatalf("want exactly one items-reordered event, got %d — one event carries the whole order, so a "+
			"second one means two answers about where the photographs go", n)
	}
	for i, subject := range pub.Subjects() {
		if !strings.Contains(subject, "itemsreordered") {
			continue
		}
		var body album.ItemsReordered
		if err := pub.Messages[i].Body(&body); err != nil {
			t.Fatalf("decoding the reorder: %v", err)
		}
		return body.PhotoIDs
	}
	return nil
}

// reorderCount is how many orders this request published. Asserted rather than assumed in several tests
// above, because "one event carries the whole order" is the property `album_item`'s composite key forces.
func reorderCount(pub *cqrstest.Publisher) int {
	n := 0
	for _, subject := range pub.Subjects() {
		if strings.Contains(subject, "itemsreordered") {
			n++
		}
	}
	return n
}

func equalOrder(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
