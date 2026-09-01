package person

import (
	"strings"
	"testing"

	"github.com/nathejk/shared-go/messages"
	"github.com/nathejk/shared-go/types"
)

// The member lifecycle (PRD 007, task 174).
//
// These events were lifted from hq's `spejderstatus` package into
// `shared-go/messages/member.go`, so this projection consumes the same bodies hq's own
// projection does. What is tested here is that every transition lands as the status the event
// itself resolves to — not a status this repo decided on, which is the drift the comment above
// handleTeamStarted warns about.

func TestMemberStatusEventsWriteTheStatusTheEventResolvesTo(t *testing.T) {
	// `team.moved` is deliberately absent: it moves the member as well as resolving to a
	// status, so it has its own handler and its own tests below. Every other transition
	// writes nothing but the status.
	tests := []struct {
		subject string
		body    any
		want    types.MemberStatus
	}{
		{
			"NATHEJK:2026.spejder.member-1.withdrawal.requested",
			messages.NathejkMemberWithdrawalRequested{MemberID: "member-1", TeamID: "team-1"},
			types.MemberStatusWaiting,
		},
		{
			"NATHEJK:2026.spejder.member-1.withdrawal.cancelled",
			messages.NathejkMemberWithdrawalCancelled{MemberID: "member-1", TeamID: "team-1"},
			types.MemberStatusRacing,
		},
		{
			"NATHEJK:2026.spejder.member-1.pickup.accepted",
			messages.NathejkMemberPickupAccepted{MemberID: "member-1", TeamID: "team-1"},
			types.MemberStatusTransit,
		},
		{
			"NATHEJK:2026.spejder.member-1.shelter.accepted",
			messages.NathejkMemberShelterAccepted{MemberID: "member-1", TeamID: "team-1"},
			types.MemberStatusSheltered,
		},
		{
			"NATHEJK:2026.spejder.member-1.shelter.placed",
			messages.NathejkMemberShelterPlaced{MemberID: "member-1", TeamID: "team-1", Placement: "Telt 3"},
			types.MemberStatusSheltered,
		},
		{
			"NATHEJK:2026.spejder.member-1.status.overridden",
			messages.NathejkMemberStatusOverridden{MemberID: "member-1", To: types.MemberStatusSheltered},
			types.MemberStatusSheltered,
		},
		{
			"NATHEJK:2026.spejder.member-1.handover.completed",
			messages.NathejkMemberHandoverCompleted{MemberID: "member-1", To: types.MemberStatusReleased},
			types.MemberStatusReleased,
		},
	}

	for _, tc := range tests {
		t.Run(tc.subject, func(t *testing.T) {
			stmts := mustHandle(t, tc.subject, tc.body)

			if len(stmts) != 1 {
				t.Fatalf("want one statement, got %d: %v", len(stmts), stmts)
			}
			stmt := stmts[0]

			if !strings.Contains(stmt, `memberStatus="`+string(tc.want)+`"`) {
				t.Errorf("want status %q, got: %s", tc.want, stmt)
			}
			if !strings.Contains(stmt, `personId="member-1"`) {
				t.Errorf("want the member id from the subject, got: %s", stmt)
			}
			if !strings.Contains(stmt, `year="2026"`) {
				t.Errorf("statement must be scoped to the year: %s", stmt)
			}
			// A lifecycle event must not invent a person: members arrive on their own
			// events, exactly as with team events.
			if strings.Contains(stmt, "INSERT") {
				t.Errorf("a lifecycle event must not insert a row: %s", stmt)
			}
		})
	}
}

// Both endings must be storable, and they are not interchangeable: released means a guardian
// came for them, reunited means their own team reached the finish. PRD 007 marks both as "out of
// the race" and purges the number for either.
func TestHandoverCompletedCarriesEitherEnding(t *testing.T) {
	for _, ending := range []types.MemberStatus{types.MemberStatusReleased, types.MemberStatusReunited} {
		stmts := mustHandle(t, "NATHEJK:2026.spejder.member-1.handover.completed",
			messages.NathejkMemberHandoverCompleted{MemberID: "member-1", To: ending})

		if len(stmts) != 1 || !strings.Contains(stmts[0], `memberStatus="`+string(ending)+`"`) {
			t.Errorf("want %q stored, got: %v", ending, stmts)
		}
	}
}

// The ordering trap hq's projector carries a warning about, having been bitten by it: these
// subjects have an extra event segment, so if they were matched after the four-part
// `spejder.*.updated` / `.deleted` patterns, a withdrawal could be projected as a member update
// or a deletion. This asserts the dispatch resolves them as lifecycle events.
func TestMemberStatusSubjectsAreNotMistakenForUpdateOrDelete(t *testing.T) {
	for _, subject := range []string{
		"NATHEJK:2026.spejder.member-1.withdrawal.requested",
		"NATHEJK:2026.spejder.member-1.status.overridden",
		"NATHEJK:2026.spejder.member-1.handover.completed",
	} {
		stmts := mustHandle(t, subject,
			messages.NathejkMemberStatusOverridden{MemberID: "member-1", To: types.MemberStatusWaiting})

		if len(stmts) != 1 {
			t.Fatalf("%s: want one statement, got %v", subject, stmts)
		}
		stmt := stmts[0]

		// A member update writes name/phone/team columns; a delete sets deleted=1. Either
		// would mean the subject matched the wrong pattern.
		if !strings.Contains(stmt, "memberStatus=") {
			t.Errorf("%s was not handled as a lifecycle event: %s", subject, stmt)
		}
		if strings.Contains(stmt, "deleted=1") {
			t.Errorf("%s was projected as a deletion: %s", subject, stmt)
		}
		if strings.Contains(stmt, "name=") || strings.Contains(stmt, "phone=") {
			t.Errorf("%s was projected as a member update: %s", subject, stmt)
		}
	}
}

// A status this build does not recognise can only mean upstream is ahead of this deployment.
// Dropping it beats storing a value the app would then have to render, and the member's next
// event corrects it.
func TestUnknownStatusIsIgnored(t *testing.T) {
	stmts := mustHandle(t, "NATHEJK:2026.spejder.member-1.status.overridden",
		messages.NathejkMemberStatusOverridden{MemberID: "member-1", To: types.MemberStatus("airlifted")})

	if len(stmts) != 0 {
		t.Errorf("want no statement for an unknown status, got: %v", stmts)
	}
}

// An empty target is the same case: nothing to store.
func TestEmptyOverrideStatusIsIgnored(t *testing.T) {
	stmts := mustHandle(t, "NATHEJK:2026.spejder.member-1.status.overridden",
		messages.NathejkMemberStatusOverridden{MemberID: "member-1"})

	if len(stmts) != 0 {
		t.Errorf("want no statement for an empty status, got: %v", stmts)
	}
}

// Replay reapplies every event, so the write has to be idempotent — which a plain UPDATE is.
func TestMemberStatusIsIdempotent(t *testing.T) {
	first := mustHandle(t, "NATHEJK:2026.spejder.member-1.pickup.accepted",
		messages.NathejkMemberPickupAccepted{MemberID: "member-1"})
	second := mustHandle(t, "NATHEJK:2026.spejder.member-1.pickup.accepted",
		messages.NathejkMemberPickupAccepted{MemberID: "member-1"})

	if len(first) != len(second) || first[0] != second[0] {
		t.Errorf("replaying produced a different statement:\nfirst:  %v\nsecond: %v", first, second)
	}
}

// `team.moved` carries the member to another team, and the team's denormalized labels have to
// follow. Replaces TestTeamMovedDoesNotChangeTheTeam, which pinned the deliberate gap that task
// 178 closed — named here so the history is findable rather than looking like a deleted test.
func TestTeamMovedMovesTheMemberAndCopiesTheTeamLabels(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK:2026.spejder.member-1.team.moved",
		messages.NathejkMemberTeamMoved{MemberID: "member-1", FromTeamID: "team-1", ToTeamID: "team-2"}))

	// The destination team, not the origin.
	if !strings.Contains(stmt, `p.teamId = "team-2"`) {
		t.Errorf("want the member moved to team-2: %s", stmt)
	}
	if strings.Contains(stmt, `p.teamId = "team-1"`) {
		t.Errorf("the member was left on the origin team: %s", stmt)
	}

	// The labels must come with them, or the pane shows the new team's id under the old team's
	// name. They are denormalized from team events, so they are copied from a sibling row.
	if !strings.Contains(stmt, "p.teamName = COALESCE(s.teamName") {
		t.Errorf("teamName is not copied from the destination team: %s", stmt)
	}
	if !strings.Contains(stmt, "p.teamNumber = COALESCE(s.teamNumber") {
		t.Errorf("teamNumber is not copied from the destination team: %s", stmt)
	}

	// A survivor moved into another patrol is still racing, so the status rides along.
	if !strings.Contains(stmt, `p.memberStatus = "racing"`) {
		t.Errorf("want the status written too: %s", stmt)
	}

	// One statement, so a reader never sees a new teamId beside the old team's name.
	if !strings.Contains(stmt, "UPDATE person AS p") {
		t.Errorf("want a single joined update: %s", stmt)
	}
	if !strings.Contains(stmt, `p.personId = "member-1"`) || !strings.Contains(stmt, `p.year = "2026"`) {
		t.Errorf("statement must be scoped to the member and the year: %s", stmt)
	}
	if strings.Contains(stmt, "INSERT") {
		t.Errorf("a move must not invent a person: %s", stmt)
	}
}

// The ordering case: the destination team may be unknown to this projection when the move
// arrives. The move must still happen, with empty labels rather than the old team's — empty is
// honest and self-correcting, wrong is neither.
func TestTeamMovedToAnUnknownTeamLeavesLabelsEmptyNotStale(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK:2026.spejder.member-1.team.moved",
		messages.NathejkMemberTeamMoved{MemberID: "member-1", FromTeamID: "team-1", ToTeamID: "team-unknown"}))

	// A LEFT JOIN is what makes this work with no sibling row; an inner join would drop the
	// move entirely and the member would silently stay put.
	if !strings.Contains(stmt, "LEFT JOIN person AS s") {
		t.Errorf("want a LEFT JOIN so the move survives an unknown destination: %s", stmt)
	}
	// COALESCE to "" rather than leaving the column untouched: an empty name is self-correcting
	// once the team's own events arrive, whereas the old team's name is a lie that never fixes
	// itself.
	if !strings.Contains(stmt, `COALESCE(s.teamName, "")`) {
		t.Errorf("want the label blanked rather than kept stale: %s", stmt)
	}
	if !strings.Contains(stmt, `p.teamId = "team-unknown"`) {
		t.Errorf("the move must still happen: %s", stmt)
	}
}

// A member already on the destination team must not copy their own labels — harmless, but it
// would read as a self-referential update in a log.
func TestTeamMovedExcludesTheMemberFromTheirOwnLabelCopy(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK:2026.spejder.member-1.team.moved",
		messages.NathejkMemberTeamMoved{MemberID: "member-1", ToTeamID: "team-2"}))

	if !strings.Contains(stmt, "s.personId <> p.personId") {
		t.Errorf("want the member excluded from the sibling lookup: %s", stmt)
	}
	// Soft-deleted rows must not be a label source either: a deleted member's team labels are
	// as stale as the row.
	if !strings.Contains(stmt, "s.deleted = 0") {
		t.Errorf("want deleted rows excluded as a label source: %s", stmt)
	}
}

// A move with no destination is not a reason to dead-letter a live event, nor to blank the team
// on the strength of a missing field.
func TestTeamMovedWithoutADestinationWritesOnlyTheStatus(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK:2026.spejder.member-1.team.moved",
		messages.NathejkMemberTeamMoved{MemberID: "member-1", FromTeamID: "team-1"}))

	if !strings.Contains(stmt, `memberStatus="racing"`) {
		t.Errorf("want the status recorded: %s", stmt)
	}
	if strings.Contains(stmt, "teamId") || strings.Contains(stmt, "teamName") {
		t.Errorf("must not touch the team without a destination: %s", stmt)
	}
}

func TestTeamMovedIsIdempotent(t *testing.T) {
	body := messages.NathejkMemberTeamMoved{MemberID: "member-1", FromTeamID: "team-1", ToTeamID: "team-2"}
	first := mustHandle(t, "NATHEJK:2026.spejder.member-1.team.moved", body)
	second := mustHandle(t, "NATHEJK:2026.spejder.member-1.team.moved", body)

	if len(first) != len(second) || first[0] != second[0] {
		t.Errorf("replaying produced a different statement:\nfirst:  %v\nsecond: %v", first, second)
	}
}

// The labels a moved member ends up with are filled in by the team's own handlers, which update
// every row with a matching teamId. This asserts that is still true — it is what bounds the
// staleness window to "until the next team event".
func TestTeamEventsReachAMovedMember(t *testing.T) {
	nameStmt := onlyStatement(t, mustHandle(t, "NATHEJK:2026.patrulje.team-2.updated",
		messages.NathejkTeamUpdated{TeamID: "team-2", Name: "Patrulje Ulvene"}))
	if !strings.Contains(nameStmt, `teamId="team-2"`) {
		t.Errorf("team name update must key on teamId so a moved member is included: %s", nameStmt)
	}

	numberStmt := onlyStatement(t, mustHandle(t, "NATHEJK:2026.patrulje.team-2.numberassigned",
		messages.NathejkPatrolNumberAssigned{TeamID: "team-2", TeamNumber: "139"}))
	if !strings.Contains(numberStmt, `teamId="team-2"`) {
		t.Errorf("number assignment must key on teamId so a moved member is included: %s", numberStmt)
	}
}
