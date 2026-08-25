package person

import (
	"strings"
	"testing"

	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/cqrs/cqrstest"
	"github.com/nathejk/shared-go/messages"
)

// fold applies a sequence of events to one consumer, returning every statement the
// writer received in order.
//
// A sequence rather than a single event because the crew half of this projection is
// assembled from three event families whose arrival order is not guaranteed, and the
// property under test is convergence — which cannot be observed one event at a time.
func fold(t *testing.T, events ...struct {
	subject string
	body    any
}) []string {
	t.Helper()

	w := &cqrstest.Writer{}
	c := consumer{w: w, normalizer: testNormalizer{}}

	for _, e := range events {
		msg := cqrstest.NewMessage(cqrs.SubjectFromStr(e.subject))
		if err := msg.SetBody(e.body); err != nil {
			t.Fatalf("SetBody(%s): %v", e.subject, err)
		}
		if err := c.HandleMessage(msg); err != nil {
			t.Fatalf("HandleMessage(%s): %v", e.subject, err)
		}
	}
	return w.Statements
}

type event = struct {
	subject string
	body    any
}

var (
	crewRegistered = event{"NATHEJK.2026.crewmember.user-1.updated",
		messages.NathejkCrewMemberUpdated{
			UserID: "user-1",
			Name:   "Mette Sørensen",
			Phone:  "30 11 22 33",
			Email:  "mette@example.dk",
		}}
	sectionAssigned = event{"NATHEJK.2026.crewmember.user-1.section.assigned",
		messages.NathejkCrewMemberSectionAssigned{UserID: "user-1", SectionSlug: "samarit"}}
	sectionAdded = event{"NATHEJK.2026.section.samarit.added",
		messages.NathejkSectionAdded{Slug: "samarit", Label: "Samaritter"}}
)

func joined(stmts []string) string { return strings.Join(stmts, "\n") }

// insertedRole extracts the appRole an INSERT writes.
//
// A helper rather than a substring assertion because the role appears in an upsert's
// VALUES list, not as `appRole=...`, and a test that greps for a bare `"samarit"`
// would also pass on a statement that only carried the *section slug* of the same
// name — which is exactly the bug it is meant to catch.
func insertedRole(t *testing.T, stmt string) string {
	t.Helper()

	open := strings.Index(stmt, "(")
	close := strings.Index(stmt, ")")
	valuesAt := strings.Index(stmt, " VALUES (")
	if open < 0 || close < 0 || valuesAt < 0 {
		t.Fatalf("not an INSERT: %s", stmt)
	}
	names := strings.Split(stmt[open+1:close], ", ")
	rest := stmt[valuesAt+len(" VALUES ("):]
	values := strings.Split(rest[:strings.Index(rest, ")")], ", ")
	if len(names) != len(values) {
		t.Fatalf("column/value mismatch in %s", stmt)
	}
	for i, name := range names {
		if name == "appRole" {
			return strings.Trim(values[i], `"`)
		}
	}
	t.Fatalf("no appRole column in %s", stmt)
	return ""
}

// lastStatementFor returns the last statement mentioning the needle, which is the one
// that determines the converged state.
func lastStatementFor(t *testing.T, stmts []string, needle string) string {
	t.Helper()
	for i := len(stmts) - 1; i >= 0; i-- {
		if strings.Contains(stmts[i], needle) {
			return stmts[i]
		}
	}
	t.Fatalf("no statement mentioning %s in %v", needle, stmts)
	return ""
}

func TestCrewMemberUpdatedWritesThePerson(t *testing.T) {
	stmt := onlyStatement(t, fold(t, crewRegistered))

	for _, want := range []string{
		"INSERT INTO person",
		`"user-1"`,
		`"2026"`,
		`"Mette Sørensen"`,
		// Normalized, or the login lookup misses.
		`"+4530112233"`,
		// Crew have no guardian number. NULL, not "": PRD 003 renders "not
		// applicable" differently from "missing".
		"NULL",
		// The least-privileged fallback until a section says otherwise.
		`"crew"`,
	} {
		if !strings.Contains(stmt, want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmt)
		}
	}
}

// The reason upsertKeepingRole exists. An organizer editing a samarit's email
// re-publishes crewmember.updated, and every replay redelivers it; if that statement
// could write appRole, it would put a classified samarit back on the generic role and
// silently take their SOS page away.
func TestCrewMemberUpdatedNeverDemotesAClassifiedRole(t *testing.T) {
	stmt := onlyStatement(t, fold(t, crewRegistered))

	if !strings.Contains(stmt, "ON DUPLICATE KEY UPDATE") {
		t.Fatalf("want an idempotent upsert, got: %s", stmt)
	}
	update := stmt[strings.Index(stmt, "ON DUPLICATE KEY UPDATE"):]
	if strings.Contains(update, "appRole") {
		t.Errorf("appRole must not be in the update clause, or a replay demotes a\n"+
			"classified crew member to the generic role\ngot: %s", update)
	}
	// The insert half must still carry it, or a crew member whose section is unknown
	// gets no role at all.
	if !strings.Contains(stmt[:strings.Index(stmt, "ON DUPLICATE KEY UPDATE")], "appRole") {
		t.Errorf("appRole must be written on insert\ngot: %s", stmt)
	}
}

// Convergence, order one: the section is named before anyone is put in it.
func TestSectionAddedThenAssignedResolvesTheLabel(t *testing.T) {
	stmts := fold(t, crewRegistered, sectionAdded, sectionAssigned)
	all := joined(stmts)

	for _, want := range []string{
		"INSERT INTO person_section",
		`"Samaritter"`,
		`"samarit"`,
		// The label is resolved by joining, because the projector cannot read.
		"LEFT JOIN person_section",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("statements are missing %s\ngot:\n%s", want, all)
		}
	}
	// Classified from the slug.
	if got := insertedRole(t, lastStatementFor(t, stmts, "appRole")); got != RoleSamarit {
		t.Errorf("appRole = %q, want %q", got, RoleSamarit)
	}
}

// Convergence, order two: someone is assigned to a section that has not been named
// yet. This is the order the back-fill in handleSectionAdded exists for — without it
// the person keeps an empty sectionName forever, and the login chooser (task 079) has
// nothing to tell two crew members on one phone apart with.
func TestAssignedThenSectionAddedBackFillsTheLabel(t *testing.T) {
	stmts := joined(fold(t, crewRegistered, sectionAssigned, sectionAdded))

	if !strings.Contains(stmts, `UPDATE person SET sectionName="Samaritter" WHERE year="2026" AND sectionSlug="samarit"`) {
		t.Errorf("section.added must back-fill people already assigned to the slug\ngot:\n%s", stmts)
	}
}

// Convergence, order three: the assignment arrives before the person does. An UPDATE
// would affect zero rows here and the role would be lost for good, because nothing
// re-publishes an assignment.
func TestAssignedBeforeRegisteredStillRecordsTheRole(t *testing.T) {
	stmts := fold(t, sectionAssigned, sectionAdded, crewRegistered)

	if !strings.Contains(stmts[0], "INSERT INTO person") {
		t.Errorf("an assignment for an unseen person must insert a stub row, not\n"+
			"update nothing\ngot: %s", stmts[0])
	}
	if got := insertedRole(t, stmts[0]); got != RoleSamarit {
		t.Errorf("appRole = %q, want %q", got, RoleSamarit)
	}
	// And the details landing afterwards must not undo it.
	last := stmts[len(stmts)-1]
	if strings.Contains(last[strings.Index(last, "ON DUPLICATE KEY UPDATE"):], "appRole") {
		t.Errorf("the later details event must leave appRole alone\ngot: %s", last)
	}
}

// An unassigned crew member is a real upstream state, not a data problem: they keep
// the generic role and nothing is reported.
func TestUnassignedCrewGetsTheGenericRoleWithoutBeingReported(t *testing.T) {
	w := &cqrstest.Writer{}
	var reported []string
	c := consumer{w: w, normalizer: testNormalizer{},
		unmapped: func(slug string) { reported = append(reported, slug) }}

	msg := cqrstest.NewMessage(cqrs.SubjectFromStr("NATHEJK.2026.crewmember.user-1.section.assigned"))
	if err := msg.SetBody(messages.NathejkCrewMemberSectionAssigned{UserID: "user-1"}); err != nil {
		t.Fatalf("SetBody: %v", err)
	}
	if err := c.HandleMessage(msg); err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}

	if got := insertedRole(t, w.Statements[0]); got != RoleCrew {
		t.Errorf("appRole = %q, want %q", got, RoleCrew)
	}
	if len(reported) != 0 {
		t.Errorf("an empty slug is not an unmapped slug, got %v", reported)
	}
}

// An unrecognised slug is reported but must not fail: organizers rename sections and
// nothing validates the values, so locking someone out of a safety app over a
// spelling is not acceptable.
func TestUnknownSectionSlugIsReportedAndFallsBackToCrew(t *testing.T) {
	w := &cqrstest.Writer{}
	var reported []string
	c := consumer{w: w, normalizer: testNormalizer{},
		unmapped: func(slug string) { reported = append(reported, slug) }}

	msg := cqrstest.NewMessage(cqrs.SubjectFromStr("NATHEJK.2026.crewmember.user-1.section.assigned"))
	if err := msg.SetBody(messages.NathejkCrewMemberSectionAssigned{
		UserID: "user-1", SectionSlug: "traffikvagt"}); err != nil {
		t.Fatalf("SetBody: %v", err)
	}
	if err := c.HandleMessage(msg); err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}

	if len(reported) != 1 || reported[0] != "traffikvagt" {
		t.Errorf("want the slug reported once, got %v", reported)
	}
	if got := insertedRole(t, w.Statements[0]); got != RoleCrew {
		t.Errorf("appRole = %q, want the least-privileged %q", got, RoleCrew)
	}
	// The slug is still recorded, so task 078 can find it in the data.
	if !strings.Contains(w.Statements[0], `"traffikvagt"`) {
		t.Errorf("the unrecognised slug must still be stored\ngot: %s", w.Statements[0])
	}
}

// A nil sink must be usable: nothing in this package may require the application's
// logger (it is bound for shared-go).
func TestUnknownSlugWithNoSinkDoesNotPanic(t *testing.T) {
	stmts := fold(t, event{"NATHEJK.2026.crewmember.user-1.section.assigned",
		messages.NathejkCrewMemberSectionAssigned{UserID: "user-1", SectionSlug: "ukendt"}})
	if len(stmts) == 0 {
		t.Fatal("want statements")
	}
}

func TestCrewMemberDeletedSoftDeletes(t *testing.T) {
	stmt := onlyStatement(t, fold(t, event{"NATHEJK.2026.crewmember.user-1.deleted",
		messages.NathejkCrewMemberUpdated{UserID: "user-1"}}))

	if !strings.Contains(stmt, "deleted=1") {
		t.Errorf("want a soft delete, got: %s", stmt)
	}
	// Never a DELETE: a tombstone survives a replay that re-delivers the registration
	// after the deletion.
	if strings.Contains(stmt, "DELETE FROM") {
		t.Errorf("want a soft delete, not a hard one, got: %s", stmt)
	}
	if !strings.Contains(stmt, `personId="user-1"`) || !strings.Contains(stmt, `year="2026"`) {
		t.Errorf("delete must be scoped to one person-year, got: %s", stmt)
	}
}

// Every statement in the crew path must survive being run twice, because the whole
// stream is replayed on every boot.
func TestCrewStatementsAreIdempotent(t *testing.T) {
	for _, stmt := range fold(t, crewRegistered, sectionAssigned, sectionAdded) {
		switch {
		case strings.HasPrefix(stmt, "UPDATE "):
			// An UPDATE with a WHERE and no relative arithmetic is naturally
			// idempotent.
			if !strings.Contains(stmt, " WHERE ") {
				t.Errorf("unscoped UPDATE: %s", stmt)
			}
		case strings.HasPrefix(stmt, "INSERT "):
			if !strings.Contains(stmt, "ON DUPLICATE KEY UPDATE") &&
				!strings.HasPrefix(stmt, "INSERT IGNORE") {
				t.Errorf("INSERT would fail on the second replay: %s", stmt)
			}
		default:
			t.Errorf("unexpected statement shape: %s", stmt)
		}
	}
}

// The five-part subjects must be matched before the four-part ones, or they are
// swallowed silently and the projection is simply missing sections. This has bitten
// the codebase before.
func TestSectionAssignedIsNotSwallowedByTheCrewMemberPattern(t *testing.T) {
	stmts := joined(fold(t, sectionAssigned))
	if !strings.Contains(stmts, "sectionSlug") {
		t.Errorf("crewmember.*.section.assigned was handled as a plain crewmember\n"+
			"event\ngot:\n%s", stmts)
	}
}

// A section event with no label carries nothing this projection wants — section
// events also carry parent/sort/type fields that are none of its business — and must
// not be an error.
func TestSectionWithoutLabelIsIgnored(t *testing.T) {
	stmts := fold(t, event{"NATHEJK.2026.section.samarit.moved",
		messages.NathejkSectionAdded{Slug: "samarit"}})
	if len(stmts) != 0 {
		t.Errorf("want no statements, got %v", stmts)
	}
}
