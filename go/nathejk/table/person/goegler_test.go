package person

import (
	"strings"
	"testing"
)

var (
	goeglerSignedUp = event{"NATHEJK.2026.gøgler.g-1.signedup",
		NathejkGoeglerSignedUp{
			TeamID: "g-1",
			Name:   "Caroline Kirkpatrick",
			Phone:  "61 66 09 95",
			Email:  "caroline@example.dk",
		}}
	goeglerUpdated = event{"NATHEJK.2026.gøgler.g-1.updated",
		NathejkGoeglerUpdated{
			UserID: "g-1",
			Name:   "Caroline Kirkpatrick",
			Phone:  "61 66 09 95",
			Email:  "caroline@example.dk",
			Group:  "Strandboerne Vallensbæk",
		}}
)

func TestGoeglerSignedUpWritesThePerson(t *testing.T) {
	stmt := onlyStatement(t, fold(t, goeglerSignedUp))

	for _, want := range []string{
		"INSERT INTO person",
		`"g-1"`,
		`"2026"`,
		`"Caroline Kirkpatrick"`,
		// Normalized, or the login lookup misses.
		`"+4561660995"`,
		`"caroline@example.dk"`,
		// No guardian number for this population.
		"NULL",
	} {
		if !strings.Contains(stmt, want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmt)
		}
	}
	if got := insertedRole(t, stmt); got != RoleGoegler {
		t.Errorf("appRole = %q, want %q", got, RoleGoegler)
	}
}

// The load-bearing property of this handler: a third of the gøglere have a `signedup`
// and no `updated`, so the signup event alone must produce a person who can log in.
func TestGoeglerSignedUpAloneIsEnoughToLogIn(t *testing.T) {
	stmt := onlyStatement(t, fold(t, goeglerSignedUp))

	if !strings.Contains(stmt, `"+4561660995"`) {
		t.Errorf("a gøgler with only a signup event must still have a phone, or they\n"+
			"cannot log in at all\ngot: %s", stmt)
	}
	if got := insertedRole(t, stmt); got != RoleGoegler {
		t.Errorf("appRole = %q, want %q — an unclassified person gets no app", got, RoleGoegler)
	}
}

func TestGoeglerUpdatedWritesTheGroupAsTeamName(t *testing.T) {
	stmt := onlyStatement(t, fold(t, goeglerUpdated))

	if !strings.Contains(stmt, `"Strandboerne Vallensbæk"`) {
		t.Errorf("the scout group is the only thing that tells two gøglere on one\n"+
			"phone apart in the login chooser\ngot: %s", stmt)
	}
	// The part that keeps the stretch of teamName safe: a gøgler must never look like
	// a member of a patrulje, because PRD 002's patrol-scoped reads key on teamId.
	if strings.Contains(stmt, "teamId") {
		t.Errorf("a gøgler must not be given a teamId\ngot: %s", stmt)
	}
}

// Replay order is signedup-then-updated, and both are re-delivered on every boot. The
// thin signup event must not blank the group the richer one supplied.
func TestGoeglerSignedUpDoesNotBlankTheGroup(t *testing.T) {
	stmts := fold(t, goeglerSignedUp, goeglerUpdated, goeglerSignedUp)

	last := stmts[len(stmts)-1]
	if strings.Contains(last, "teamName") {
		t.Errorf("the signup event carries no group and must not touch teamName,\n"+
			"or a redelivery erases it\ngot: %s", last)
	}
}

// The two gøgler events disagree about the name of the id field — `teamId` on signup,
// `userId` on update — and both must resolve to the same person, or a third of the
// population ends up duplicated under two keys.
func TestGoeglerEventsAgreeOnIdentity(t *testing.T) {
	signup := onlyStatement(t, fold(t, goeglerSignedUp))
	update := onlyStatement(t, fold(t, goeglerUpdated))

	for _, stmt := range []string{signup, update} {
		if !strings.Contains(stmt, `"g-1"`) {
			t.Errorf("both gøgler events must key the same person\ngot: %s", stmt)
		}
	}
}

// Neither handler may trust the body alone: if the id field is absent the subject's
// entity id is authoritative.
func TestGoeglerFallsBackToTheSubjectId(t *testing.T) {
	for _, e := range []event{
		{"NATHEJK.2026.gøgler.g-2.signedup", NathejkGoeglerSignedUp{Name: "Uden id"}},
		{"NATHEJK.2026.gøgler.g-2.updated", NathejkGoeglerUpdated{Name: "Uden id"}},
	} {
		stmt := onlyStatement(t, fold(t, e))
		if !strings.Contains(stmt, `"g-2"`) {
			t.Errorf("%s: want the id from the subject\ngot: %s", e.subject, stmt)
		}
	}
}

// Every gøgler statement is re-run on each boot.
func TestGoeglerStatementsAreIdempotent(t *testing.T) {
	for _, stmt := range fold(t, goeglerSignedUp, goeglerUpdated) {
		if !strings.Contains(stmt, "ON DUPLICATE KEY UPDATE") &&
			!strings.HasPrefix(stmt, "INSERT IGNORE") {
			t.Errorf("INSERT would fail on the second replay: %s", stmt)
		}
	}
}

// The gøgler prefix also carries five-part mail/sms notification subjects, which
// outnumber the two this projection wants. Matching one of those as a person would
// write a row from a body that has no person in it.
func TestGoeglerNotificationSubjectsAreIgnored(t *testing.T) {
	for _, subject := range []string{
		"NATHEJK.2026.gøgler.g-1.mail.signup.sent",
		"NATHEJK.2026.gøgler.g-1.mail.validate.sent",
		"NATHEJK.2026.gøgler.g-1.sms.validate.sent",
	} {
		stmts := fold(t, event{subject, map[string]any{"to": "x@example.dk"}})
		if len(stmts) != 0 {
			t.Errorf("%s must be ignored, got %v", subject, stmts)
		}
	}
}

// Guardian phone is "not applicable" for this population, which is not the same as
// missing — PRD 005 skips its confirmation step on the distinction.
func TestGoeglerHasNoGuardianPhone(t *testing.T) {
	if HasGuardianPhone(PopulationGoegler) {
		t.Error("gøglere have no guardian phone")
	}
	stmt := onlyStatement(t, fold(t, goeglerUpdated))
	if !strings.Contains(stmt, "NULL") {
		t.Errorf("phoneParent must be NULL, not an empty string\ngot: %s", stmt)
	}
}
