package checkpoint

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/cqrs/cqrstest"
	"github.com/nathejk/shared-go/messages"
	"github.com/nathejk/shared-go/types"
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

func str(s string) *string { return &s }

func TestUpdatedWritesNameAndPosition(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.checkpoint.cp-1.updated",
		messages.NathejkCheckpointUpdated{
			CheckpointID: "cp-1",
			Name:         str("Post 4A"),
			Position:     &types.Coordinate{Latitude: 55.716595, Longitude: 12.264819},
		})

	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	for _, want := range []string{
		"INSERT INTO checkpoint", "ON DUPLICATE KEY UPDATE",
		`"cp-1"`, `"2026"`, `"Post 4A"`, "55.716595", "12.264819",
	} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmts[0])
		}
	}
}

// The property this handler exists for. Every field on NathejkCheckpointUpdated is a pointer,
// so an event that only renames a checkpoint must not erase its position — the race area is
// derived from positions, so blanking one silently shrinks the cached region.
func TestPartialUpdateDoesNotBlankTheOtherFields(t *testing.T) {
	nameOnly := fold(t, "NATHEJK.2026.checkpoint.cp-1.updated",
		messages.NathejkCheckpointUpdated{CheckpointID: "cp-1", Name: str("Renamed")})

	if strings.Contains(nameOnly[0], "latitude") || strings.Contains(nameOnly[0], "longitude") {
		t.Errorf("a name-only update must not touch the position columns\ngot: %s", nameOnly[0])
	}

	positionOnly := fold(t, "NATHEJK.2026.checkpoint.cp-1.updated",
		messages.NathejkCheckpointUpdated{
			CheckpointID: "cp-1",
			Position:     &types.Coordinate{Latitude: 55.8, Longitude: 12.1},
		})

	if strings.Contains(positionOnly[0], "name") {
		t.Errorf("a position-only update must not touch the name\ngot: %s", positionOnly[0])
	}

	// Extended in task 252: the window columns have the same property, and for a sharper reason than
	// the position — a blanked window turns "reached on time" into "no verdict" for every patrol at
	// that post, and nothing about the app would look broken.
	for _, col := range []string{"openFromUts", "openUntilUts", "openDuration", "checkgroupId", "sortOrder"} {
		if strings.Contains(nameOnly[0], col) {
			t.Errorf("a name-only update must not touch %s\ngot: %s", col, nameOnly[0])
		}
	}
}

// A fixed window is stored as two absolute instants, in seconds.
func TestFixedTimeRangeIsStored(t *testing.T) {
	start := time.Date(2026, 6, 19, 20, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 19, 23, 30, 0, 0, time.UTC)

	stmts := fold(t, "NATHEJK.2026.checkpoint.cp-1.updated",
		messages.NathejkCheckpointUpdated{
			CheckpointID:   "cp-1",
			FixedTimeRange: &types.TimeRange{Start: start, End: end},
		})

	for _, want := range []string{
		"openFromUts", "openUntilUts",
		fmt.Sprintf("%d", start.Unix()), fmt.Sprintf("%d", end.Unix()),
	} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmts[0])
		}
	}
}

// A relative window is stored as **minutes**, matching the organizers' own model. Its anchor is the
// patrol's own scan at another checkgroup, so it cannot be resolved here — only the duration can be
// stored, and the verdict logic anchors it per patrol.
func TestRelativeTimeDurationIsStoredInMinutes(t *testing.T) {
	d := 90 * time.Minute

	stmts := fold(t, "NATHEJK.2026.checkpoint.cp-1.updated",
		messages.NathejkCheckpointUpdated{
			CheckpointID:         "cp-1",
			RelativeTimeDuration: &d,
		})

	if !strings.Contains(stmts[0], "openDuration") || !strings.Contains(stmts[0], "90") {
		t.Errorf("want a 90-minute duration\ngot: %s", stmts[0])
	}
	// Nanoseconds would be the accident to make here, and it would read as a window 60 billion times
	// too long rather than as an error.
	if strings.Contains(stmts[0], "5400000000000") {
		t.Errorf("duration stored in nanoseconds\ngot: %s", stmts[0])
	}
}

// The two window shapes are mutually exclusive upstream, and neither is merged into the other on the
// way in: a fixed window is a pair of instants, a relative one a duration with a per-patrol anchor
// this projection cannot see.
func TestFixedAndRelativeAreStoredSeparately(t *testing.T) {
	d := 45 * time.Minute
	stmts := fold(t, "NATHEJK.2026.checkpoint.cp-1.updated",
		messages.NathejkCheckpointUpdated{
			CheckpointID:         "cp-1",
			FixedTimeRange:       &types.TimeRange{Start: time.Unix(1750000000, 0), End: time.Unix(1750003600, 0)},
			RelativeTimeDuration: &d,
		})

	for _, want := range []string{"openFromUts", "openUntilUts", "openDuration"} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmts[0])
		}
	}
}

// 0,0 is the Atlantic off Ghana and is what an unset coordinate serialises to. Storing it
// would drag the convex hull across two continents, which the plausibility bound would then
// reject — losing the whole race area because one checkpoint was not sited yet.
func TestZeroCoordinateIsStoredAsNull(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.checkpoint.cp-1.updated",
		messages.NathejkCheckpointUpdated{
			CheckpointID: "cp-1",
			Position:     &types.Coordinate{Latitude: 0, Longitude: 0},
		})

	if !strings.Contains(stmts[0], "NULL") {
		t.Errorf("0,0 must be stored as NULL\ngot: %s", stmts[0])
	}
	// And it must not be stored as the number 0.
	if strings.Contains(stmts[0], "latitude, longitude) VALUES (0, 0") {
		t.Errorf("0,0 was stored as a real coordinate\ngot: %s", stmts[0])
	}
}

func TestDeletedSoftDeletes(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.checkpoint.cp-1.deleted",
		messages.NathejkCheckpointCreated{CheckpointID: "cp-1"})

	stmt := stmts[0]
	if !strings.Contains(stmt, "deleted=1") {
		t.Errorf("want a soft delete\ngot: %s", stmt)
	}
	if strings.Contains(stmt, "INSERT") {
		t.Errorf("a delete must not invent a checkpoint\ngot: %s", stmt)
	}
	if !strings.Contains(stmt, `checkpointId="cp-1"`) || !strings.Contains(stmt, `year="2026"`) {
		t.Errorf("a delete must be scoped to one checkpoint-year\ngot: %s", stmt)
	}
}

// An update after a delete restores the checkpoint: the last event wins, as in `person`.
func TestUpdateClearsASoftDelete(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.checkpoint.cp-1.updated",
		messages.NathejkCheckpointUpdated{CheckpointID: "cp-1", Name: str("Back")})

	if !strings.Contains(stmts[0], "deleted=VALUES(deleted)") {
		t.Errorf("an update must clear a previous soft delete\ngot: %s", stmts[0])
	}
}

// The id may come from the subject when the body omits it.
func TestFallsBackToTheSubjectId(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.checkpoint.cp-9.updated",
		messages.NathejkCheckpointUpdated{Name: str("No id in body")})

	if !strings.Contains(stmts[0], `"cp-9"`) {
		t.Errorf("want the id from the subject\ngot: %s", stmts[0])
	}
}

// Every statement is re-run on each boot, because projections replay from sequence zero.
func TestStatementsAreIdempotent(t *testing.T) {
	body := messages.NathejkCheckpointUpdated{
		CheckpointID: "cp-1", Name: str("Post"),
		Position: &types.Coordinate{Latitude: 55.8, Longitude: 12.1},
	}
	first := fold(t, "NATHEJK.2026.checkpoint.cp-1.updated", body)
	second := fold(t, "NATHEJK.2026.checkpoint.cp-1.updated", body)

	if first[0] != second[0] {
		t.Errorf("same event produced different statements:\n%s\n%s", first[0], second[0])
	}
	if !strings.Contains(first[0], "ON DUPLICATE KEY UPDATE") {
		t.Errorf("INSERT would fail on the second replay\ngot: %s", first[0])
	}
}

// `.created` **is** consumed, since task 252 — and this test used to assert the opposite.
//
// The original reasoning was sound for what the projection then did: a create carries no name and no
// position, so for a table read only by the race area it could write nothing but an empty row. What
// changed is the reader, not the event — the reveal rule (PRD 016) needs a checkpoint's **checkgroup**,
// which is precisely what a create does carry.
//
// Kept as a test rather than deleted, inverted, because the decision is worth pinning in both
// directions: the subscription is load-bearing now (drop it and every reveal-by-checkgroup silently
// stops working), and the next person to read the old comment should find the reversal recorded rather
// than wonder whether it was an accident.
func TestCreatedIsConsumedForItsCheckgroup(t *testing.T) {
	subjects := make([]string, 0, 4)
	for _, s := range (consumer{}).Consumes() {
		subjects = append(subjects, s.Subject())
	}
	joined := strings.Join(subjects, " ")

	for _, want := range []string{"created", "updated", "deleted", "checkpoints_sorted"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing subscription to %s: %s", want, joined)
		}
	}
}

// The one thing a create is authoritative about. Without it the reveal rule cannot tell which
// checkpoints belong to a checkgroup, so scanning a post would reveal nothing.
func TestCreatedWritesTheCheckgroup(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.checkpoint.cp-1.created",
		messages.NathejkCheckpointCreated{CheckpointID: "cp-1", CheckgroupID: "cg-3"})

	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	for _, want := range []string{"INSERT INTO checkpoint", `"cp-1"`, `"2026"`, `"cg-3"`} {
		if !strings.Contains(stmts[0], want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmts[0])
		}
	}
}

// A create is a placeholder: the operator adds a post, then sites and names it. So a create replayed
// after the update that described it must not blank the description — the same property handleUpdated
// has, for the same reason.
func TestCreatedDoesNotBlankNameOrPosition(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.checkpoint.cp-1.created",
		messages.NathejkCheckpointCreated{CheckpointID: "cp-1", CheckgroupID: "cg-3"})

	for _, forbidden := range []string{"name", "latitude", "longitude", "openFromUts"} {
		if strings.Contains(stmts[0], forbidden) {
			t.Errorf("a create must not touch %s\ngot: %s", forbidden, stmts[0])
		}
	}
}

// A create carrying no checkgroup must not blank the one already stored: the column is what the
// reveal rule reads, and an empty value there makes a checkpoint unreachable by rule 3.
func TestCreatedWithoutACheckgroupLeavesItAlone(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.checkpoint.cp-1.created",
		messages.NathejkCheckpointCreated{CheckpointID: "cp-1"})

	if strings.Contains(stmts[0], "checkgroupId") {
		t.Errorf("checkgroupId must be left alone when absent\ngot: %s", stmts[0])
	}
}

// Position in the list is the order. Deliberately *not* what HQ's handler for this subject does — it
// deletes the checkgroup's checkpoints and applies no order at all (and reads the id from the year
// segment, so the delete matches nothing). Copying that here would delete the positions the offline
// map's race area is derived from.
func TestCheckpointsSortedAppliesPositionAsOrder(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.checkgroup.cg-3.checkpoints_sorted",
		messages.NathejkCheckpointsSorted{
			SortedCheckpointIDs: []types.CheckpointID{"cp-a", "cp-b", "cp-c"},
		})

	if len(stmts) != 3 {
		t.Fatalf("want one statement per named id, got %d: %v", len(stmts), stmts)
	}
	for i, want := range []struct{ order, id string }{
		{"sortOrder=0", `"cp-a"`}, {"sortOrder=1", `"cp-b"`}, {"sortOrder=2", `"cp-c"`},
	} {
		if !strings.Contains(stmts[i], want.order) || !strings.Contains(stmts[i], want.id) {
			t.Errorf("statement %d: want %s and %s\ngot: %s", i, want.order, want.id, stmts[i])
		}
	}
	for _, stmt := range stmts {
		if strings.Contains(stmt, "DELETE") {
			t.Errorf("a reorder must never delete checkpoints — the race area is derived from their "+
				"positions\ngot: %s", stmt)
		}
	}
}

// An unrecognised subject is ignored rather than erroring — a projection that failed on
// messages it does not care about would dead-letter half the stream.
func TestUnrelatedSubjectIsIgnored(t *testing.T) {
	w := &cqrstest.Writer{}
	c := consumer{w: w}
	msg := cqrstest.NewMessage(cqrs.SubjectFromStr("NATHEJK.2026.spejder.member-1.updated"))
	if err := msg.SetBody(map[string]any{"memberId": "member-1"}); err != nil {
		t.Fatalf("SetBody: %v", err)
	}
	if err := c.HandleMessage(msg); err != nil {
		t.Fatalf("want no error, got %v", err)
	}
	if len(w.Statements) != 0 {
		t.Errorf("want no statements, got %v", w.Statements)
	}
}

// A subject with no year has no primary key to write, and must be reported rather than
// guessed at — with the subject attached, since the stream library drops handler errors
// rather than dead-lettering them.
func TestMissingYearIsAnAnnotatedError(t *testing.T) {
	w := &cqrstest.Writer{}
	c := consumer{w: w}
	msg := cqrstest.NewMessage(cqrs.SubjectFromStr("NATHEJK"))
	if err := msg.SetBody(map[string]any{}); err != nil {
		t.Fatalf("SetBody: %v", err)
	}
	err := c.HandleMessage(msg)
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(err.Error(), "NATHEJK") {
		t.Errorf("error must name the subject, got: %v", err)
	}
}
