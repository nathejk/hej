package person

import (
	"strings"
	"testing"

	"github.com/nathejk/shared-go/messages"
)

func TestTeamStartedMarksNamedMembersRacing(t *testing.T) {
	stmts := mustHandle(t, "NATHEJK:2026.patrulje.team-9.started",
		messages.NathejkTeamStarted{
			TeamID: "team-9",
			Members: []messages.NathejkTeamStarted_Member{
				{MemberID: "member-1"},
				{MemberID: "member-2"},
			},
		})

	if len(stmts) != 2 {
		t.Fatalf("want one statement per named member, got %d: %v", len(stmts), stmts)
	}
	for _, stmt := range stmts {
		if !strings.Contains(stmt, `memberStatus="racing"`) {
			t.Errorf("want memberStatus set to racing, got: %s", stmt)
		}
		// A start event must not create a person whose details have not arrived.
		if strings.Contains(stmt, "INSERT") {
			t.Errorf("a start event must not insert a row: %s", stmt)
		}
		if !strings.Contains(stmt, `year="2026"`) {
			t.Errorf("statement must be scoped to the year: %s", stmt)
		}
	}
}

// The event names exactly who started. Marking the whole team instead would wrongly
// mark no-shows as racing — hq's projector notes that StartPatrulje publishes a
// separate `deleted` for every member who did not start.
func TestTeamStartedIgnoresMembersWithoutID(t *testing.T) {
	stmts := mustHandle(t, "NATHEJK:2026.patrulje.team-9.started",
		messages.NathejkTeamStarted{
			TeamID: "team-9",
			Members: []messages.NathejkTeamStarted_Member{
				{MemberID: "member-1"},
				{MemberID: ""},
			},
		})

	if len(stmts) != 1 {
		t.Fatalf("want 1 statement for 1 identifiable member, got %d", len(stmts))
	}
	if !strings.Contains(stmts[0], `"member-1"`) {
		t.Errorf("wrong member targeted: %s", stmts[0])
	}
}

func TestTeamStartedIsIdempotent(t *testing.T) {
	body := messages.NathejkTeamStarted{
		TeamID:  "team-9",
		Members: []messages.NathejkTeamStarted_Member{{MemberID: "member-1"}},
	}

	first := onlyStatement(t, mustHandle(t, "NATHEJK:2026.patrulje.team-9.started", body))
	second := onlyStatement(t, mustHandle(t, "NATHEJK:2026.patrulje.team-9.started", body))

	if first != second {
		t.Fatalf("replay produced different SQL:\n%s\n%s", first, second)
	}
}

// The `.started` subject has the same shape as `.updated`, so a careless switch would
// route it to the team-name handler and never set a status at all.
func TestStartedIsNotMistakenForUpdated(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK:2026.patrulje.team-9.started",
		messages.NathejkTeamStarted{
			TeamID:  "team-9",
			Members: []messages.NathejkTeamStarted_Member{{MemberID: "member-1"}},
		}))

	if strings.Contains(stmt, "teamName") {
		t.Errorf("a started event was handled as a team-name update: %s", stmt)
	}
}

// HasStarted is the skip rule; NeedsPortrait is the nudge. They must not be the same
// signal: a member who started without a photo still needs nudging (PRD 005).
func TestHasStartedAndNeedsPortraitAreIndependent(t *testing.T) {
	started := Person{MemberStatus: MemberStatusRacing}
	if !started.HasStarted() {
		t.Error("a racing member must report as started")
	}
	if !started.NeedsPortrait() {
		t.Error("a started member with no portrait must still be nudged — this is the case PRD 005 called out")
	}

	withPhoto := Person{MemberStatus: MemberStatusRacing, PortraitRef: "abc"}
	if withPhoto.NeedsPortrait() {
		t.Error("a member with a portrait must not be nudged")
	}

	fresh := Person{}
	if fresh.HasStarted() {
		t.Error("a member with no status has not started")
	}
	if !fresh.NeedsPortrait() {
		t.Error("a member with no portrait needs one")
	}
}
