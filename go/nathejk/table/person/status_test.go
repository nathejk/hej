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

// The start event has always carried the numbers check-in recorded, and this projection discarded
// them until task 230. After the start they are what staff actually hold.
func TestTeamStartedRecordsTheNumbersCheckInHolds(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK:2026.patrulje.team-9.started",
		messages.NathejkTeamStarted{
			TeamID: "team-9",
			Members: []messages.NathejkTeamStarted_Member{
				{MemberID: "member-1", Phone: "30 11 22 33", PhoneGuardian: "40 55 66 77"},
			},
		}))

	// Normalized, like every other number in this projection: the profile read compares these
	// against a canonical own number, so a raw value would read as a different number.
	if !strings.Contains(stmt, `startedPhone="+4530112233"`) {
		t.Errorf("own number missing or unnormalized: %s", stmt)
	}
	if !strings.Contains(stmt, `startedPhoneContact="+4540556677"`) {
		t.Errorf("contact number missing or unnormalized: %s", stmt)
	}
	if !strings.Contains(stmt, `memberStatus="racing"`) {
		t.Errorf("the status must still be written: %s", stmt)
	}
}

// An omitted number says nothing about it. Writing "" would turn that silence into the claim
// "check-in recorded no contact number" — and that claim is what the app would then show the member
// in place of a number we do hold.
func TestTeamStartedDoesNotBlankNumbersItWasNotGiven(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK:2026.patrulje.team-9.started",
		messages.NathejkTeamStarted{
			TeamID:  "team-9",
			Members: []messages.NathejkTeamStarted_Member{{MemberID: "member-1"}},
		}))

	if strings.Contains(stmt, "startedPhone") || strings.Contains(stmt, "startedPhoneContact") {
		t.Errorf("an event with no numbers must not write the columns at all: %s", stmt)
	}
}

// A member whose contact number is unusable is still started — the status is what the race runs on,
// and a bad phone number must not dead-letter it (the same rule handleSpejderUpdated follows).
func TestTeamStartedSurvivesAnUnusableNumber(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK:2026.patrulje.team-9.started",
		messages.NathejkTeamStarted{
			TeamID: "team-9",
			Members: []messages.NathejkTeamStarted_Member{
				{MemberID: "member-1", PhoneGuardian: "123"},
			},
		}))

	if !strings.Contains(stmt, `memberStatus="racing"`) {
		t.Errorf("the member must still be marked racing: %s", stmt)
	}
	if strings.Contains(stmt, "startedPhoneContact") {
		t.Errorf("an unusable number must not be stored: %s", stmt)
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
