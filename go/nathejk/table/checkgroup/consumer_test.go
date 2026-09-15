package checkgroup

import (
	"regexp"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
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

func str(s string) *string                                    { return &s }
func scheme(s types.CheckgroupScheme) *types.CheckgroupScheme { return &s }
func cgID(s types.CheckgroupID) *types.CheckgroupID           { return &s }

func single(t *testing.T, stmts []string) string {
	t.Helper()
	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d: %v", len(stmts), stmts)
	}
	return stmts[0]
}

func mustContain(t *testing.T, stmt string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(stmt, want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmt)
		}
	}
}

func TestUpdatedWritesNameSchemeAndAnchor(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.checkgroup.cg-1.updated",
		messages.NathejkCheckgroupUpdated{
			CheckgroupID:         "cg-1",
			Name:                 str("Postlinje 1"),
			Scheme:               scheme(types.CheckgroupSchemeRelative),
			RelativeCheckgroupID: cgID("cg-0"),
		}))

	mustContain(t, stmt, "INSERT INTO checkgroup", "ON DUPLICATE KEY UPDATE",
		`"cg-1"`, `"2026"`, `"Postlinje 1"`, `"relative"`, `"cg-0"`)
}

// The patch property, and here it has a sharper consequence than elsewhere: a blanked `scheme` turns
// every on-time verdict at this group's posts into "no verdict", and nothing about the app looks
// broken — the badges simply stop appearing.
func TestPartialUpdateDoesNotBlankTheScheme(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.checkgroup.cg-1.updated",
		messages.NathejkCheckgroupUpdated{CheckgroupID: "cg-1", Name: str("Renamed")}))

	mustContain(t, stmt, `"Renamed"`)
	for _, forbidden := range []string{"scheme", "relativeCheckgroupId", "sortOrder"} {
		if strings.Contains(stmt, forbidden) {
			t.Errorf("a name-only update must not touch %s\ngot: %s", forbidden, stmt)
		}
	}
}

// Clearing the anchor is a real edit: a stale one would make the verdict measure from a group the
// organizer no longer intends, which is a wrong verdict rather than a missing one.
func TestEmptyRelativeCheckgroupIsWritten(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.checkgroup.cg-1.updated",
		messages.NathejkCheckgroupUpdated{CheckgroupID: "cg-1", RelativeCheckgroupID: cgID("")}))

	mustContain(t, stmt, "relativeCheckgroupId")
}

// An unrecognised scheme is stored as sent. It yields no verdict, which is the same outcome as `none`
// and the right one: inventing a verdict from a scheme we do not understand is the failure to avoid.
func TestUnknownSchemeIsStoredAsSent(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.checkgroup.cg-1.updated",
		messages.NathejkCheckgroupUpdated{
			CheckgroupID: "cg-1",
			Scheme:       scheme(types.CheckgroupScheme("something-new")),
		}))

	mustContain(t, stmt, `"something-new"`)
}

// The decision recorded in PRD 016 §11.3, pinned so it cannot be quietly reversed: `showOnMap` is on
// the event and must not reach our read model. Nothing upstream reads it, its intent is unverified, and
// our reveal rule is grounded in physical possession instead — which cannot over-reveal whatever the
// flag means.
func TestShowOnMapIsNotStored(t *testing.T) {
	yes := true
	stmt := single(t, fold(t, "NATHEJK.2026.checkgroup.cg-1.updated",
		messages.NathejkCheckgroupUpdated{
			CheckgroupID: "cg-1",
			Name:         str("Postlinje 1"),
			ShowOnMap:    &yes,
			Mandatory:    &yes,
		}))

	for _, forbidden := range []string{"showOnMap", "mandatory"} {
		if strings.Contains(stmt, forbidden) {
			t.Errorf("%s must not be projected (PRD 016 §11.3)\ngot: %s", forbidden, stmt)
		}
	}
}

func TestUpdateRestoresADeletedGroup(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.checkgroup.cg-1.updated",
		messages.NathejkCheckgroupUpdated{CheckgroupID: "cg-1", Name: str("Postlinje 1")}))

	mustContain(t, stmt, "deleted", "0")
}

// No cascade to the group's checkpoints: upstream emits no per-checkpoint event, and the reveal rule
// resolves the dangling reference on read instead (task 256). Cascading here would couple two
// independent projections' replay order, and getting it wrong would delete checkpoints the race area is
// derived from.
func TestDeletedDoesNotCascadeToCheckpoints(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.checkgroup.cg-1.deleted",
		messages.NathejkCheckgroupDeleted{CheckgroupID: "cg-1"}))

	mustContain(t, stmt, "UPDATE checkgroup SET deleted=1", `"cg-1"`)
	if strings.Contains(stmt, "checkpoint") {
		t.Errorf("a checkgroup delete must not touch the checkpoint table\ngot: %s", stmt)
	}
}

// The sorted subject is **plural** — `checkgroups.sorted` — while the checkpoint reorder is
// `checkgroup.{id}.checkpoints_sorted`. Neither can be guessed from the other, so the subscription is
// pinned here.
func TestSortedSubjectIsPlural(t *testing.T) {
	subjects := make([]string, 0, 3)
	for _, s := range (consumer{}).Consumes() {
		subjects = append(subjects, s.Subject())
	}
	joined := strings.Join(subjects, " ")

	if !strings.Contains(joined, "checkgroups.sorted") {
		t.Errorf("want the plural checkgroups.sorted subject: %s", joined)
	}
	for _, want := range []string{"checkgroup.*.updated", "checkgroup.*.deleted"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing subscription to %s: %s", want, joined)
		}
	}
}

func TestSortedAppliesPositionAsOrder(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.checkgroups.sorted",
		messages.NathejkCheckgroupsSorted{
			SortedCheckgroupIDs: []types.CheckgroupID{"cg-a", "cg-b", "cg-c"},
		})

	if len(stmts) != 3 {
		t.Fatalf("want one statement per named id, got %d: %v", len(stmts), stmts)
	}
	mustContain(t, stmts[0], "sortOrder=0", `"cg-a"`)
	mustContain(t, stmts[1], "sortOrder=1", `"cg-b"`)
	mustContain(t, stmts[2], "sortOrder=2", `"cg-c"`)
}

func TestStatementsAreIdempotent(t *testing.T) {
	body := messages.NathejkCheckgroupUpdated{
		CheckgroupID: "cg-1",
		Name:         str("Postlinje 1"),
		Scheme:       scheme(types.CheckgroupSchemeFixed),
	}
	first := single(t, fold(t, "NATHEJK.2026.checkgroup.cg-1.updated", body))
	second := single(t, fold(t, "NATHEJK.2026.checkgroup.cg-1.updated", body))

	if first != second {
		t.Fatalf("folding the same event twice differed:\n%s\n%s", first, second)
	}
	mustContain(t, first, "ON DUPLICATE KEY UPDATE")
}

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
		t.Errorf("nothing should be written: %v", w.Statements)
	}
}

func TestByYearReturnsRouteOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("FROM checkgroup")).
		WithArgs("2026").
		WillReturnRows(sqlmock.NewRows(
			[]string{"checkgroupId", "name", "sortOrder", "scheme", "relativeCheckgroupId"}).
			AddRow("cg-1", "Postlinje 1", 0, "fixed", "").
			AddRow("cg-2", "Postlinje 2", 1, "relative", "cg-1"))

	got, err := querier{db: db}.ByYear("2026")
	if err != nil {
		t.Fatalf("ByYear: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 groups, got %d", len(got))
	}
	if got[1].Scheme != types.CheckgroupSchemeRelative || got[1].RelativeCheckgroupID != "cg-1" {
		t.Errorf("relative group: %+v", got[1])
	}
}

func TestByYearEmptyIsNotAnError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("FROM checkgroup")).
		WithArgs("2026").
		WillReturnRows(sqlmock.NewRows(
			[]string{"checkgroupId", "name", "sortOrder", "scheme", "relativeCheckgroupId"}))

	got, err := querier{db: db}.ByYear("2026")
	if err != nil {
		t.Fatalf("ByYear: %v", err)
	}
	if got == nil {
		t.Fatal("want an empty slice, not nil")
	}
}
