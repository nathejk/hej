package kort

import (
	"encoding/json"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/types"
)

// Queries is the read API handed to the application.
//
// What it deliberately does not offer: a way to ask for every sheet's checkpoints. A patrol may see
// the checkpoints on the sheets *it* holds, and nothing more, so the reveal rule (task 255) is the
// only consumer of `checkpointIds` — this interface serves it and the handout list, not a general
// "give me the maps" call. Same reasoning as checkpoint.Queries exposing only the hull: keeping the
// dangerous read out of the interface means no call site can get the decision wrong.
type Queries interface {
	// PatrolSheets returns the year's sheets that belong to a patrol map set, in handout order.
	//
	// Scoped by team type, never by set name. Empty is a normal answer — early in the year, or a
	// year whose sets are not marked yet — and callers must treat it as "nothing to show" rather
	// than an error.
	PatrolSheets(year string) ([]Sheet, error)

	// Sets returns the year's map sets, in their own order.
	Sets(year string) ([]Set, error)
}

// Sheet is one printed map sheet, as this app needs it.
//
// Narrower than HQ's equivalent: no note, no version, no audit fields. A note is written for an
// organizer deciding what to print, and carrying it here would mean a field we must remember not to
// render.
type Sheet struct {
	ID         KortID
	KortsaetID KortsaetID
	Name       string
	Format     Format

	// SortOrder is handout order along the route, within a set.
	SortOrder int

	// HandoutCheckgroupID is the sheet's reveal trigger: a checkgroup id means the sheet is handed
	// out at that post, and "" means the QR rule. Never "unknown" — see the field's documentation
	// on Updated.
	//
	// An id that does not resolve against our own checkgroup projection must be read as "" by the
	// caller (task 256). It is not normalised here, because this package has no view of
	// checkgroups and inventing one would couple two projections that are otherwise independent.
	HandoutCheckgroupID types.CheckgroupID

	// CheckpointIDs are the checkpoints drawn on this sheet, as stored.
	//
	// **Unresolved.** A sheet carries the ids that were saved, and nothing re-publishes them when a
	// checkpoint later disappears — in particular, deleting a checkgroup emits no per-checkpoint
	// event. So the caller must resolve these against its own checkpoint projection and drop what
	// does not resolve (task 256). Order carries no meaning; a checkpoint may appear on any number
	// of sheets, including several in one set, because adjacent sheets overlap by design.
	CheckpointIDs []types.CheckpointID

	// Extents is zero, one or two rectangles of ground. Two means a double-sided sheet — one sheet,
	// one handover, one reveal — and nothing says which side is which.
	Extents []Extent
}

// Set is one map set.
type Set struct {
	ID   KortsaetID
	Name string

	// TeamType is the team type this set is *specifically for*, or "" for none.
	//
	// "" is the ordinary case rather than missing data: the crew set covers gøglere, banditter and
	// crew, who are not one team type. Read it as a filter, not a key — several sets may carry the
	// same value.
	TeamType types.TeamType

	SortOrder int
}

type querier struct {
	db cqrs.Reader

	// counts reports the aggregate shape of the year's map definitions. See ReportCounts.
	counts func(year string, sets, patrolSets, sheets, patrolSheets int)
}

const selectSheet = `SELECT k.id, k.kortsaetId, k.name, k.format, k.sortOrder,
	k.handoutCheckgroupId, k.checkpointIds, k.extents
	FROM kort k`

// PatrolSheets returns the sheets in the year's patrol map set(s).
//
// # Why the filter is a join on teamType and not a name
//
// Set names are Danish free text an organizer may rename mid-season — "Patruljer", "Patruljekort",
// "Patruljer nord" — so matching on them will break, quietly, in the middle of a race. The team type
// is the stable marker, and the value is `patrulje`: `spejder` is the domain's word for a person and
// HQ refuses it on write, so a filter written against it would match nothing and reveal nothing
// while looking entirely correct (PRD 016 §11.1).
//
// # Why *all* matching sets, not one
//
// teamType is a filter yielding candidate sheets, not a key. More than one set may carry it — a year
// that splits its patrol maps into two sets is legitimate — so there is no "the" patrol set to
// select. `IN (SELECT ...)` rather than a lookup of a single id for exactly that reason.
//
// One query for the whole year, no pagination: there are on the order of fifteen sheets, and both
// the handout list and the reveal rule want all of them.
//
// `id` is the tiebreak after sortOrder so two sheets sharing a sort order — easy to produce
// mid-reorder — come back in a stable order rather than whatever the storage engine feels like. Two
// consecutive loads disagreeing about the order would look like a bug in the app.
// patrolSheetsQuery selects the sheets in the year's patrol map set(s).
//
// A package-level const rather than an inline string so a test can assert on it directly (the scan
// package does the same with byTeamQuery): the guarantee that matters here — filter on the team type,
// never on the set name — is a property of this exact text, and pinning it is how a name comparison
// slipped in during a later edit gets caught.
const patrolSheetsQuery = selectSheet + `
		JOIN kortsaet s ON s.id = k.kortsaetId AND s.year = k.year AND s.deleted = 0
		WHERE k.year = ? AND k.deleted = 0 AND s.teamType = ?
		ORDER BY s.sortOrder ASC, k.sortOrder ASC, k.id ASC`

func (q querier) PatrolSheets(year string) ([]Sheet, error) {
	rows, err := q.db.Query(patrolSheetsQuery, year, string(PatrolTeamType))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sheets := []Sheet{}
	for rows.Next() {
		var (
			s                   Sheet
			checkpointIDs, exts string
		)
		if err := rows.Scan(&s.ID, &s.KortsaetID, &s.Name, &s.Format, &s.SortOrder,
			&s.HandoutCheckgroupID, &checkpointIDs, &exts); err != nil {
			return nil, err
		}

		// A malformed JSON column yields an empty list rather than an error. The column is written
		// by our own consumer from our own types, so this should be unreachable — but the
		// alternative on a bad row is failing the whole handout list, and a sheet with no
		// checkpoints is a far better outcome than a patrol with no map page at all.
		if err := json.Unmarshal([]byte(checkpointIDs), &s.CheckpointIDs); err != nil {
			s.CheckpointIDs = nil
		}
		if err := json.Unmarshal([]byte(exts), &s.Extents); err != nil {
			s.Extents = nil
		}

		sheets = append(sheets, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if q.counts != nil {
		q.reportCounts(year, len(sheets))
	}
	return sheets, nil
}

// Sets returns the year's map sets in their own order.
func (q querier) Sets(year string) ([]Set, error) {
	rows, err := q.db.Query(`
		SELECT id, name, COALESCE(teamType, ''), sortOrder
		FROM kortsaet
		WHERE year = ? AND deleted = 0
		ORDER BY sortOrder ASC, id ASC`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sets := []Set{}
	for rows.Next() {
		var s Set
		if err := rows.Scan(&s.ID, &s.Name, &s.TeamType, &s.SortOrder); err != nil {
			return nil, err
		}
		sets = append(sets, s)
	}
	return sets, rows.Err()
}

// reportCounts tells the application the aggregate shape of the year's definitions.
//
// Reported from the read rather than after replay, because "how many sheets does this year have" is
// only meaningful once the projection has caught up, and a read is the first moment we know it has.
// Failures here are swallowed: a diagnostic must never be able to fail the request it describes.
func (q querier) reportCounts(year string, patrolSheets int) {
	var sets, patrolSets, sheets int

	row := q.db.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM kortsaet WHERE year = ? AND deleted = 0),
			(SELECT COUNT(*) FROM kortsaet WHERE year = ? AND deleted = 0 AND teamType = ?),
			(SELECT COUNT(*) FROM kort WHERE year = ? AND deleted = 0)`,
		year, year, string(PatrolTeamType), year)
	if err := row.Scan(&sets, &patrolSets, &sheets); err != nil {
		return
	}
	q.counts(year, sets, patrolSets, sheets, patrolSheets)
}

// Ensure the querier satisfies the read API.
var _ Queries = querier{}
