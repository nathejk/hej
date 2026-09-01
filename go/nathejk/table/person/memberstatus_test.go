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
			"NATHEJK:2026.spejder.member-1.team.moved",
			messages.NathejkMemberTeamMoved{MemberID: "member-1", FromTeamID: "team-1", ToTeamID: "team-2"},
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

// `team.moved` writes the status and deliberately not the team. Documented as a known gap
// (task 178) rather than left to be discovered: a moved member's teamId goes stale, so they
// would appear under their old patrol in PRD 007's lookup.
func TestTeamMovedDoesNotChangeTheTeam(t *testing.T) {
	stmts := mustHandle(t, "NATHEJK:2026.spejder.member-1.team.moved",
		messages.NathejkMemberTeamMoved{MemberID: "member-1", FromTeamID: "team-1", ToTeamID: "team-2"})

	if len(stmts) != 1 {
		t.Fatalf("want one statement, got %v", stmts)
	}
	if strings.Contains(stmts[0], "teamId=") {
		t.Errorf("team.moved must not write teamId yet — see task 178: %s", stmts[0])
	}
}
