package kort

import (
	"strings"
	"testing"

	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/cqrs/cqrstest"
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

func str(s string) *string                          { return &s }
func i(n int) *int                                  { return &n }
func setID(s KortsaetID) *KortsaetID                { return &s }
func format(f Format) *Format                       { return &f }
func cgID(s types.CheckgroupID) *types.CheckgroupID { return &s }
func cpIDs(ids ...types.CheckpointID) *[]types.CheckpointID {
	if ids == nil {
		ids = []types.CheckpointID{}
	}
	return &ids
}
func teamType(s types.TeamType) *types.TeamType { return &s }

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

func mustNotContain(t *testing.T, stmt string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if strings.Contains(stmt, want) {
			t.Errorf("statement must not mention %s\ngot: %s", want, stmt)
		}
	}
}

func TestSheetCreatedWritesSetAndName(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.kort.kort-1.created",
		Created{KortID: "kort-1", KortsaetID: "kortsaet-1", Name: "Kort 1"}))

	mustContain(t, stmt, "INSERT INTO kort", "ON DUPLICATE KEY UPDATE",
		`"kort-1"`, `"2026"`, `"kortsaet-1"`, `"Kort 1"`)
}

// A sheet may legitimately be materialised before its set: replay delivers events in stream order,
// and an operator's set may simply come later. Tolerated rather than dropped — dropping would lose
// the sheet permanently, since nothing re-publishes a create.
func TestSheetCreatedBeforeItsSetIsStored(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.kort.kort-1.created",
		Created{KortID: "kort-1", KortsaetID: "kortsaet-nonexistent", Name: "Kort 1"}))

	mustContain(t, stmt, "INSERT INTO kort", `"kortsaet-nonexistent"`)
}

// The id in the subject is authoritative: bodies also carry it, but where they disagree the subject
// wins, because that is what the stream routed on.
func TestSheetIDFallsBackToTheSubject(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.kort.kort-from-subject.created",
		Created{Name: "Kort 1"}))

	mustContain(t, stmt, `"kort-from-subject"`)
}

func TestSheetUpdatedWritesTheFieldsItCarries(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.kort.kort-1.updated",
		Updated{
			KortID:        "kort-1",
			Name:          str("Kort 1 — Start til Post 2"),
			Format:        format(FormatA3),
			SortOrder:     i(3),
			CheckpointIDs: cpIDs("cp-1", "cp-2"),
		}))

	mustContain(t, stmt, "INSERT INTO kort", `"Kort 1 — Start til Post 2"`, `"a3"`,
		"sortOrder", `cp-1`, `cp-2`)
}

// The property this handler exists for, and the one whose failure is silent and expensive: every
// field on Updated is a pointer, so an event that only renames a sheet must not erase its checkpoint
// list — a patrol may see exactly the checkpoints on the sheets it holds, so blanking the list makes
// posts vanish from the map mid-race.
func TestPartialSheetUpdateDoesNotBlankTheOtherFields(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.kort.kort-1.updated",
		Updated{KortID: "kort-1", Name: str("Kort 1")}))

	mustContain(t, stmt, `"Kort 1"`)
	mustNotContain(t, stmt, "checkpointIds", "extents", "format", "note", "handoutCheckgroupId")
}

// An explicitly empty array is a real edit — "this sheet now has no checkpoints" — and must be
// written, not skipped. This is why the field is a pointer to a slice: a plain nil slice could not
// tell this case apart from the one above.
func TestEmptyCheckpointListIsWritten(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.kort.kort-1.updated",
		Updated{KortID: "kort-1", CheckpointIDs: cpIDs()}))

	mustContain(t, stmt, "checkpointIds", `"[]"`)
}

// "" for handoutCheckgroupId is a value — the QR rule — and it is how an organizer switches a sheet
// back from a post. So it must be written. Skipping it as "empty" would leave the sheet revealing off
// a post forever, and the operator's edit would appear to do nothing.
func TestExplicitEmptyHandoutCheckgroupIsWritten(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.kort.kort-1.updated",
		Updated{KortID: "kort-1", HandoutCheckgroupID: cgID("")}))

	mustContain(t, stmt, `handoutCheckgroupId`, `""`)
}

func TestHandoutCheckgroupIDIsWritten(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.kort.kort-1.updated",
		Updated{KortID: "kort-1", HandoutCheckgroupID: cgID("cg-4")}))

	mustContain(t, stmt, "handoutCheckgroupId", `"cg-4"`)
}

// An update after a delete restores the sheet: the last event wins, as in checkpoint and person.
func TestSheetUpdateRestoresADeletedSheet(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.kort.kort-1.updated",
		Updated{KortID: "kort-1", Name: str("Kort 1")}))

	mustContain(t, stmt, "deleted", "0")
}

func TestSheetDeletedSoftDeletes(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.kort.kort-1.deleted", Deleted{KortID: "kort-1"}))

	mustContain(t, stmt, "UPDATE kort SET deleted=1", `"kort-1"`, `"2026"`)
	// No INSERT: a delete for a sheet never seen affects zero rows, which is correct and idempotent.
	mustNotContain(t, stmt, "INSERT")
}

// Position in the list is the sortOrder, and ids not named keep their current order — so this must
// not renumber sheets the operator never touched.
func TestSheetsSortedAppliesPositionAsOrder(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.kort.sorted",
		Sorted{KortIDs: []KortID{"kort-a", "kort-b", "kort-c"}})

	if len(stmts) != 3 {
		t.Fatalf("want one statement per named id, got %d: %v", len(stmts), stmts)
	}
	mustContain(t, stmts[0], "sortOrder=0", `"kort-a"`)
	mustContain(t, stmts[1], "sortOrder=1", `"kort-b"`)
	mustContain(t, stmts[2], "sortOrder=2", `"kort-c"`)
	for _, stmt := range stmts {
		mustNotContain(t, stmt, "kort-unnamed")
	}
}

// A set's created/updated carries the whole record, so a team type present is written as a value.
func TestSetCreatedWritesTeamType(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.kortsaet.kortsaet-1.created",
		SetCreated{KortsaetID: "kortsaet-1", Name: "Patruljer", TeamType: teamType(PatrolTeamType)}))

	mustContain(t, stmt, "INSERT INTO kortsaet", `"kortsaet-1"`, `"Patruljer"`, `"patrulje"`)
}

// The asymmetry with the sheet handlers, and the reason it exists: an absent teamType on a set means
// the set has none, so it must be written as NULL rather than left alone. Under patch semantics an
// operator could never un-mark the patrol set — and since the handout list is filtered on this
// column, a stale value shows one audience's sheets to another.
func TestSetUpdatedWithoutTeamTypeWritesNull(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.kortsaet.kortsaet-1.updated",
		SetUpdated{KortsaetID: "kortsaet-1", Name: "Crew"}))

	mustContain(t, stmt, "teamType", "NULL", `"Crew"`)
	mustNotContain(t, stmt, `"patrulje"`)
}

// The same for a set that *was* marked and is being cleared: the event carries no team type, and the
// projection must overwrite the old value rather than preserve it.
func TestSetUpdatedClearingTeamTypeOverwrites(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.kortsaet.kortsaet-1.updated",
		SetUpdated{KortsaetID: "kortsaet-1", Name: "Patruljer"}))

	mustContain(t, stmt, "teamType=VALUES(teamType)", "NULL")
}

func TestSetDeletedSoftDeletes(t *testing.T) {
	stmt := single(t, fold(t, "NATHEJK.2026.kortsaet.kortsaet-1.deleted",
		SetDeleted{KortsaetID: "kortsaet-1"}))

	mustContain(t, stmt, "UPDATE kortsaet SET deleted=1", `"kortsaet-1"`)
}

func TestSetsSortedAppliesPositionAsOrder(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.kortsaet.sorted",
		SetsSorted{KortsaetIDs: []KortsaetID{"kortsaet-a", "kortsaet-b"}})

	if len(stmts) != 2 {
		t.Fatalf("want 2 statements, got %d", len(stmts))
	}
	mustContain(t, stmts[0], "UPDATE kortsaet", "sortOrder=0", `"kortsaet-a"`)
	mustContain(t, stmts[1], "sortOrder=1", `"kortsaet-b"`)
}

// The collection-level `.sorted` subjects carry no entity id, so they must not be dispatched as
// `{id}.{verb}` with "sorted" read as an id. If they were, a reorder would be folded as an update to
// a sheet called "sorted" — creating a phantom row and losing the reorder.
func TestSortedIsNotMistakenForAnEntityEvent(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.kort.sorted", Sorted{KortIDs: []KortID{"kort-a"}})

	for _, stmt := range stmts {
		mustNotContain(t, stmt, `"sorted"`, "INSERT")
	}
}

// Every statement must be re-runnable: projections replay from sequence zero on every boot, so the
// same event is folded again on each start.
func TestStatementsAreIdempotent(t *testing.T) {
	first := fold(t, "NATHEJK.2026.kort.kort-1.updated",
		Updated{KortID: "kort-1", Name: str("Kort 1"), CheckpointIDs: cpIDs("cp-1")})
	second := fold(t, "NATHEJK.2026.kort.kort-1.updated",
		Updated{KortID: "kort-1", Name: str("Kort 1"), CheckpointIDs: cpIDs("cp-1")})

	if len(first) != len(second) || first[0] != second[0] {
		t.Fatalf("folding the same event twice produced different statements:\n%v\n%v", first, second)
	}
	mustContain(t, first[0], "ON DUPLICATE KEY UPDATE")
}

// A subject with no year cannot be attributed to an event, and every read here is year-scoped, so it
// is an error rather than a silent write into the wrong year.
func TestMissingYearIsAnError(t *testing.T) {
	w := &cqrstest.Writer{}
	c := consumer{w: w}

	msg := cqrstest.NewMessage(cqrs.SubjectFromStr("NATHEJK"))
	if err := msg.SetBody(Created{KortID: "kort-1"}); err != nil {
		t.Fatalf("SetBody: %v", err)
	}
	if err := c.HandleMessage(msg); err == nil {
		t.Fatal("want an error for a subject with no year")
	}
	if len(w.Statements) != 0 {
		t.Errorf("nothing should be written: %v", w.Statements)
	}
}

// A body we cannot decode is reported rather than silently skipped: these shapes are mirrored from
// another repo's unstable types, so an unreadable body is our only signal that the mirror has drifted
// from the contract (PRD 016 §11.11).
func TestUnknownBodyIsReported(t *testing.T) {
	var gotSubject string
	var gotErr error

	w := &cqrstest.Writer{}
	c := consumer{w: w, unknownBody: func(subject string, err error) {
		gotSubject, gotErr = subject, err
	}}

	msg := cqrstest.NewMessage(cqrs.SubjectFromStr("NATHEJK.2026.kort.kort-1.updated"))
	// A body whose shape cannot be read into Updated at all: a JSON array where an object belongs.
	if err := msg.SetBody([]string{"not", "an", "object"}); err != nil {
		t.Fatalf("SetBody: %v", err)
	}
	if err := c.HandleMessage(msg); err == nil {
		t.Fatal("want an error for an undecodable body")
	}
	if gotSubject != "NATHEJK.2026.kort.kort-1.updated" || gotErr == nil {
		t.Errorf("unknownBody not reported: subject=%q err=%v", gotSubject, gotErr)
	}
}

func TestExtentsAreStoredAsJSON(t *testing.T) {
	extents := []Extent{{
		NorthWest: types.Position{Latitude: 56, Longitude: 9},
		SouthEast: types.Position{Latitude: 55, Longitude: 9.4},
	}}
	stmt := single(t, fold(t, "NATHEJK.2026.kort.kort-1.updated",
		Updated{KortID: "kort-1", Extents: &extents}))

	mustContain(t, stmt, "extents", "northWest", "southEast", "56", "9.4")
}
