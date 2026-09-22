package photo

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

// hash is a stand-in for a content-addressed photo id: 64 hex characters, as the real one is.
func hash(c string) string { return strings.Repeat(c, 64) }

func TestUploadedWritesThePhotograph(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".uploaded", Uploaded{
		PhotoID: hash("a"), Year: "2026",
		Ref: ref("b"), ThumbRef: ref("c"),
		Width: 1600, Height: 1067, Bytes: 402_113,
		UploadedAt: at,
	})

	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	for _, want := range []string{
		"INSERT INTO photo", "ON DUPLICATE KEY UPDATE",
		`"` + hash("a") + `"`, `"2026"`, `"` + ref("b") + `"`, `"` + ref("c") + `"`,
		"width=1600", "height=1067", "bytes=402113",
		`"2026-09-19 21:30:00"`,
	} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmts[0])
		}
	}
}

// The rule with the most consequence in this projection.
//
// Because the id is the content hash, a re-upload of the same file republishes the *same* uploaded event.
// If the update clause cleared `deleted`, a photographer re-dragging a folder would silently undo a
// deletion — an objection honoured on Tuesday reversed on Wednesday by somebody who was never told there
// was one.
func TestUploadedDoesNotResurrectADeletedPhotograph(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".uploaded", Uploaded{
		PhotoID: hash("a"), Year: "2026", Ref: ref("b"), UploadedAt: at,
	})

	clause := stmts[0][strings.Index(stmts[0], "ON DUPLICATE KEY UPDATE"):]
	if strings.Contains(clause, "deleted") {
		t.Errorf("the update clause must not set deleted, or a re-upload would undo a takedown\n%s",
			clause)
	}
}

// A re-upload must not discard a curator's words. The upload event carries no caption — there is nothing
// to caption a file with at upload time — so including the column in the update clause would write an
// empty string over an evening's editing.
func TestUploadedDoesNotOverwriteTheCaption(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".uploaded", Uploaded{
		PhotoID: hash("a"), Year: "2026", Ref: ref("b"), UploadedAt: at,
	})

	clause := stmts[0][strings.Index(stmts[0], "ON DUPLICATE KEY UPDATE"):]
	if strings.Contains(clause, "caption") {
		t.Errorf("the update clause must not set caption, or a re-upload would blank it\n%s", clause)
	}
	// It is still set on insert, because the column is NOT NULL and a first arrival needs a value.
	if !strings.Contains(stmts[0][:strings.Index(stmts[0], "ON DUPLICATE")], `caption=""`) {
		t.Error("the insert must give the NOT NULL caption column a value")
	}
}

// There is no publishable state in this projection at all: a photograph reaches the open web only through
// a published album. Asserted rather than assumed, because a `published` column added here later would
// look harmless and would route around the one gate PRD 011 §0b depends on.
func TestUploadedWritesNoPublishedState(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".uploaded", Uploaded{
		PhotoID: hash("a"), Year: "2026", Ref: ref("b"), UploadedAt: at,
	})
	if strings.Contains(stmts[0], "published") {
		t.Errorf("the library has no publishable state\ngot: %s", stmts[0])
	}
}

func TestUploadedFallsBackToTheSubjectID(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("d")+".uploaded", Uploaded{
		Year: "2026", Ref: ref("b"), UploadedAt: at,
	})
	if !strings.Contains(stmts[0], `"`+hash("d")+`"`) {
		t.Errorf("want the id from the subject\ngot: %s", stmts[0])
	}
}

// A ref ends up in a SQL statement and later in a URL path. "../../etc/passwd" is a ref-shaped string.
func TestUploadedRefusesAnInvalidRef(t *testing.T) {
	for _, bad := range []string{
		"", "short", strings.Repeat("g", 64), strings.Repeat("A", 64),
		strings.Repeat("a", 63), strings.Repeat("a", 65), "../../etc/passwd",
	} {
		err := foldErr(t, "NATHEJK.2026.photo."+hash("a")+".uploaded", Uploaded{
			PhotoID: hash("a"), Year: "2026", Ref: bad, UploadedAt: at,
		})
		if err == nil {
			t.Errorf("want an error for ref %q", bad)
		}
	}
}

// A malformed thumbnail costs the thumbnail, not the photograph: readers fall back to the full image.
func TestUploadedBlanksAnInvalidThumbRef(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".uploaded", Uploaded{
		PhotoID: hash("a"), Year: "2026", Ref: ref("b"), ThumbRef: "../../etc/passwd",
		UploadedAt: at,
	})
	if !strings.Contains(stmts[0], `thumbRef=""`) {
		t.Errorf("want a blanked thumbRef\ngot: %s", stmts[0])
	}
}

// No replay-stable fallback exists: time.Now() would differ on every rebuild, and this column orders the
// whole contact sheet.
func TestUploadedRefusesAZeroTimestamp(t *testing.T) {
	err := foldErr(t, "NATHEJK.2026.photo."+hash("a")+".uploaded", Uploaded{
		PhotoID: hash("a"), Year: "2026", Ref: ref("b"),
	})
	if err == nil {
		t.Error("want an error for a missing uploadedAt")
	}
}

func TestUploadedWritesNoCoordinateWhenThereIsNone(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".uploaded", Uploaded{
		PhotoID: hash("a"), Year: "2026", Ref: ref("b"), UploadedAt: at,
	})
	for _, want := range []string{"latitude=NULL", "longitude=NULL", `boundsVerdict="none"`} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmts[0])
		}
	}
}

func TestUploadedWritesACoordinateWithItsVerdict(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".uploaded", Uploaded{
		PhotoID: hash("a"), Year: "2026", Ref: ref("b"), UploadedAt: at,
		Location: &Location{Lat: 55.6761, Lng: 12.5683, BoundsVerdict: BoundsInside},
	})
	for _, want := range []string{"latitude=55.6761", "longitude=12.5683", `boundsVerdict="inside"`} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmts[0])
		}
	}
}

// The failure direction that matters. The map read filters on this column, so an unrecognised verdict must
// make a photograph unplottable rather than plot one whose coordinate was never checked.
func TestAnUnrecognisedVerdictBecomesUnknownNeverInside(t *testing.T) {
	for _, bad := range []string{"", "INSIDE", "Inside", "ok", "yes", "true", "plottable"} {
		stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".uploaded", Uploaded{
			PhotoID: hash("a"), Year: "2026", Ref: ref("b"), UploadedAt: at,
			Location: &Location{Lat: 55.6, Lng: 12.5, BoundsVerdict: bad},
		})
		if !strings.Contains(stmts[0], `boundsVerdict="unknown"`) {
			t.Errorf("verdict %q must become unknown\ngot: %s", bad, stmts[0])
		}
		if strings.Contains(stmts[0], `boundsVerdict="inside"`) {
			t.Fatalf("verdict %q must never become inside", bad)
		}
	}
}

// `none` beside a real coordinate is contradictory, and the coordinate is the more trustworthy half:
// something placed it. Judged unknown rather than silently plotted — or silently dropped.
func TestNoneBesideACoordinateBecomesUnknown(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".uploaded", Uploaded{
		PhotoID: hash("a"), Year: "2026", Ref: ref("b"), UploadedAt: at,
		Location: &Location{Lat: 55.6, Lng: 12.5, BoundsVerdict: BoundsNone},
	})
	if !strings.Contains(stmts[0], `boundsVerdict="unknown"`) {
		t.Errorf("want unknown\ngot: %s", stmts[0])
	}
}

// An out-of-bounds coordinate is kept, not discarded. A curator has to be able to see that a photograph
// was rejected rather than wonder why it is missing from the map.
func TestAnOutOfBoundsCoordinateIsStored(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".uploaded", Uploaded{
		PhotoID: hash("a"), Year: "2026", Ref: ref("b"), UploadedAt: at,
		Location: &Location{Lat: 40.7128, Lng: -74.0060, BoundsVerdict: BoundsOutside},
	})
	for _, want := range []string{"latitude=40.7128", `boundsVerdict="outside"`} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmts[0])
		}
	}
}

func TestUpdatedSetsOnlyTheCaption(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".updated", Updated{
		PhotoID: hash("a"), Year: "2026", Caption: str("Solen over Gribskov"), UpdatedAt: at,
	})

	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], `caption="Solen over Gribskov"`) {
		t.Errorf("want the caption\ngot: %s", stmts[0])
	}
	// The bulk-action hazard: a location must not be touched by a caption edit.
	for _, forbidden := range []string{"latitude", "longitude", "boundsVerdict"} {
		if strings.Contains(stmts[0], forbidden) {
			t.Errorf("a caption edit must not write %s\ngot: %s", forbidden, stmts[0])
		}
	}
}

// The reason `Updated`'s fields are pointers, and the reason it matters more here than anywhere else in
// the service: this is the event a bulk action publishes. Setting a location on forty selected
// photographs must not blank forty captions.
func TestUpdatedSettingALocationDoesNotBlankTheCaption(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".updated", Updated{
		PhotoID: hash("a"), Year: "2026", UpdatedAt: at,
		Location: &Location{Lat: 55.6, Lng: 12.5, BoundsVerdict: BoundsInside},
	})
	if strings.Contains(stmts[0], "caption") {
		t.Errorf("a location edit must not write caption\ngot: %s", stmts[0])
	}
	for _, want := range []string{"latitude=55.6", "longitude=12.5", `boundsVerdict="inside"`} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmts[0])
		}
	}
}

// A coordinate and the judgement made about it are one fact. A write that moved the point without
// restating the verdict would leave a photograph plotted at its old judgement.
func TestUpdatedAlwaysWritesTheVerdictWithTheCoordinate(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".updated", Updated{
		PhotoID: hash("a"), Year: "2026", UpdatedAt: at,
		Location: &Location{Lat: 40.7, Lng: -74.0, BoundsVerdict: BoundsOutside},
	})
	if !strings.Contains(stmts[0], "latitude") || !strings.Contains(stmts[0], "boundsVerdict") {
		t.Errorf("a coordinate and its verdict travel together\ngot: %s", stmts[0])
	}
}

// An update carrying nothing is a no-op, not an error: refusing it would turn a harmless client bug into
// a dropped message in a log nobody reads.
func TestUpdatedWithNothingIsANoOp(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".updated", Updated{
		PhotoID: hash("a"), Year: "2026", UpdatedAt: at,
	})
	if len(stmts) != 0 {
		t.Errorf("want no statements, got %d: %v", len(stmts), stmts)
	}
}

// Clearing resets the verdict along with the NULLs. Leaving `inside` behind would leave the row matching
// the map read's filter with nothing to draw.
func TestLocationClearedResetsTheVerdict(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".locationcleared", LocationCleared{
		PhotoID: hash("a"), Year: "2026", Reason: "forkert fix", ClearedAt: at,
	})

	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	for _, want := range []string{"latitude=NULL", "longitude=NULL", `boundsVerdict="none"`} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmts[0])
		}
	}
	// The reason is a curator's note and has no column. It must not leak into the statement.
	if strings.Contains(stmts[0], "forkert fix") {
		t.Errorf("the reason is not stored\ngot: %s", stmts[0])
	}
}

func TestPatrolTaggedWritesIdAndNumber(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".patroltagged", PatrolTagged{
		PhotoID: hash("a"), Year: "2026", TeamID: "team-9", Number: "42", TaggedAt: at,
	})

	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	for _, want := range []string{
		"INSERT INTO photo_patrol", "ON DUPLICATE KEY UPDATE", `"team-9"`, `"42"`,
	} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmts[0])
		}
	}
}

// `teamNumber` is not unique per year, so a tag holding only the number would be free to start pointing at
// a different patrol. The id is the identity; a tag without one is a row that can never be matched,
// untagged or joined.
func TestPatrolTaggedRequiresATeamID(t *testing.T) {
	err := foldErr(t, "NATHEJK.2026.photo."+hash("a")+".patroltagged", PatrolTagged{
		PhotoID: hash("a"), Year: "2026", Number: "42", TaggedAt: at,
	})
	if err == nil {
		t.Error("want an error for a tag with no teamId")
	}
}

// What makes the bulk tag action safe to re-run over a selection that partly overlaps what is already
// tagged: the upsert converges instead of duplicating, and it supersedes an earlier untag.
func TestPatrolTaggedSupersedesAnUntag(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".patroltagged", PatrolTagged{
		PhotoID: hash("a"), Year: "2026", TeamID: "team-9", Number: "42", TaggedAt: at,
	})
	clause := stmts[0][strings.Index(stmts[0], "ON DUPLICATE KEY UPDATE"):]
	if !strings.Contains(clause, "deleted=0") {
		t.Errorf("re-tagging must clear the removal\n%s", clause)
	}
}

func TestPatrolUntaggedIsASoftDelete(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".patroluntagged", PatrolUntagged{
		PhotoID: hash("a"), Year: "2026", TeamID: "team-9", UntaggedAt: at,
	})
	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "SET deleted=1") || strings.Contains(stmts[0], "DELETE FROM") {
		t.Errorf("want a soft delete\ngot: %s", stmts[0])
	}
	if !strings.Contains(stmts[0], `"team-9"`) {
		t.Errorf("want the team scoped\ngot: %s", stmts[0])
	}
}

func TestDeletedIsASoftDelete(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".deleted", Deleted{
		PhotoID: hash("a"), Year: "2026", Reason: "forælder har bedt om det", DeletedAt: at,
	})

	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], "UPDATE photo SET deleted=1") {
		t.Errorf("want a soft delete\ngot: %s", stmts[0])
	}
	if strings.Contains(stmts[0], "DELETE FROM") {
		t.Errorf("a removal must be recoverable\ngot: %s", stmts[0])
	}
	// The reason has no column and must not leak into the statement.
	if strings.Contains(stmts[0], "forælder") {
		t.Errorf("the reason is not stored\ngot: %s", stmts[0])
	}
}

// Deleting a photograph does not touch its album memberships, and that is deliberate rather than an
// omission: every read resolves the photograph through this table, so the join hides it — while the
// membership rows are exactly what an undelete would need.
func TestDeletedDoesNotTouchAlbumMemberships(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".deleted", Deleted{
		PhotoID: hash("a"), Year: "2026", DeletedAt: at,
	})
	for _, s := range stmts {
		if strings.Contains(s, "album_item") {
			t.Errorf("the photograph's deletion must not destroy which albums it was in\ngot: %s", s)
		}
	}
}

// A year is required on every fold: the year scopes every write, and a message without one would write
// rows no year-scoped read could ever find.
func TestAMessageWithNoYearIsRefused(t *testing.T) {
	err := foldErr(t, "NATHEJK", Uploaded{
		PhotoID: hash("a"), Ref: ref("b"), UploadedAt: at,
	})
	if err == nil {
		t.Error("want an error for a subject with no year")
	}
}

// An unrecognised verb is ignored rather than refused. The projection subscribes to six patterns, so
// anything else arriving is a broker or configuration matter, not a malformed message to log per event.
func TestAnUnknownVerbIsIgnored(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.photo."+hash("a")+".somethingelse", Uploaded{
		PhotoID: hash("a"), Year: "2026", Ref: ref("b"), UploadedAt: at,
	})
	if len(stmts) != 0 {
		t.Errorf("want no statements, got %d: %v", len(stmts), stmts)
	}
}

// Every verb the consumer folds must also be subscribed, and vice versa. The two lists are written in
// different places and drift silently: an unsubscribed verb is never delivered, which would leave a
// photograph somebody took down still on display.
func TestEverySubscribedVerbIsFolded(t *testing.T) {
	subscribed := map[string]bool{}
	for _, s := range (consumer{}).Consumes() {
		parts := s.Parts()
		subscribed[parts[len(parts)-1]] = true
	}

	for _, verb := range []string{
		VerbUploaded, VerbUpdated, VerbLocationCleared,
		VerbPatrolTagged, VerbPatrolUntagged, VerbDeleted,
	} {
		if !subscribed[verb] {
			t.Errorf("verb %q is published by Subject but not subscribed by Consumes", verb)
		}
	}
	if len(subscribed) != 6 {
		t.Errorf("want 6 subscribed verbs, got %d: %v", len(subscribed), subscribed)
	}
}

// An id with a dot publishes fine, still matches NATHEJK.>, and quietly stops matching the per-photo
// patterns — which would make that photograph impossible to edit or take down.
func TestSubjectRefusesUnusableTokens(t *testing.T) {
	for _, tc := range []struct{ year, id, verb string }{
		{"", hash("a"), VerbUploaded},
		{"2026", "", VerbUploaded},
		{"2026", hash("a"), ""},
		{"2026", "with.dot", VerbUploaded},
		{"2026", "with space", VerbUploaded},
		{"2026", "with*star", VerbUploaded},
		{"2026", "with>gt", VerbUploaded},
		{"20.26", hash("a"), VerbUploaded},
	} {
		if _, err := Subject(tc.year, tc.id, tc.verb); err == nil {
			t.Errorf("want an error for Subject(%q, %q, %q)", tc.year, tc.id, tc.verb)
		}
	}
}

func TestSubjectBuildsTheExpectedString(t *testing.T) {
	s, err := Subject("2026", hash("a"), VerbUploaded)
	if err != nil {
		t.Fatalf("Subject: %v", err)
	}
	want := "NATHEJK.2026.photo." + hash("a") + ".uploaded"
	if s.Subject() != want {
		t.Errorf("want %s, got %s", want, s.Subject())
	}
	// And the subject the consumer subscribes to must actually match what Subject builds — the one
	// agreement a typo would break silently, since an unmatched subject is never delivered to anything.
	if !s.Match("nathejk.*.photo.*.uploaded") {
		t.Errorf("Subject's output does not match the subscribed pattern: %s", s.Subject())
	}
}

// Only `inside` is plottable. `unknown` deliberately is not: we could not check it, and plotting an
// unchecked coordinate on a public page is the failure the verdict exists to prevent.
func TestOnlyInsideIsPlottable(t *testing.T) {
	if !Plottable(BoundsInside) {
		t.Error("inside must be plottable")
	}
	for _, verdict := range []string{BoundsNone, BoundsOutside, BoundsUnknown, "", "anything"} {
		if Plottable(verdict) {
			t.Errorf("%q must not be plottable", verdict)
		}
	}
}
