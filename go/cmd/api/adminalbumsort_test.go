package main

import (
	"reflect"
	"strings"
	"testing"

	"nathejk.dk/nathejk/table/album"
)

// The album sort (task 443, PRD 024 §6 R2/R4/R6).
//
// A list in, a list out, no database — the same shape as `moveAlbumOrder`'s tests above, and the reason the
// sort is a function rather than an `ORDER BY`.

// sortItem builds one live item. The zero value is a photograph whose file said nothing, which is a case the
// sort has to handle rather than an edge.
func sortItem(id, shotAt, uploadedAt, fileName string) album.CuratorItem {
	return album.CuratorItem{
		PhotoID:        id,
		SortShotAt:     shotAt,
		SortUploadedAt: uploadedAt,
		SortFileName:   fileName,
	}
}

func TestSortAlbumOrderByTime(t *testing.T) {
	// Deliberately given in an order that is neither the answer nor its reverse.
	items := []album.CuratorItem{
		sortItem("c", "2026-09-12 23:41:07", "2026-09-13 09:00:00", "IMG_0003.JPG"),
		sortItem("a", "2026-09-12 21:15:00", "2026-09-13 09:00:01", "IMG_0001.JPG"),
		sortItem("b", "2026-09-12 22:30:30", "2026-09-13 09:00:02", "IMG_0002.JPG"),
	}

	if got, want := sortAlbumOrder(items, album.SortModeTimeAsc), []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("time-asc = %v, want %v", got, want)
	}
	if got, want := sortAlbumOrder(items, album.SortModeTimeDesc), []string{"c", "b", "a"}; !reflect.DeepEqual(got, want) {
		t.Errorf("time-desc = %v, want %v", got, want)
	}
}

// The fallback (PRD 024 §6 R3), which is the reason the rule is in Go rather than a COALESCE in SQL.
//
// A large minority of files carry no capture time. The two keys are comparable because both are the
// projection's fixed-width `YYYY-MM-DD HH:MM:SS`, so a photograph without one still sorts sensibly against
// photographs that have one — for a card uploaded the day after the event, among its neighbours rather than
// at one end.
func TestSortAlbumOrderFallsBackToUploadTime(t *testing.T) {
	items := []album.CuratorItem{
		sortItem("late", "2026-09-12 23:00:00", "2026-09-13 09:00:00", ""),
		// No EXIF at all: a screenshot, a scan, a camera whose clock was never set.
		sortItem("none", "", "2026-09-12 22:00:00", ""),
		sortItem("early", "2026-09-12 21:00:00", "2026-09-13 09:00:02", ""),
	}

	got := sortAlbumOrder(items, album.SortModeTimeAsc)
	if want := []string{"early", "none", "late"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v — a photograph with no capture time sorts by when it was uploaded, which "+
			"puts it among its neighbours rather than at one end", got, want)
	}

	// An album where *nothing* has a capture time is upload order, not an error and not a shuffle. That is
	// every album uploaded before task 441 shipped, so it is the common case for a while.
	noneAtAll := []album.CuratorItem{
		sortItem("second", "", "2026-09-13 09:00:01", ""),
		sortItem("first", "", "2026-09-13 09:00:00", ""),
	}
	if got, want := sortAlbumOrder(noneAtAll, album.SortModeTimeAsc), []string{"first", "second"}; !reflect.DeepEqual(got, want) {
		t.Errorf("with no capture times at all, got %v, want upload order %v", got, want)
	}
}

func TestSortAlbumOrderByFileName(t *testing.T) {
	items := []album.CuratorItem{
		sortItem("b", "", "", "IMG_0002.JPG"),
		sortItem("c", "", "", "img_0003.jpg"),
		sortItem("a", "", "", "IMG_0001.JPG"),
	}

	// Case-insensitively: two cameras in one album must not separate into an upper-case block and a
	// lower-case one (PRD 024 §6 R6).
	if got, want := sortAlbumOrder(items, album.SortModeFilenameAsc), []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("filename-asc = %v, want %v", got, want)
	}
	if got, want := sortAlbumOrder(items, album.SortModeFilenameDesc), []string{"c", "b", "a"}; !reflect.DeepEqual(got, want) {
		t.Errorf("filename-desc = %v, want %v", got, want)
	}

	// Danish filenames sort by their bytes, which is not the Danish alphabet — "æ" lands after "z". Asserted
	// so the behaviour is recorded rather than discovered: a collation-aware order would need the database to
	// do the sorting, which is the design PRD 024 §8 D1 rejected for stronger reasons.
	danish := []album.CuratorItem{
		sortItem("aa", "", "", "ængen.jpg"),
		sortItem("zz", "", "", "zebra.jpg"),
	}
	if got, want := sortAlbumOrder(danish, album.SortModeFilenameAsc), []string{"zz", "aa"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v: the order is by code point, so \"æ\" follows \"z\"", got, want)
	}
}

// Every mode is a **total** order, ties included.
//
// Two photographs from a burst share a capture time to the second, and two cards hold the same filename.
// Without the `photoId` tiebreak the answer would depend on the input order — and an *add* would then
// reshuffle photographs nobody touched, changing the public page for no reason a curator could explain.
func TestSortAlbumOrderBreaksTiesOnPhotoID(t *testing.T) {
	for _, mode := range []string{album.SortModeTimeAsc, album.SortModeTimeDesc,
		album.SortModeFilenameAsc, album.SortModeFilenameDesc} {

		// Identical keys, given in two different input orders.
		forward := []album.CuratorItem{
			sortItem("aaa", "2026-09-12 23:00:00", "2026-09-13 09:00:00", "IMG_0001.JPG"),
			sortItem("bbb", "2026-09-12 23:00:00", "2026-09-13 09:00:00", "IMG_0001.JPG"),
			sortItem("ccc", "2026-09-12 23:00:00", "2026-09-13 09:00:00", "IMG_0001.JPG"),
		}
		reversed := []album.CuratorItem{forward[2], forward[1], forward[0]}

		a := sortAlbumOrder(forward, mode)
		b := sortAlbumOrder(reversed, mode)
		if !reflect.DeepEqual(a, b) {
			t.Errorf("%s: input order changed the result (%v vs %v). Equal keys must be broken by photoId, "+
				"or an add reshuffles photographs nobody touched", mode, a, b)
		}
	}

	// And descending is ascending reversed, tiebreak included — the property a curator would assume, that
	// reversing the mode reverses the album.
	items := []album.CuratorItem{
		sortItem("x", "2026-09-12 23:00:00", "", "same.jpg"),
		sortItem("y", "2026-09-12 23:00:00", "", "same.jpg"),
	}
	asc := sortAlbumOrder(items, album.SortModeFilenameAsc)
	desc := sortAlbumOrder(items, album.SortModeFilenameDesc)
	if len(asc) != 2 || asc[0] != desc[1] || asc[1] != desc[0] {
		t.Errorf("descending must be ascending reversed: asc=%v desc=%v", asc, desc)
	}
}

// Sorting an already-sorted album changes nothing.
//
// This is the property that makes applying the mode on **every add** (R7) safe rather than merely tidy: if a
// re-sort could churn, then adding one photograph to a two-hundred-photograph album would rewrite positions
// throughout it, and every one of those is a public page changing.
func TestSortAlbumOrderIsIdempotent(t *testing.T) {
	items := []album.CuratorItem{
		sortItem("a", "2026-09-12 21:00:00", "", "IMG_0001.JPG"),
		sortItem("b", "2026-09-12 22:00:00", "", "IMG_0002.JPG"),
		sortItem("c", "", "2026-09-13 09:00:00", ""),
	}

	for _, mode := range album.SortModes() {
		once := sortAlbumOrder(items, mode)

		// Feed the result back in, as an add would: same photographs, now in the sorted order.
		byID := map[string]album.CuratorItem{}
		for _, it := range items {
			byID[it.PhotoID] = it
		}
		again := make([]album.CuratorItem, 0, len(once))
		for _, id := range once {
			again = append(again, byID[id])
		}

		if got := sortAlbumOrder(again, mode); !reflect.DeepEqual(got, once) {
			t.Errorf("%s: re-sorting moved things (%v then %v)", mode, once, got)
		}
	}
}

// `manual` is the order it was given, and that is what makes it free.
func TestSortAlbumOrderLeavesAManualArrangementAlone(t *testing.T) {
	items := []album.CuratorItem{
		sortItem("z", "2026-09-12 23:00:00", "", "IMG_0009.JPG"),
		sortItem("a", "2026-09-12 21:00:00", "", "IMG_0001.JPG"),
		sortItem("m", "2026-09-12 22:00:00", "", "IMG_0005.JPG"),
	}

	if got, want := sortAlbumOrder(items, album.SortModeManual), []string{"z", "a", "m"}; !reflect.DeepEqual(got, want) {
		t.Errorf("manual = %v, want the arrangement it was given, %v", got, want)
	}
	// An unrecognised mode behaves the same way rather than shuffling, and this is not a hypothetical:
	// `sortMode` is a column, so a rollback puts a binary in front of a value a newer one wrote. The first
	// draft gave unknown modes an empty key, which made every photograph compare equal and left the `photoId`
	// tiebreak to reorder the whole album **by content hash** — a downgrade silently shuffling curated work.
	if got, want := sortAlbumOrder(items, "time-by-vibes"), []string{"z", "a", "m"}; !reflect.DeepEqual(got, want) {
		t.Errorf("an unknown mode = %v, want the order untouched, %v", got, want)
	}
}

// Removed positions are skipped and nothing else is, including an item whose photograph was deleted from the
// library. Same rule as `moveAlbumOrder`: a removed item left *this album*, and a deleted photograph left
// everywhere — but the position still exists, and the curator's view is what says so.
func TestSortAlbumOrderSkipsRemovedAndKeepsEverythingElse(t *testing.T) {
	items := []album.CuratorItem{
		sortItem("keep", "2026-09-12 22:00:00", "", "IMG_0002.JPG"),
		sortItem("gone", "2026-09-12 21:00:00", "", "IMG_0001.JPG"),
		sortItem("orphan", "", "", ""), // its photograph was deleted: the join found nothing
	}
	items[1].Removed = true
	items[2].PhotoDeleted = true

	got := sortAlbumOrder(items, album.SortModeTimeAsc)
	if want := []string{"orphan", "keep"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v: the removed position is dropped, the orphaned one keeps a place and "+
			"sorts as if it had no key", got, want)
	}

	// An empty album is an empty order, not a nil one — it is published as a list.
	if got := sortAlbumOrder(nil, album.SortModeTimeAsc); got == nil || len(got) != 0 {
		t.Errorf("an empty album must sort to an empty list, got %#v", got)
	}
}

// The sort must not become a second place that decides what a mode means.
//
// `album.SortModes()` is the list, `ValidSortMode` is the gate, and `albumSortKey` answers only "what does
// this mode compare". If a sixth mode is added to the package and not here, it silently sorts on nothing —
// which looks exactly like a working sort on an album whose photographs all have the same key.
func TestEverySortModeHasAKey(t *testing.T) {
	for _, mode := range album.SortModes() {
		if mode == album.SortModeManual {
			continue // manual compares nothing by definition
		}
		// Three items in an input order that is **neither** the ascending nor the descending answer, so that
		// "unchanged" is distinguishable from "sorted" whichever direction the mode runs. Two items cannot do
		// that: for a descending mode the sorted answer and the input can legitimately be the same list,
		// which is what the first draft of this test got wrong.
		items := []album.CuratorItem{
			sortItem("b", "2026-09-12 22:00:00", "2026-09-13 09:00:01", "IMG_0002.JPG"),
			sortItem("a", "2026-09-12 21:00:00", "2026-09-13 09:00:00", "IMG_0001.JPG"),
			sortItem("c", "2026-09-12 23:00:00", "2026-09-13 09:00:02", "IMG_0003.JPG"),
		}
		if got := sortAlbumOrder(items, mode); reflect.DeepEqual(got, []string{"b", "a", "c"}) {
			t.Errorf("%s left the album in its input order: the mode exists in the album package but "+
				"albumSortKey does not know what it compares", mode)
		}
	}

	// And the key function is where that knowledge lives, for every mode the package offers.
	for _, mode := range album.SortModes() {
		if mode == album.SortModeManual {
			continue
		}
		key, ok := albumSortKey(mode)
		if !ok {
			t.Errorf("%s is offered to curators but this binary cannot sort it", mode)
			continue
		}
		if key(sortItem("a", "2026-09-12 21:00:00", "2026-09-13 09:00:00", "IMG_0001.JPG")) == "" {
			t.Errorf("%s has a key function that compares nothing, so every photograph is equal", mode)
		}
	}
}

// The reasoning above is only as good as the mode names matching the package's.
func TestTheSortReadsItsModesFromTheAlbumPackage(t *testing.T) {
	src := stripGoComments(adminSource(t, "adminalbum.go"))

	for _, mode := range []string{"time-asc", "time-desc", "filename-asc", "filename-desc", "manual"} {
		if strings.Contains(src, `"`+mode+`"`) {
			t.Errorf("adminalbum.go spells the mode %q as a literal: use album.SortMode… so that the "+
				"handler and the fold cannot disagree about what a mode is called", mode)
		}
	}
}
