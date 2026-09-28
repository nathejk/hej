package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"nathejk.dk/nathejk/table/album"
)

// The album's sort mode (PRD 024 §6 R1/R9, task 442) — the data and the reads, not the sorting.

// **The half of R9 that lives in code.** A new album is `time-asc` because this handler chose it; the column
// defaults to `manual` so that every album which already exists keeps the arrangement a curator made by hand.
// If this assertion ever fails because the value moved into the column's default instead, the first add to an
// old album would reshuffle it on the upload-time fallback — the one irreversible thing this feature can do.
func TestAdminCreateAlbumSortsByTimeAscending(t *testing.T) {
	_, srv, pub := albumWriteApp(t, newAlbumCurator())

	if got := postAdmin(t, srv, "/api/admin/albums", `{"title":"Natten"}`).StatusCode; got != http.StatusCreated {
		t.Fatalf("want 201, got %d", got)
	}
	if len(pub.Messages) != 1 {
		t.Fatalf("want 1 event, got %d", len(pub.Messages))
	}
	var created album.Created
	if err := pub.Messages[0].Body(&created); err != nil {
		t.Fatal(err)
	}
	if created.SortMode != album.SortModeTimeAsc {
		t.Errorf("a new album must be created %q, got %q", album.SortModeTimeAsc, created.SortMode)
	}
}

// The mode is the server's choice on create, not the client's: it is a setting, and a setting belongs on the
// edit where it can be changed again. `ReadJSON` refuses unknown fields, so asking is a 400 rather than a
// silently ignored field — the same three layers `published` has.
func TestAdminCreateAlbumRefusesAClientChosenSortMode(t *testing.T) {
	_, srv, _ := albumWriteApp(t, newAlbumCurator())

	body := `{"title":"Målet","sortMode":"filename-asc"}`
	if got := postAdmin(t, srv, "/api/admin/albums", body).StatusCode; got != http.StatusBadRequest {
		t.Errorf("want 400 for a sortMode on create, got %d", got)
	}
	for _, f := range structFieldNames(createAdminAlbumRequest{}) {
		if strings.EqualFold(f, "sortMode") {
			t.Error("createAdminAlbumRequest must have no sortMode field; the mode is chosen here and changed by an edit")
		}
	}
}

func TestAdminSetsAnAlbumSortMode(t *testing.T) {
	curator := newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{
		ID: "al-1", Slug: "natten", Title: "Natten", SortMode: album.SortModeTimeAsc,
	}})
	_, srv, pub := albumWriteApp(t, curator)

	resp := moveAdmin(t, srv, "/api/admin/albums/al-1", `{"sortMode":"filename-desc"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, adminBody(t, resp))
	}

	if len(pub.Messages) != 1 {
		t.Fatalf("want 1 event, got %d", len(pub.Messages))
	}
	var u album.Updated
	if err := pub.Messages[0].Body(&u); err != nil {
		t.Fatal(err)
	}
	if u.SortMode == nil || *u.SortMode != album.SortModeFilenameDesc {
		t.Fatalf("want the update to carry filename-desc, got %+v", u.SortMode)
	}
	if u.Title != nil || u.Published != nil {
		t.Error("changing the mode must carry nothing else")
	}

	// The response reflects the change rather than the stale projection, like the other fields.
	var out adminAlbumSummary
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if out.SortMode != album.SortModeFilenameDesc {
		t.Errorf("want the response to echo filename-desc, got %q", out.SortMode)
	}
}

// Refused, not ignored. A mode nothing implements would leave the album in a state no later recompute can
// explain, and a curator watching the select snap back with no message would assume it worked.
func TestAdminRefusesAnUnknownSortMode(t *testing.T) {
	for _, mode := range []string{`"tid"`, `"time"`, `"TIME-ASC"`, `""`, `"manual "`} {
		curator := newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{ID: "al-1", Slug: "s", Title: "S"}})
		_, srv, pub := albumWriteApp(t, curator)

		resp := moveAdmin(t, srv, "/api/admin/albums/al-1", `{"sortMode":`+mode+`}`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("sortMode %s: want 400, got %d", mode, resp.StatusCode)
		}
		if got := len(pub.Messages); got != 0 {
			t.Errorf("sortMode %s: a refused edit must publish nothing, got %d", mode, got)
		}
	}
}

func TestAdminAcceptsEverySortMode(t *testing.T) {
	for _, mode := range album.SortModes() {
		curator := newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{ID: "al-1", Slug: "s", Title: "S"}})
		_, srv, _ := albumWriteApp(t, curator)

		resp := moveAdmin(t, srv, "/api/admin/albums/al-1", `{"sortMode":"`+mode+`"}`)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("sortMode %q should be accepted, got %d: %s", mode, resp.StatusCode, adminBody(t, resp))
		}
	}
}

// The curator's list carries the mode so the editor card can render it (R10).
func TestAdminAlbumListCarriesTheSortMode(t *testing.T) {
	curator := newAlbumCurator(&curatedAlbum{a: album.CuratorAlbum{
		ID: "al-1", Slug: "natten", Title: "Natten", SortMode: album.SortModeTimeDesc,
	}})
	_, srv, _ := albumWriteApp(t, curator)

	resp := getAdmin(t, srv, "/api/admin/albums", testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var out listAdminAlbumsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(out.Albums) != 1 || out.Albums[0].SortMode != album.SortModeTimeDesc {
		t.Fatalf("want the list to carry time-desc, got %+v", out.Albums)
	}
}
