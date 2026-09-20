package album

import (
	"strings"
	"testing"
	"time"

	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/cqrs/cqrstest"
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
func f(v float64) *float64 { return &v }

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

func TestItemAddedWritesThePhotograph(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.itemadded", ItemAdded{
		AlbumID: "al-1", Year: "2026", Ordinal: 2,
		Ref: ref("a"), ThumbRef: ref("b"), Caption: "Ved posten",
		Width: 1600, Height: 1200, Bytes: 240000,
		Lat: f(55.7332), Lng: f(12.2648), BoundsVerdict: BoundsInside, AddedAt: at,
	})

	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	for _, want := range []string{
		"INSERT INTO album_item", "ordinal=2", ref("a"), ref("b"), `"Ved posten"`,
		"latitude=55.7332", "longitude=12.2648", `boundsVerdict="inside"`, "deleted=0",
	} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmts[0])
		}
	}
}

// The common case: no coordinate at all.
func TestItemAddedWithoutACoordinateWritesNull(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.itemadded", ItemAdded{
		AlbumID: "al-1", Year: "2026", Ordinal: 0,
		Ref: ref("a"), BoundsVerdict: BoundsNone, AddedAt: at,
	})
	for _, want := range []string{"latitude=NULL", "longitude=NULL", `boundsVerdict="none"`} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmts[0])
		}
	}
}

// An out-of-bounds coordinate is **kept** and not plotted. Discarding it would destroy the evidence a
// curator needs to see that something was rejected rather than mysteriously missing.
func TestItemAddedKeepsAnOutOfBoundsCoordinate(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.itemadded", ItemAdded{
		AlbumID: "al-1", Year: "2026", Ordinal: 0, Ref: ref("a"),
		Lat: f(40.7128), Lng: f(-74.0060), BoundsVerdict: BoundsOutside, AddedAt: at,
	})
	if !strings.Contains(stmts[0], "latitude=40.7128") {
		t.Errorf("the coordinate must be stored even when rejected\ngot: %s", stmts[0])
	}
	if !strings.Contains(stmts[0], `boundsVerdict="outside"`) {
		t.Errorf("want the outside verdict\ngot: %s", stmts[0])
	}
}

// **The failure direction that matters.** A verdict we do not recognise must make the photograph
// unplottable, never plottable — the map read filters on this column.
func TestItemAddedDowngradesAnUnknownVerdict(t *testing.T) {
	for _, verdict := range []string{"", "Inside", "yes", "insid", "true"} {
		stmts := fold(t, "NATHEJK.2026.album.al-1.itemadded", ItemAdded{
			AlbumID: "al-1", Year: "2026", Ordinal: 0, Ref: ref("a"),
			Lat: f(55.7), Lng: f(12.2), BoundsVerdict: verdict, AddedAt: at,
		})
		if !strings.Contains(stmts[0], `boundsVerdict="unknown"`) {
			t.Errorf("verdict %q should become unknown\ngot: %s", verdict, stmts[0])
		}
		if strings.Contains(stmts[0], `boundsVerdict="inside"`) {
			t.Errorf("verdict %q must never become plottable", verdict)
		}
	}
}

// A verdict about a coordinate that did not arrive would claim a judgement about nothing.
func TestItemAddedWithAVerdictButNoCoordinateRecordsNone(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.itemadded", ItemAdded{
		AlbumID: "al-1", Year: "2026", Ordinal: 0, Ref: ref("a"),
		BoundsVerdict: BoundsInside, AddedAt: at,
	})
	if !strings.Contains(stmts[0], `boundsVerdict="none"`) {
		t.Errorf("a verdict without a coordinate must record none\ngot: %s", stmts[0])
	}
}

// Half a coordinate is not a position.
func TestItemAddedIgnoresAHalfCoordinate(t *testing.T) {
	for name, body := range map[string]ItemAdded{
		"latitude only":  {AlbumID: "al-1", Year: "2026", Ref: ref("a"), Lat: f(55.7), BoundsVerdict: BoundsInside, AddedAt: at},
		"longitude only": {AlbumID: "al-1", Year: "2026", Ref: ref("a"), Lng: f(12.2), BoundsVerdict: BoundsInside, AddedAt: at},
	} {
		stmts := fold(t, "NATHEJK.2026.album.al-1.itemadded", body)
		if !strings.Contains(stmts[0], "latitude=NULL") || !strings.Contains(stmts[0], "longitude=NULL") {
			t.Errorf("%s: want both NULL\ngot: %s", name, stmts[0])
		}
		if !strings.Contains(stmts[0], `boundsVerdict="none"`) {
			t.Errorf("%s: want the none verdict\ngot: %s", name, stmts[0])
		}
	}
}

// A ref is the one string here that could become a filesystem path.
func TestItemAddedRefusesABadRef(t *testing.T) {
	for _, r := range []string{
		"", "short", strings.Repeat("A", 64), strings.Repeat("a", 63), strings.Repeat("g", 64),
		"../../etc/passwd" + strings.Repeat("a", 48),
	} {
		err := foldErr(t, "NATHEJK.2026.album.al-1.itemadded", ItemAdded{
			AlbumID: "al-1", Year: "2026", Ref: r, BoundsVerdict: BoundsNone, AddedAt: at,
		})
		if err == nil {
			t.Errorf("ref %q should be refused", r)
		}
	}
}

// A malformed thumbnail costs the thumbnail, not the photograph — the page falls back to the full image.
func TestItemAddedDropsABadThumbRefButKeepsTheItem(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.itemadded", ItemAdded{
		AlbumID: "al-1", Year: "2026", Ref: ref("a"), ThumbRef: "nonsense",
		BoundsVerdict: BoundsNone, AddedAt: at,
	})
	if !strings.Contains(stmts[0], `thumbRef=""`) {
		t.Errorf("want an empty thumbRef\ngot: %s", stmts[0])
	}
	if !strings.Contains(stmts[0], ref("a")) {
		t.Error("the item itself must survive a bad thumbnail ref")
	}
}

// Re-adding at an ordinal supersedes whatever was there, including a removal.
func TestItemAddedClearsTheDeletedFlag(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.itemadded", ItemAdded{
		AlbumID: "al-1", Year: "2026", Ordinal: 1, Ref: ref("a"),
		BoundsVerdict: BoundsNone, AddedAt: at,
	})
	clause := stmts[0][strings.Index(stmts[0], "ON DUPLICATE KEY UPDATE"):]
	if !strings.Contains(clause, "deleted=0") {
		t.Errorf("re-adding an item must undo its removal\n%s", clause)
	}
}

func TestItemRemovedIsASoftDelete(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.album.al-1.itemremoved", ItemRemoved{
		AlbumID: "al-1", Year: "2026", Ordinal: 2, Reason: "objection", RemovedAt: at,
	})
	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	if strings.Contains(stmts[0], "DELETE FROM") {
		t.Error("removal must be soft, so an accidental takedown is recoverable")
	}
	for _, want := range []string{"UPDATE album_item SET deleted=1", "ordinal=2", `"al-1"`} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmts[0])
		}
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
