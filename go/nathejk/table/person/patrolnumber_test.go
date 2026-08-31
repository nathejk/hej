package person

import (
	"strings"
	"testing"

	"github.com/nathejk/shared-go/messages"
)

// The patrol number is what PRD 007's lookup matches a typed "138" against, so these tests
// are about it reaching every member of the team and nothing else.

func TestPatrolNumberAssignedDenormalizesOntoMembers(t *testing.T) {
	stmts := mustHandle(t, "NATHEJK:2026.patrulje.team-9.numberassigned",
		messages.NathejkPatrolNumberAssigned{TeamID: "team-9", TeamNumber: "138"})

	if len(stmts) != 1 {
		t.Fatalf("want one statement, got %d: %v", len(stmts), stmts)
	}
	stmt := stmts[0]

	if !strings.Contains(stmt, `teamNumber="138"`) {
		t.Errorf("want the number set, got: %s", stmt)
	}
	if !strings.Contains(stmt, `teamId="team-9"`) {
		t.Errorf("want the update keyed on teamId, got: %s", stmt)
	}
	if !strings.Contains(stmt, `year="2026"`) {
		t.Errorf("statement must be scoped to the year: %s", stmt)
	}
	// Same rule as teamName and the start event: members arrive on their own events, and a
	// team event must not invent people.
	if strings.Contains(stmt, "INSERT") {
		t.Errorf("a team event must not insert rows: %s", stmt)
	}
}

// The number rides on the subject when the body omits the id, matching how the arm-number
// handler reads it.
func TestPatrolNumberAssignedFallsBackToSubject(t *testing.T) {
	stmts := mustHandle(t, "NATHEJK:2026.patrulje.team-42.numberassigned",
		messages.NathejkPatrolNumberAssigned{TeamNumber: "42"})

	if len(stmts) != 1 {
		t.Fatalf("want one statement, got %d: %v", len(stmts), stmts)
	}
	if !strings.Contains(stmts[0], `teamId="team-42"`) {
		t.Errorf("want the team id taken from the subject, got: %s", stmts[0])
	}
}

// An unnumbered team is a normal state, not an error: klaner and sections never get a
// number, and a patrulje has none until one is assigned.
func TestPatrolNumberAssignedIgnoresEmptyNumber(t *testing.T) {
	stmts := mustHandle(t, "NATHEJK:2026.patrulje.team-9.numberassigned",
		messages.NathejkPatrolNumberAssigned{TeamID: "team-9", TeamNumber: ""})

	if len(stmts) != 0 {
		t.Errorf("want no statement for an empty number, got: %v", stmts)
	}
}

func TestPatrolNumberAssignedIgnoresEmptyTeam(t *testing.T) {
	stmts := mustHandle(t, "NATHEJK:2026.patrulje..numberassigned",
		messages.NathejkPatrolNumberAssigned{TeamNumber: "138"})

	if len(stmts) != 0 {
		t.Errorf("want no statement without a team id, got: %v", stmts)
	}
}

// Replaying the same event must not produce a different result. It is a plain UPDATE, so
// this is cheap to assert and worth having: a replay runs every numberassigned event again.
func TestPatrolNumberAssignedIsIdempotent(t *testing.T) {
	first := mustHandle(t, "NATHEJK:2026.patrulje.team-9.numberassigned",
		messages.NathejkPatrolNumberAssigned{TeamID: "team-9", TeamNumber: "138"})
	second := mustHandle(t, "NATHEJK:2026.patrulje.team-9.numberassigned",
		messages.NathejkPatrolNumberAssigned{TeamID: "team-9", TeamNumber: "138"})

	if len(first) != len(second) || first[0] != second[0] {
		t.Errorf("replaying produced a different statement:\nfirst:  %v\nsecond: %v", first, second)
	}
}

// A renumbering must overwrite rather than accumulate: the last assignment wins, and the
// statement carries no condition that would skip an already-numbered team.
func TestPatrolNumberAssignedOverwritesPreviousNumber(t *testing.T) {
	stmts := mustHandle(t, "NATHEJK:2026.patrulje.team-9.numberassigned",
		messages.NathejkPatrolNumberAssigned{TeamID: "team-9", TeamNumber: "139"})

	stmt := stmts[0]
	if strings.Contains(strings.ToLower(stmt), `teamnumber=""`) {
		t.Errorf("the update must not be conditional on the number being unset: %s", stmt)
	}
	if !strings.Contains(stmt, `teamNumber="139"`) {
		t.Errorf("want the new number, got: %s", stmt)
	}
}
