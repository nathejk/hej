package album

import (
	"strings"
	"testing"
)

// Reordering an album (task 378).
//
// # Why this is one event and not one per moved item
//
// `album_item` is keyed `(albumId, ordinal)` and also holds `UNIQUE (albumId, photoId)`. Moving an item into a
// position another holds collides on whichever of the two keys the write touches first, and MariaDB has no
// deferred constraint check — so there is no ordering of individual updates that avoids it. A plain swap is not
// expressible as two independent writes at all.
//
// These tests pin the consequence: the fold vacates the album's positions before placing any of them.

func TestReorderVacatesBeforePlacing(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.itemsreordered", ItemsReordered{
		AlbumID: "al-1", Year: "2026",
		PhotoIDs:    []string{ref("c"), ref("a")},
		ReorderedAt: at,
	})

	// One offset statement, then one placement per photograph.
	if len(stmts) != 3 {
		t.Fatalf("want 1 offset + 2 placements, got %d: %v", len(stmts), stmts)
	}

	// **The offset comes first.** Without it the first placement lands on an occupied position.
	if !strings.Contains(stmts[0], "ordinal = ordinal +") {
		t.Fatalf("the first statement must vacate the album's positions, got: %s", stmts[0])
	}
	// And it covers the whole album, with no `deleted` filter: a soft-deleted row still occupies its ordinal and
	// its photo id, so leaving it behind would put it in the way of a live item.
	if strings.Contains(stmts[0], "deleted") {
		t.Errorf("the offset must move every row including removed ones\ngot: %s", stmts[0])
	}
	if !strings.Contains(stmts[0], `albumId="al-1"`) || !strings.Contains(stmts[0], `year="2026"`) {
		t.Errorf("the offset must be scoped to this album and year\ngot: %s", stmts[0])
	}

	// Then each photograph is placed at its index.
	if !strings.Contains(stmts[1], "ordinal=0") || !strings.Contains(stmts[1], ref("c")) {
		t.Errorf("want the first named photograph at ordinal 0\ngot: %s", stmts[1])
	}
	if !strings.Contains(stmts[2], "ordinal=1") || !strings.Contains(stmts[2], ref("a")) {
		t.Errorf("want the second named photograph at ordinal 1\ngot: %s", stmts[2])
	}
}

// Placement is scoped by **photo id**, not by the old ordinal. That is what makes the fold idempotent on replay:
// after the first pass the offset has moved, but each photograph is still found by who it is.
func TestReorderFindsPhotographsByIdentity(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.itemsreordered", ItemsReordered{
		AlbumID: "al-1", Year: "2026", PhotoIDs: []string{ref("a")}, ReorderedAt: at,
	})

	place := stmts[1]
	if !strings.Contains(place, "photoId=") {
		t.Errorf("placement must match on the photograph, not its old position\ngot: %s", place)
	}
	// A WHERE naming an ordinal would break a replay, because the offset has already moved it.
	where := place[strings.Index(place, "WHERE"):]
	if strings.Contains(where, "ordinal") {
		t.Errorf("placement must not match on an ordinal, or a replay finds nothing\ngot: %s", where)
	}
}

// The offset has to be larger than any real album, or a vacated row could collide with a placement.
func TestReorderOffsetClearsAnyRealAlbum(t *testing.T) {
	// A curated album is tens of photographs (PRD 011 §6 puts 3–5 albums on the frontpage). A million is ample.
	if reorderOffset < 10000 {
		t.Errorf("the offset (%d) must exceed any plausible album size", reorderOffset)
	}
	// And it must not risk overflowing a signed INT column when added to an existing ordinal.
	const maxInt32 = 1 << 31
	if reorderOffset > maxInt32/2 {
		t.Errorf("the offset (%d) is large enough to overflow the INT column rather than move a row",
			reorderOffset)
	}
}

// An empty order is a no-op rather than an error: an empty album's order is trivially already correct, and
// refusing would turn a harmless client into a dropped message in a log nobody reads.
func TestReorderWithNoItemsIsANoOp(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.itemsreordered", ItemsReordered{
		AlbumID: "al-1", Year: "2026", ReorderedAt: at,
	})
	if len(stmts) != 0 {
		t.Errorf("want no statements for an empty order, got %d: %v", len(stmts), stmts)
	}
}

// A photograph named twice is a contradiction about where it goes, and the fold would silently apply whichever
// came last — leaving an album in an order the curator was never shown.
func TestReorderRefusesADuplicate(t *testing.T) {
	err := foldErr(t, "NATHEJK.2026.album.al-1.itemsreordered", ItemsReordered{
		AlbumID: "al-1", Year: "2026",
		PhotoIDs:    []string{ref("a"), ref("c"), ref("a")},
		ReorderedAt: at,
	})
	if err == nil {
		t.Error("want an error when the same photograph is named twice")
	}
}

// A photo id is a content hash and reaches a SQL statement, so it is validated like every other ref.
func TestReorderRefusesAnInvalidPhotoID(t *testing.T) {
	for _, bad := range []string{"", "short", strings.Repeat("A", 64), "../../etc/passwd"} {
		err := foldErr(t, "NATHEJK.2026.album.al-1.itemsreordered", ItemsReordered{
			AlbumID: "al-1", Year: "2026", PhotoIDs: []string{bad}, ReorderedAt: at,
		})
		if err == nil {
			t.Errorf("want an error for photoId %q", bad)
		}
	}
}

// The verb is subscribed as well as published. An unsubscribed verb is never delivered, which here would mean a
// curator's reordering silently never happening.
func TestReorderIsSubscribed(t *testing.T) {
	var found bool
	for _, s := range (consumer{}).Consumes() {
		if strings.HasSuffix(s.Subject(), ".itemsreordered") {
			found = true
		}
	}
	if !found {
		t.Error("the reorder verb must be subscribed, or the event is delivered to nothing")
	}

	s, err := Subject("2026", "al-1", VerbItemsReordered)
	if err != nil {
		t.Fatalf("Subject: %v", err)
	}
	if !s.Match("nathejk.*.album.*.itemsreordered") {
		t.Errorf("Subject's output does not match the subscribed pattern: %s", s.Subject())
	}
}

// **The slug is frozen at creation**, and the event shape is what enforces it: `Updated` has no slug field, so no
// handler can change an album's public address even by accident.
//
// A retitled album answering 404 is a dead link in a family's chat history, which is the one failure the whole
// slug-versus-id distinction exists to prevent.
func TestTheUpdateEventCannotChangeTheSlug(t *testing.T) {
	// Asserted against the fold's generated SQL rather than the struct, so a field that existed but was written
	// would also be caught.
	stmts := fold(t, "NATHEJK.2026.album.al-1.updated", Updated{
		AlbumID: "al-1", Year: "2026",
		Title: str("En ny titel"), UpdatedAt: at,
	})
	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	if strings.Contains(stmts[0], "slug") {
		t.Errorf("an update must never write the slug\ngot: %s", stmts[0])
	}
}
