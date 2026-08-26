package person

import (
	"strings"
	"testing"

	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/cqrs/cqrstest"
	"github.com/nathejk/shared-go/messages"
)

// handle builds a message on the given subject with the given body and folds it in,
// returning the statements the writer received.
func handle(t *testing.T, subject string, body any) ([]string, error) {
	t.Helper()

	w := &cqrstest.Writer{}
	c := consumer{w: w, normalizer: testNormalizer{}}

	msg := cqrstest.NewMessage(cqrs.SubjectFromStr(subject))
	if err := msg.SetBody(body); err != nil {
		t.Fatalf("SetBody: %v", err)
	}

	err := c.HandleMessage(msg)
	return w.Statements, err
}

func mustHandle(t *testing.T, subject string, body any) []string {
	t.Helper()
	stmts, err := handle(t, subject, body)
	if err != nil {
		t.Fatalf("HandleMessage(%s): %v", subject, err)
	}
	return stmts
}

func onlyStatement(t *testing.T, stmts []string) string {
	t.Helper()
	if len(stmts) != 1 {
		t.Fatalf("want exactly 1 statement, got %d: %v", len(stmts), stmts)
	}
	return stmts[0]
}

// upsertStatement picks out the INSERT from a handler that emits several statements.
//
// The spejder handler emits two: the upsert, and a conditional verification
// invalidation (task 076). Tests about the row's contents want the first, and should not
// break every time a handler grows a second concern.
func upsertStatement(t *testing.T, stmts []string) string {
	t.Helper()
	var found []string
	for _, s := range stmts {
		if strings.HasPrefix(s, "INSERT ") {
			found = append(found, s)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want exactly 1 INSERT, got %d: %v", len(found), stmts)
	}
	return found[0]
}

func TestSpejderUpdatedWritesThePerson(t *testing.T) {
	stmt := upsertStatement(t, mustHandle(t, "NATHEJK.2026.spejder.member-1.updated",
		messages.NathejkScoutUpdated{
			MemberID:     "member-1",
			Name:         "Freja Hansen",
			Address:      "Skovvej 1",
			PostalCode:   "2800",
			City:         "Lyngby",
			Email:        "freja@example.dk",
			Phone:        "30 11 22 33",
			PhoneContact: "40 55 66 77",
		}))

	for _, want := range []string{
		"INSERT INTO person",
		"ON DUPLICATE KEY UPDATE",
		`"member-1"`,
		`"2026"`,
		`"spejder"`,
		`"Freja Hansen"`,
		// Both numbers must be stored normalized, or the login lookup misses.
		`"+4530112233"`,
		`"+4540556677"`,
	} {
		if !strings.Contains(stmt, want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmt)
		}
	}
}

// Idempotency is the contract: the same event is replayed on every boot, so the
// statement must be safe to run repeatedly.
func TestSpejderUpdatedIsIdempotent(t *testing.T) {
	body := messages.NathejkScoutUpdated{MemberID: "member-1", Name: "Freja", Phone: "30112233"}

	first := upsertStatement(t, mustHandle(t, "NATHEJK.2026.spejder.member-1.updated", body))
	second := upsertStatement(t, mustHandle(t, "NATHEJK.2026.spejder.member-1.updated", body))

	if first != second {
		t.Fatalf("identical events produced different SQL:\n%s\n%s", first, second)
	}
	if !strings.Contains(first, "ON DUPLICATE KEY UPDATE") {
		t.Error("a replayed INSERT must be an upsert, or the second boot fails")
	}
}

// A guardian number is what makes spejder different, and its absence must be NULL —
// not "" — so PRD 005 can tell "none on file" from "this population has none".
func TestMissingGuardianPhoneIsNull(t *testing.T) {
	stmt := upsertStatement(t, mustHandle(t, "NATHEJK.2026.spejder.member-1.updated",
		messages.NathejkScoutUpdated{MemberID: "member-1", Phone: "30112233"}))

	if !strings.Contains(stmt, "NULL") {
		t.Errorf("an absent guardian number must be written as NULL\ngot: %s", stmt)
	}
	if strings.Contains(stmt, `phoneParent, "`) {
		t.Errorf("guardian number must not be an empty string literal\ngot: %s", stmt)
	}
}

// An unparseable number must not become a lookup key. It normalizes to "", which the
// querier refuses to look up, so the person simply cannot log in — visible and
// fixable, rather than matching every row with no number on file.
func TestUnparseablePhoneStoresEmpty(t *testing.T) {
	stmt := upsertStatement(t, mustHandle(t, "NATHEJK.2026.spejder.member-1.updated",
		messages.NathejkScoutUpdated{MemberID: "member-1", Phone: "not a number"}))

	if !strings.Contains(stmt, `phone, `) && !strings.Contains(stmt, `""`) {
		t.Errorf("expected an empty phone literal\ngot: %s", stmt)
	}
}

// An update must clear a previous soft delete: a member deleted and re-added upstream
// should get their login back.
func TestSpejderUpdatedClearsSoftDelete(t *testing.T) {
	stmt := upsertStatement(t, mustHandle(t, "NATHEJK.2026.spejder.member-1.updated",
		messages.NathejkScoutUpdated{MemberID: "member-1", Phone: "30112233"}))

	if !strings.Contains(stmt, "deleted") {
		t.Errorf("an update must reset the deleted flag\ngot: %s", stmt)
	}
}

func TestSpejderDeletedSoftDeletes(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.spejder.member-1.deleted",
		messages.NathejkMemberAdded{MemberID: "member-1"}))

	if !strings.HasPrefix(stmt, "UPDATE person SET deleted=1") {
		t.Errorf("want a soft delete, got: %s", stmt)
	}
	// A delete must not create a row for someone never seen.
	if strings.Contains(stmt, "INSERT") {
		t.Errorf("a delete must not insert a tombstone row\ngot: %s", stmt)
	}
}

// The team name is denormalized onto members so the login path reads one row without
// a join.
func TestPatruljeUpdatedDenormalizesTheTeamName(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK:2026.patrulje.team-9.updated",
		messages.NathejkTeamUpdated{TeamID: "team-9", Name: "Patrulje Ravnene"}))

	if !strings.Contains(stmt, "UPDATE person SET teamName=") {
		t.Errorf("want a teamName update, got: %s", stmt)
	}
	if !strings.Contains(stmt, `"Patrulje Ravnene"`) || !strings.Contains(stmt, `"team-9"`) {
		t.Errorf("statement is missing the team name or id\ngot: %s", stmt)
	}
	if strings.Contains(stmt, "INSERT") {
		t.Errorf("a team event must not invent people\ngot: %s", stmt)
	}
}

// A team event with no name is not an error: team messages carry many fields and most
// are none of this projection's business.
func TestPatruljeUpdatedWithoutNameIsANoOp(t *testing.T) {
	stmts := mustHandle(t, "NATHEJK:2026.patrulje.team-9.updated",
		messages.NathejkTeamUpdated{TeamID: "team-9"})

	if len(stmts) != 0 {
		t.Fatalf("want no statements, got %v", stmts)
	}
}

// An unrecognised subject must be ignored, not error: erroring would dead-letter
// every message this projection does not care about.
func TestUnknownSubjectIsIgnored(t *testing.T) {
	stmts, err := handle(t, "NATHEJK.2026.payment.p-1.received", map[string]string{"a": "b"})
	if err != nil {
		t.Fatalf("an unrelated subject must not error: %v", err)
	}
	if len(stmts) != 0 {
		t.Fatalf("want no statements, got %v", stmts)
	}
}

// Without a year there is no primary key. Failing loudly beats guessing a year and
// silently writing rows nobody will ever find.
func TestSubjectWithoutYearIsAnError(t *testing.T) {
	if _, err := handle(t, "NATHEJK", messages.NathejkScoutUpdated{MemberID: "m"}); err == nil {
		t.Fatal("want an error when the subject carries no year")
	}
}

func TestSpejderUpdatedWithoutMemberIDIsAnError(t *testing.T) {
	if _, err := handle(t, "NATHEJK.2026.spejder.x.updated", messages.NathejkScoutUpdated{}); err == nil {
		t.Fatal("want an error when the event carries no memberId")
	}
}

// Escaping is this file's job, since cqrs.Writer takes a finished statement. A quote
// in a name must not be able to terminate the literal.
func TestNameWithQuotesIsEscaped(t *testing.T) {
	stmt := upsertStatement(t, mustHandle(t, "NATHEJK.2026.spejder.member-1.updated",
		messages.NathejkScoutUpdated{MemberID: "member-1", Name: `Freja "Fre" O'Hansen`, Phone: "30112233"}))

	// %q escapes the inner double quotes; the raw sequence must not appear.
	if strings.Contains(stmt, `"Freja "Fre"`) {
		t.Errorf("quotes were not escaped\ngot: %s", stmt)
	}
	if !strings.Contains(stmt, `\"Fre\"`) {
		t.Errorf("expected escaped quotes in the literal\ngot: %s", stmt)
	}
}
