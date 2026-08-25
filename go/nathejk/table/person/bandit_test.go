package person

import (
	"strings"
	"testing"

	"github.com/nathejk/shared-go/messages"
)

func TestSeniorUpdatedIsClassifiedAsBandit(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.senior.senior-1.updated",
		messages.NathejkSeniorUpdated{
			MemberID:   "senior-1",
			Name:       "Rasmus Klanmedlem",
			Address:    "Klanvej 2",
			PostalCode: "8000",
			City:       "Aarhus",
			Email:      "rasmus@example.dk",
			Phone:      "40 11 22 33",
		}))

	for _, want := range []string{
		"INSERT INTO person",
		"ON DUPLICATE KEY UPDATE",
		`"senior-1"`,
		`"bandit"`,
		`"Rasmus Klanmedlem"`,
		`"+4540112233"`,
	} {
		if !strings.Contains(stmt, want) {
			t.Errorf("statement missing %s\ngot: %s", want, stmt)
		}
	}
}

// Seniors have no guardian number: NathejkSeniorUpdated has no PhoneContact field and
// shared-go's senior table has no phoneParent column. NULL, not "", so PRD 005 can tell
// "this population has none" from "should have one and it is missing".
func TestSeniorHasNullGuardianPhone(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.senior.senior-1.updated",
		messages.NathejkSeniorUpdated{MemberID: "senior-1", Phone: "40112233"}))

	if !strings.Contains(stmt, "phoneParent, ") && !strings.Contains(stmt, "NULL") {
		t.Errorf("guardian phone must be written as NULL\ngot: %s", stmt)
	}
	// Guard against a "" literal sneaking in for phoneParent specifically.
	if strings.Contains(stmt, `phoneParent="`) {
		t.Errorf("guardian phone must not be an empty string literal\ngot: %s", stmt)
	}
}

func TestSeniorUpdatedIsIdempotent(t *testing.T) {
	body := messages.NathejkSeniorUpdated{MemberID: "senior-1", Name: "Rasmus", Phone: "40112233"}

	first := onlyStatement(t, mustHandle(t, "NATHEJK.2026.senior.senior-1.updated", body))
	second := onlyStatement(t, mustHandle(t, "NATHEJK.2026.senior.senior-1.updated", body))

	if first != second {
		t.Fatalf("replay produced different SQL:\n%s\n%s", first, second)
	}
}

func TestSeniorDeletedSoftDeletes(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.senior.senior-1.deleted",
		messages.NathejkMemberAdded{MemberID: "senior-1"}))

	if !strings.HasPrefix(stmt, "UPDATE person SET deleted=1") {
		t.Errorf("want a soft delete, got: %s", stmt)
	}
}

// The arm number is published on a bandit.* subject with a five-part name, and the
// message body carries only the number — so the member id has to come from the subject.
func TestArmNumberAssignedTakesTheIDFromTheSubject(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.bandit.senior-7.armNumber.assigned",
		messages.NathejkLokArmNumberAssigned{ArmNumber: "B42"}))

	if !strings.Contains(stmt, "UPDATE person SET armNumber=") {
		t.Errorf("want an armNumber update, got: %s", stmt)
	}
	if !strings.Contains(stmt, `"B42"`) {
		t.Errorf("statement missing the arm number: %s", stmt)
	}
	if !strings.Contains(stmt, `"senior-7"`) {
		t.Errorf("member id must come from the subject: %s", stmt)
	}
	// An arm number describes a senior whose details arrive on their own event.
	if strings.Contains(stmt, "INSERT") {
		t.Errorf("must not insert a row for an arm number alone: %s", stmt)
	}
}

// The five-part bandit subject must not be swallowed by the four-part senior patterns,
// which is why it is matched first. If it were, the arm number would silently never be
// recorded.
func TestArmNumberSubjectIsNotMistakenForSenior(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.bandit.senior-7.armNumber.assigned",
		messages.NathejkLokArmNumberAssigned{ArmNumber: "B42"}))

	if strings.Contains(stmt, "appRole") || strings.Contains(stmt, "INSERT") {
		t.Errorf("an armNumber event was handled as a member update: %s", stmt)
	}
}

func TestEmptyArmNumberIsANoOp(t *testing.T) {
	stmts := mustHandle(t, "NATHEJK.2026.bandit.senior-7.armNumber.assigned",
		messages.NathejkLokArmNumberAssigned{})

	if len(stmts) != 0 {
		t.Fatalf("want no statements for an empty arm number, got %v", stmts)
	}
}

// Klan names must be denormalized onto bandits, or the login chooser shows two
// candidates from different klans as identical rows (task 079).
func TestKlanUpdatedDenormalizesTheTeamName(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK:2026.klan.klan-3.updated",
		messages.NathejkKlanUpdated{TeamID: "klan-3", Name: "Klan Ulvene"}))

	if !strings.Contains(stmt, "UPDATE person SET teamName=") {
		t.Errorf("want a teamName update, got: %s", stmt)
	}
	if !strings.Contains(stmt, `"Klan Ulvene"`) || !strings.Contains(stmt, `"klan-3"`) {
		t.Errorf("statement missing the klan name or id: %s", stmt)
	}
}

func TestKlanSignedupAlsoSetsTheTeamName(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK:2026.klan.klan-3.signedup",
		messages.NathejkKlanUpdated{TeamID: "klan-3", Name: "Klan Ulvene"}))

	if !strings.Contains(stmt, `"Klan Ulvene"`) {
		t.Errorf("signedup must set the name too: %s", stmt)
	}
}
