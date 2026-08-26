package checkpoint

import (
	"strings"
	"testing"

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

// `.created` carries no name and no position (verified against the live stream), so it is
// deliberately not consumed. A subscription to it could only ever write an empty row.
func TestCreatedIsNotConsumed(t *testing.T) {
	subjects := make([]string, 0, 4)
	for _, s := range (consumer{}).Consumes() {
		subjects = append(subjects, s.Subject())
	}
	joined := strings.Join(subjects, " ")

	if strings.Contains(joined, "created") {
		t.Errorf("`.created` must not be subscribed: %s", joined)
	}
	for _, want := range []string{"updated", "deleted"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing subscription to %s: %s", want, joined)
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
