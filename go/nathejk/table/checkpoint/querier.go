package checkpoint

import (
	"database/sql"
	"strings"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/types"
)

// Queries is the read API handed to the application.
//
// # The shape is the security boundary
//
// Originally this exposed the **area** and nothing else: the event area is deliberately not fully known
// to participants (PRD 002), so an interface that returned positions would put the decision "should this
// reach the client?" at every call site, and one of them would eventually get it wrong.
//
// PRD 016 requires some checkpoints to reach a patrol — the ones drawn on the sheets it holds — so the
// interface had to widen. It widened in the one way that keeps the property: **both new reads are
// bounded by what the caller already names.** `ByIDs` can only return checkpoints whose ids the caller
// supplied; `ByCheckgroups` only those in groups it supplied. There is still no way to ask this package
// for *all* checkpoints, so a handler cannot leak a position it did not already have grounds to know
// about.
//
// The reveal rule itself lives in internal/reveal, which decides *which* ids a patrol has earned. This
// package's job is to refuse to answer any broader question.
type Queries interface {
	// RaceArea returns the buffered hull of the year's positioned checkpoints.
	//
	// ok is false when there is nothing to derive an area from — no checkpoints yet, none
	// with positions, or a result too large to be plausible. Callers must fall back rather
	// than substituting a default region.
	RaceArea(year string) (RaceArea, bool, error)

	// ByIDs returns the named checkpoints that exist, are not deleted, and have a position.
	//
	// Ids that do not resolve are **dropped silently**, which is a requirement rather than leniency: a
	// map sheet carries the checkpoint ids that were saved, and nothing re-publishes them when a
	// checkpoint later disappears — in particular, deleting a checkgroup emits no per-checkpoint event.
	// So a stale id is an ordinary state of the data, and the caller has nothing useful to do with an
	// error about it (see task 256).
	//
	// An empty or nil id list returns no checkpoints and runs no query. That is the safe reading of
	// "the caller named nothing": the alternative — treating an empty filter as "everything" — is the
	// classic way a bounded read becomes an unbounded one.
	ByIDs(year string, ids []types.CheckpointID) ([]Checkpoint, error)

	// ByCheckgroups returns the positioned checkpoints in the named checkgroups.
	//
	// The read behind reveal rule 3: scanning a checkpoint reveals its whole group. Bounded the same
	// way as ByIDs, and empty in means empty out for the same reason.
	ByCheckgroups(year string, groups []types.CheckgroupID) ([]Checkpoint, error)
}

// Checkpoint is a checkpoint as it may be shown to the patrol that has earned sight of it.
//
// Every field here is publishable **given that the caller had grounds to name it** — which is what the
// bounded reads guarantee. There is no flag on this struct saying "safe to show": safety is a property of
// how the value was obtained, not of the value, and a boolean here would invite somebody to set it.
type Checkpoint struct {
	ID   types.CheckpointID
	Name string

	// Checkgroup and SortOrder give route order together with the group's own order — one sequence for
	// every patrol.
	Checkgroup types.CheckgroupID
	SortOrder  int

	// Lat and Lng are always set: an unpositioned checkpoint is not returned at all, because there is
	// nothing to draw and nothing to point an arrow at. It is not an error — organizers add posts before
	// siting them — so it is simply absent.
	Lat float64
	Lng float64

	// The open window as stored. OpenFromUts/OpenUntilUts are absolute instants for a fixed scheme;
	// OpenDuration is minutes for a relative one, whose anchor is per patrol and resolved elsewhere.
	// All zero when the group's scheme is none, which yields no verdict.
	OpenFromUts  int64
	OpenUntilUts int64
	OpenDuration int
}

type querier struct {
	db cqrs.Reader
	// positionless reports how many checkpoints lacked a position, in aggregate. See
	// ReportPositionless.
	positionless func(year string, positionless, total int)
}

// RaceArea derives the area from the projection.
//
// Computed on demand rather than stored. It is a handful of rows and a hull over at most a few
// dozen points, so caching it would add an invalidation problem — the checkpoint set changes
// up to the event — in exchange for microseconds.
func (q querier) RaceArea(year string) (RaceArea, bool, error) {
	rows, err := q.db.Query(`
		SELECT latitude, longitude
		FROM checkpoint
		WHERE year = ? AND deleted = 0
		ORDER BY checkpointId`, year)
	if err != nil {
		return RaceArea{}, false, err
	}
	defer rows.Close()

	var points []Point
	total := 0
	for rows.Next() {
		var lat, lng sql.NullFloat64
		if err := rows.Scan(&lat, &lng); err != nil {
			return RaceArea{}, false, err
		}
		total++
		if !lat.Valid || !lng.Valid {
			continue
		}
		points = append(points, Point{Lat: lat.Float64, Lng: lng.Float64})
	}
	if err := rows.Err(); err != nil {
		return RaceArea{}, false, err
	}

	// Reported here rather than per event: individual gaps are expected, the systematic case
	// is what matters, and only the aggregate shows it (see ReportPositionless).
	if q.positionless != nil && total > 0 {
		q.positionless(year, total-len(points), total)
	}

	area, ok := ComputeRaceArea(points, total)
	return area, ok, nil
}

// selectCheckpoint lists the columns a publishable Checkpoint needs.
//
// A named constant so the two bounded reads cannot drift apart, and so adding a column is one edit with
// one place to think about whether it may reach a participant.
const selectCheckpoint = `SELECT checkpointId, name, checkgroupId, sortOrder,
	latitude, longitude, openFromUts, openUntilUts, openDuration
	FROM checkpoint`

// ByIDs returns the named checkpoints that exist, are not deleted, and are positioned.
//
// Bounded by the caller's list — see the interface doc for why that is the point rather than a
// convenience. An empty list short-circuits: no query, no rows. Treating "named nothing" as "everything"
// is how a bounded read silently becomes an unbounded one, so it is refused explicitly rather than left
// to SQL's `IN ()`.
func (q querier) ByIDs(year string, ids []types.CheckpointID) ([]Checkpoint, error) {
	if len(ids) == 0 {
		return []Checkpoint{}, nil
	}

	args := make([]any, 0, len(ids)+1)
	args = append(args, year)
	for _, id := range ids {
		args = append(args, string(id))
	}

	query := selectCheckpoint + `
		WHERE year = ? AND deleted = 0
		  AND latitude IS NOT NULL AND longitude IS NOT NULL
		  AND checkpointId IN (` + placeholders(len(ids)) + `)
		ORDER BY sortOrder ASC, checkpointId ASC`

	return q.scanCheckpoints(query, args...)
}

// ByCheckgroups returns the positioned checkpoints in the named groups.
//
// The read behind reveal rule 3. Bounded and empty-safe for the same reasons as ByIDs.
func (q querier) ByCheckgroups(year string, groups []types.CheckgroupID) ([]Checkpoint, error) {
	if len(groups) == 0 {
		return []Checkpoint{}, nil
	}

	args := make([]any, 0, len(groups)+1)
	args = append(args, year)
	for _, id := range groups {
		args = append(args, string(id))
	}

	query := selectCheckpoint + `
		WHERE year = ? AND deleted = 0
		  AND latitude IS NOT NULL AND longitude IS NOT NULL
		  AND checkgroupId <> '' AND checkgroupId IN (` + placeholders(len(groups)) + `)
		ORDER BY sortOrder ASC, checkpointId ASC`

	return q.scanCheckpoints(query, args...)
}

func (q querier) scanCheckpoints(query string, args ...any) ([]Checkpoint, error) {
	rows, err := q.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Checkpoint{}
	for rows.Next() {
		var c Checkpoint
		if err := rows.Scan(&c.ID, &c.Name, &c.Checkgroup, &c.SortOrder,
			&c.Lat, &c.Lng, &c.OpenFromUts, &c.OpenUntilUts, &c.OpenDuration); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// placeholders renders n comma-separated `?` marks.
//
// The ids come from another service's event bodies by way of a JSON column, so they are bound as
// arguments rather than interpolated — unlike the write path, where cqrs.Writer takes a finished
// statement and quoting is unavoidable.
func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return "?" + strings.Repeat(", ?", n-1)
}

var _ Queries = querier{}
