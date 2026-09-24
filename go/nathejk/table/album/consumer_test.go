package album

import (
	"strings"
	"testing"
	"time"

	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/cqrs/cqrstest"

	"nathejk.dk/nathejk/table/photo"
)

func fold(t *testing.T, subject string, body any) []string {
	t.Helper()

	w := &cqrstest.Writer{}
	c := consumer{w: w}

	msg := cqrstest.NewMessage(cqrs.SubjectFromStr(subject))
	if err := msg.SetBody(body); err != nil {
		t.Fatalf("SetBody: %v", err)
	}
	if err := c.HandleMessage(msg); err != nil {
		t.Fatalf("HandleMessage(%s): %v", subject, err)
	}
	return w.Statements
}

func foldErr(t *testing.T, subject string, body any) error {
	t.Helper()

	w := &cqrstest.Writer{}
	c := consumer{w: w}

	msg := cqrstest.NewMessage(cqrs.SubjectFromStr(subject))
	if err := msg.SetBody(body); err != nil {
		t.Fatalf("SetBody: %v", err)
	}
	return c.HandleMessage(msg)
}

var at = time.Date(2026, 9, 19, 21, 30, 0, 0, time.UTC)

func ref(c string) string { return strings.Repeat(c, 64) }

func str(s string) *string { return &s }
func b(v bool) *bool       { return &v }
func i(v int) *int         { return &v }

// There was an `f(float64) *float64` here too, for the coordinates `ItemAdded` used to carry. It went with
// them to the photo package (PRD 022 §8.3) — `staticcheck` is what noticed, via the dev container's gates.

func TestCreatedWritesTheAlbum(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.created", Created{
		AlbumID: "al-1", Year: "2026", Slug: "loerdag-morgen",
		Title: "Lørdag morgen", Description: "Da solen kom", SortOrder: 10, CreatedAt: at,
	})

	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	for _, want := range []string{
		"INSERT INTO album", "ON DUPLICATE KEY UPDATE",
		`"al-1"`, `"2026"`, `"loerdag-morgen"`, `"Lørdag morgen"`, "sortOrder=10",
		`"2026-09-19 21:30:00"`,
	} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmts[0])
		}
	}
}

// An album is created unpublished, and a replayed create must not republish one that was taken down.
// Both follow from `published` and `deleted` being absent from the update clause — the rule the glimt
// fold established for `hiddenAt`.
func TestCreatedDoesNotTouchPublishedOrDeleted(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.created", Created{
		AlbumID: "al-1", Year: "2026", Slug: "en-titel", Title: "En titel", CreatedAt: at,
	})

	clause := stmts[0][strings.Index(stmts[0], "ON DUPLICATE KEY UPDATE"):]
	for _, forbidden := range []string{"published", "deleted"} {
		if strings.Contains(clause, forbidden) {
			t.Errorf("the update clause must not set %s, or a replayed create would undo a takedown\n%s",
				forbidden, clause)
		}
	}
	// And it must not set published on insert either: an album is assembled before it is shown.
	if strings.Contains(stmts[0][:strings.Index(stmts[0], "ON DUPLICATE")], "published=1") {
		t.Error("a create must not publish the album")
	}
}

func TestCreatedFallsBackToTheSubjectID(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-7.created", Created{
		Year: "2026", Slug: "noget", Title: "Noget", CreatedAt: at,
	})
	if !strings.Contains(stmts[0], `"al-7"`) {
		t.Errorf("want the id from the subject\ngot: %s", stmts[0])
	}
}

// The slug is the album's public address. An unusable one is refused rather than stored, because the
// result would be a page nobody can open with nothing to say why.
func TestCreatedRefusesAnUnusableSlug(t *testing.T) {
	for _, slug := range []string{
		"", "Store Bogstaver", "med mellemrum", "med/skråstreg", "æøå", "-foran", "bagved-",
		"dobbelt--bindestreg", strings.Repeat("a", 65), "../../etc/passwd", "spørgsmål?",
	} {
		err := foldErr(t, "NATHEJK.2026.album.al-1.created", Created{
			AlbumID: "al-1", Year: "2026", Slug: slug, Title: "T", CreatedAt: at,
		})
		if err == nil {
			t.Errorf("slug %q should be refused", slug)
		}
	}
}

func TestCreatedAcceptsAUsableSlug(t *testing.T) {
	for _, slug := range []string{"a", "loerdag", "loerdag-morgen", "2026-start", "42"} {
		if err := foldErr(t, "NATHEJK.2026.album.al-1.created", Created{
			AlbumID: "al-1", Year: "2026", Slug: slug, Title: "T", CreatedAt: at,
		}); err != nil {
			t.Errorf("slug %q should be accepted: %v", slug, err)
		}
	}
}

func TestCreatedRefusesAZeroTimestamp(t *testing.T) {
	err := foldErr(t, "NATHEJK.2026.album.al-1.created", Created{
		AlbumID: "al-1", Year: "2026", Slug: "s", Title: "T",
	})
	if err == nil {
		t.Fatal("a zero createdAt must be refused: time.Now() would differ on every replay")
	}
}

// The delta shape's whole point: only what was sent is written.
func TestUpdatedAppliesOnlyTheFieldsItCarries(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.updated", Updated{
		AlbumID: "al-1", Year: "2026", Title: str("Ny titel"), UpdatedAt: at,
	})

	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], `title="Ny titel"`) {
		t.Errorf("want the title set\ngot: %s", stmts[0])
	}
	for _, forbidden := range []string{"description=", "sortOrder=", "published="} {
		if strings.Contains(stmts[0], forbidden) {
			t.Errorf("an update that did not mention %s must not write it\ngot: %s", forbidden, stmts[0])
		}
	}
}

// Clearing a description and renaming an album are different events. If they were not, one would wipe
// the other's field.
func TestUpdatedCanClearAField(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.updated", Updated{
		AlbumID: "al-1", Year: "2026", Description: str(""), UpdatedAt: at,
	})
	if !strings.Contains(stmts[0], `description=""`) {
		t.Errorf("an explicit empty string must be written\ngot: %s", stmts[0])
	}
}

func TestUpdatedPublishes(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.updated", Updated{
		AlbumID: "al-1", Year: "2026", Published: b(true), SortOrder: i(3), UpdatedAt: at,
	})
	if !strings.Contains(stmts[0], "published=1") || !strings.Contains(stmts[0], "sortOrder=3") {
		t.Errorf("want published and sortOrder set\ngot: %s", stmts[0])
	}
}

// A no-op update is a client bug, not a reason to drop a message into a log nobody reads.
func TestUpdatedWithNothingIsANoOp(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.updated", Updated{
		AlbumID: "al-1", Year: "2026", UpdatedAt: at,
	})
	if len(stmts) != 0 {
		t.Fatalf("want no statements, got %v", stmts)
	}
}

// # The item tests after PRD 022
//
// Nine tests used to live here covering coordinates, verdicts, refs and thumbnails, because `album_item`
// carried all of that. They were not deleted — they moved to `nathejk/table/photo/consumer_test.go` with
// the columns they describe, and in one case got stricter on the way (a `none` verdict beside a real
// coordinate is now `unknown` rather than being dropped).
//
// What is left to test here is what an album item still is: a photograph at a position.

func TestItemAddedWritesTheMembership(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.itemadded", ItemAdded{
		AlbumID: "al-1", Year: "2026", Ordinal: 2, PhotoID: ref("a"), AddedAt: at,
	})

	// Two statements since task 386: reinstate an existing membership in place, then create one if there is
	// none. See `handleItemAdded` for why a single upsert cannot express this.
	if len(stmts) != 2 {
		t.Fatalf("want 2 statements (reinstate, then create), got %d: %v", len(stmts), stmts)
	}
	for _, want := range []string{
		"INSERT INTO album_item", "2", `"` + ref("a") + `"`, "0",
	} {
		if !strings.Contains(stmts[1], want) {
			t.Errorf("the insert is missing %s\ngot: %s", want, stmts[1])
		}
	}
	if !strings.Contains(stmts[1], "NOT EXISTS") {
		t.Errorf("the insert must be conditional, or it deadletters on replay\ngot: %s", stmts[1])
	}
}

// The membership carries nothing but the membership. A caption or a coordinate reappearing on this table
// would recreate the divergence PRD 022 §8.3 removed — two albums holding two copies of one fact.
func TestItemAddedWritesNoPhotographFields(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.itemadded", ItemAdded{
		AlbumID: "al-1", Year: "2026", Ordinal: 0, PhotoID: ref("a"), AddedAt: at,
	})
	for _, stmt := range stmts {
		for _, forbidden := range []string{
			"blobRef", "thumbRef", "caption", "width", "height", "bytes",
			"latitude", "longitude", "boundsVerdict",
		} {
			if strings.Contains(stmt, forbidden) {
				t.Errorf("an album item must not carry %s — that is the photograph's\ngot: %s",
					forbidden, stmt)
			}
		}
	}
}

// A pre-PRD-022 event: it named the bytes directly and has no photoId.
//
// Skipped, and **not** refused. The stream library logs a handler error and drops the message, so
// refusing would print a warning per legacy item on every boot — noise that teaches nobody anything and
// buries the errors that matter. There is also nothing to recover: the library row such an event would
// need to point at was never created.
func TestItemAddedSkipsALegacyEvent(t *testing.T) {
	// The old shape, as JSON, because the Go struct no longer has the fields to express it.
	legacy := map[string]any{
		"albumId": "al-1", "year": "2026", "ordinal": 0,
		"ref": ref("a"), "thumbRef": ref("b"), "caption": "Ved posten",
		"width": 1600, "height": 1200, "boundsVerdict": "inside",
		"addedAt": at,
	}

	if err := foldErr(t, "NATHEJK.2026.album.al-1.itemadded", legacy); err != nil {
		t.Fatalf("a legacy item event must not fail the replay: %v", err)
	}
	stmts := fold(t, "NATHEJK.2026.album.al-1.itemadded", legacy)
	if len(stmts) != 0 {
		t.Errorf("a legacy item event must write nothing, got %d: %v", len(stmts), stmts)
	}
}

// A photo id is a content hash, and it is the one string here that could otherwise become a filesystem
// path or reach a URL. Unlike the empty case above, a malformed one is not a legacy event — it is a bug —
// so it is refused loudly.
func TestItemAddedRefusesAnInvalidPhotoID(t *testing.T) {
	for _, id := range []string{
		"short", strings.Repeat("A", 64), strings.Repeat("a", 63), strings.Repeat("g", 64),
		"../../etc/passwd" + strings.Repeat("a", 48),
	} {
		err := foldErr(t, "NATHEJK.2026.album.al-1.itemadded", ItemAdded{
			AlbumID: "al-1", Year: "2026", PhotoID: id, AddedAt: at,
		})
		if err == nil {
			t.Errorf("photoId %q should be refused", id)
		}
	}
}

func TestItemAddedRefusesAZeroTimestamp(t *testing.T) {
	err := foldErr(t, "NATHEJK.2026.album.al-1.itemadded", ItemAdded{
		AlbumID: "al-1", Year: "2026", PhotoID: ref("a"),
	})
	if err == nil {
		t.Error("want an error for a missing addedAt")
	}
}

// Re-adding a photograph undoes its removal, and leaves it where it is.
func TestItemAddedClearsTheDeletedFlag(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.itemadded", ItemAdded{
		AlbumID: "al-1", Year: "2026", Ordinal: 1, PhotoID: ref("a"), AddedAt: at,
	})

	reinstate := stmts[0]
	if !strings.Contains(reinstate, "deleted=0") {
		t.Errorf("re-adding an item must undo its removal\n%s", reinstate)
	}
	// Addressed by photograph, so it finds the row whatever position it now holds.
	if !strings.Contains(reinstate, `photoId="`+ref("a")+`"`) {
		t.Errorf("the reinstatement must match on the photograph\n%s", reinstate)
	}
	// And it must **not** move it. A replayed old add would otherwise drag a photograph back out of the
	// position a later reorder gave it. See handleItemAdded.
	if strings.Contains(reinstate[:strings.Index(reinstate, "WHERE")], "ordinal") {
		t.Errorf("reinstating a membership must not set its ordinal\n%s", reinstate)
	}
}

func TestItemRemovedIsASoftDelete(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.itemremoved", ItemRemoved{
		AlbumID: "al-1", Year: "2026", PhotoID: ref("a"), Ordinal: 2, Reason: "objection", RemovedAt: at,
	})
	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	if strings.Contains(stmts[0], "DELETE FROM") {
		t.Error("removal must be soft, so an accidental takedown is recoverable")
	}
	for _, want := range []string{"UPDATE album_item SET deleted=1", `photoId="` + ref("a") + `"`, `"al-1"`} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmts[0])
		}
	}
	// **Not the ordinal.** After a reorder that names a different photograph, so a removal keyed on the slot
	// takes a live photograph off a public album — silently (task 386).
	if strings.Contains(stmts[0], "ordinal") {
		t.Errorf("a removal must not match on an ordinal when it knows the photograph\ngot: %s", stmts[0])
	}
}

// A removal published before task 386 carries only an ordinal, and is applied by it.
//
// **Applied, not skipped** — the opposite choice from a legacy `itemadded`. Skipping a removal puts a
// photograph somebody asked to have taken down back on a public page, which is the one direction this
// projection must never fail in.
func TestItemRemovedFallsBackToTheOrdinalForALegacyEvent(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.itemremoved", ItemRemoved{
		AlbumID: "al-1", Year: "2026", Ordinal: 2, RemovedAt: at,
	})
	if len(stmts) != 1 {
		t.Fatalf("a legacy removal must still be applied, got %d statements: %v", len(stmts), stmts)
	}
	if !strings.Contains(stmts[0], "ordinal=2") {
		t.Errorf("with no photoId the ordinal is all there is\ngot: %s", stmts[0])
	}
}

// A malformed id is a bug, not a legacy event, and is refused loudly — the same rule handleItemAdded applies.
func TestItemRemovedRefusesAnInvalidPhotoID(t *testing.T) {
	err := foldErr(t, "NATHEJK.2026.album.al-1.itemremoved", ItemRemoved{
		AlbumID: "al-1", Year: "2026", PhotoID: "../../etc/passwd" + strings.Repeat("a", 48), RemovedAt: at,
	})
	if err == nil {
		t.Error("a malformed photoId must be refused")
	}
}

// Deleting an album must mark its items too: the map read looks at items across albums and has no
// reason to join the parent, so an album deleted alone would keep its photographs on the map.
func TestDeletedMarksTheAlbumAndItsItems(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.deleted", Deleted{
		AlbumID: "al-1", Year: "2026", DeletedAt: at,
	})
	if len(stmts) != 2 {
		t.Fatalf("want 2 statements (album and items), got %d: %v", len(stmts), stmts)
	}
	if !strings.Contains(stmts[0], "UPDATE album SET deleted=1, published=0") {
		t.Errorf("want the album marked and unpublished\ngot: %s", stmts[0])
	}
	if !strings.Contains(stmts[1], "UPDATE album_item SET deleted=1") {
		t.Errorf("want the items marked\ngot: %s", stmts[1])
	}
}

func TestSubjectBuildsTheVerbs(t *testing.T) {
	for _, verb := range []string{VerbCreated, VerbUpdated, VerbItemAdded, VerbItemRemoved, VerbDeleted} {
		s, err := Subject("2026", "al-1", verb)
		if err != nil {
			t.Fatalf("Subject(%s): %v", verb, err)
		}
		want := "NATHEJK.2026.album.al-1." + verb
		if s.Subject() != want {
			t.Errorf("got %s, want %s", s.Subject(), want)
		}
	}
}

// An id containing a dot publishes successfully and then quietly stops matching the per-album
// patterns, which would make that album impossible to update or take down.
func TestSubjectRefusesTokensThatWouldSplit(t *testing.T) {
	for _, id := range []string{"", "al.1", "al 1", "al*", "al>", "al\n1"} {
		if _, err := Subject("2026", id, VerbCreated); err == nil {
			t.Errorf("album id %q should be refused", id)
		}
	}
	for _, year := range []string{"", "20.26", "*"} {
		if _, err := Subject(year, "al-1", VerbCreated); err == nil {
			t.Errorf("year %q should be refused", year)
		}
	}
}

// The subjects the consumer subscribes to must be the ones Subject builds, or events publish fine and
// are delivered to nothing.
func TestConsumesCoversEveryVerb(t *testing.T) {
	subscribed := consumer{}.Consumes()

	for _, verb := range []string{VerbCreated, VerbUpdated, VerbItemAdded, VerbItemRemoved, VerbDeleted} {
		built, err := Subject("2026", "al-1", verb)
		if err != nil {
			t.Fatalf("Subject(%s): %v", verb, err)
		}
		matched := false
		for _, pattern := range subscribed {
			if built.Match(strings.ToLower(pattern.Subject())) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("nothing subscribes to %s", built.Subject())
		}
	}
}

func TestPlottableOnlyAcceptsInside(t *testing.T) {
	if !Plottable(BoundsInside) {
		t.Error("inside must be plottable")
	}
	for _, verdict := range []string{BoundsNone, BoundsOutside, BoundsUnknown, "", "yes"} {
		if Plottable(verdict) {
			t.Errorf("%q must not be plottable", verdict)
		}
	}
}

// The verdict constants here are legacy duplicates of `photo`'s, which is their canonical home after
// PRD 022 §8.3. They are kept because this package's querier still selects the column — through a join to
// `photo` — and `cmd/api` has callers written against these names.
//
// Duplicated string constants cannot be checked by the compiler, and the values are compared against each
// other in SQL (`WHERE p.boundsVerdict = ?` is fed `album.BoundsInside`), so a drift would not fail to
// build — it would silently stop matching and quietly empty the public map. Hence a test.
func TestVerdictsAgreeWithThePhotoPackage(t *testing.T) {
	for _, tc := range []struct {
		name         string
		mine, theirs string
	}{
		{"none", BoundsNone, photo.BoundsNone},
		{"inside", BoundsInside, photo.BoundsInside},
		{"outside", BoundsOutside, photo.BoundsOutside},
		{"unknown", BoundsUnknown, photo.BoundsUnknown},
	} {
		if tc.mine != tc.theirs {
			t.Errorf("%s has drifted: album has %q, photo has %q — the join compares these",
				tc.name, tc.mine, tc.theirs)
		}
	}

	// And the rule itself must agree, not just the strings: `Plottable` is applied on both sides of the
	// split and a divergence would mean one of them plots what the other rejects.
	for _, verdict := range []string{BoundsNone, BoundsInside, BoundsOutside, BoundsUnknown, "nonsense"} {
		if Plottable(verdict) != photo.Plottable(verdict) {
			t.Errorf("Plottable(%q) disagrees between the packages", verdict)
		}
	}
}

// The chosen cover (task 396): set, cleared, and a malformed id refused because it is spliced into SQL.
func TestUpdatedChoosesAndClearsTheCover(t *testing.T) {
	cover := ref("a")
	stmts := fold(t, "NATHEJK.2026.album.al-1.updated", Updated{
		AlbumID: "al-1", Year: "2026", CoverPhotoID: str(cover), UpdatedAt: at,
	})
	if len(stmts) != 1 || !strings.Contains(stmts[0], `coverPhotoId="`+cover+`"`) {
		t.Errorf("want the cover chosen\ngot: %v", stmts)
	}
	for _, forbidden := range []string{"title=", "published="} {
		if strings.Contains(stmts[0], forbidden) {
			t.Errorf("choosing a cover must not write %s\ngot: %s", forbidden, stmts[0])
		}
	}

	stmts = fold(t, "NATHEJK.2026.album.al-1.updated", Updated{
		AlbumID: "al-1", Year: "2026", CoverPhotoID: str(""), UpdatedAt: at,
	})
	if !strings.Contains(stmts[0], `coverPhotoId=""`) {
		t.Errorf("an empty cover must clear the choice\ngot: %s", stmts[0])
	}
}

func TestUpdatedRefusesAMalformedCover(t *testing.T) {
	err := foldErr(t, "NATHEJK.2026.album.al-1.updated", Updated{
		AlbumID: "al-1", Year: "2026", CoverPhotoID: str(`x" OR 1=1 --`), UpdatedAt: at,
	})
	if err == nil {
		t.Error("a malformed coverPhotoId must be refused")
	}
}

// pickCover is coverOrder for items already in Go: the choice while it is live, else the first.
func TestPickCover(t *testing.T) {
	items := []Item{{Ordinal: 0, PhotoID: "a"}, {Ordinal: 1, PhotoID: "b"}}
	if got, _ := pickCover(items, "b"); got.PhotoID != "b" {
		t.Errorf("the chosen cover should win, got %s", got.PhotoID)
	}
	if got, _ := pickCover(items, "gone"); got.PhotoID != "a" {
		t.Errorf("a choice that left the album falls back to the first, got %s", got.PhotoID)
	}
	if got, _ := pickCover(items, ""); got.PhotoID != "a" {
		t.Errorf("no choice means the first, got %s", got.PhotoID)
	}
	if _, ok := pickCover(nil, "b"); ok {
		t.Error("an empty album has no cover")
	}
}
