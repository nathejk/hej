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
