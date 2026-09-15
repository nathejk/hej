package maphandout

import (
	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/types"
)

// Queries is the read API handed to the application.
//
// One read, deliberately: a team's own handouts. There is no "who holds code X" and no "every
// handout this year" — an organizer wants both and a participant may have neither, and the safest
// way to keep an organizer's question out of a participant's app is for it to be unaskable here.
type Queries interface {
	// ByPatrol returns every sheet ever handed to this team, oldest first.
	//
	// Empty is a normal answer, not an error: a patrol before its first handout, and every user
	// without a patrol at all.
	ByPatrol(year string, teamID types.TeamID) ([]Handout, error)
}

// Handout is one map sheet this team was given.
//
// # What is absent, and why
//
// HQ's equivalent carries the successor team's id, number and name — who holds a reassigned sheet
// now — because an organizer needs to chase it. Those fields are not here and are not selected by
// the query, so no handler can return them by accident and no future edit can "just add" them
// without noticing this comment (PRD 016 §4).
type Handout struct {
	// QrID is the printed sticker number.
	//
	// Worth returning to a participant, unlike most internal ids: it is printed on the physical
	// sheet, so it is the one identifier a patrol can read aloud down a phone when nothing else
	// matches — which is the problem this feature exists to solve.
	QrID string

	// MapID is the kort sheet, or "" for a code registered before its sheet was recorded.
	//
	// "" means *unknown sheet*, not *no sheet*: the patrol is holding something. The caller joins
	// this to `kort` for a name and renders "Ukendt kort" when it cannot.
	MapID string

	// FirstUts is when this team was first given the code; LastUts the most recent binding.
	//
	// Unix **seconds**, as upstream carries them. Equal for a code handed over once.
	FirstUts int64
	LastUts  int64

	// Current is true when this team is still the code's holder.
	//
	// False means the sheet was reassigned — to a discontinued team's successor, typically — and the
	// app shows it as "afleveret" without naming anybody. Note this does **not** un-reveal its
	// checkpoints: revealing is monotonic, because the knowledge left with the scout rather than the
	// sheet (PRD 016 §11.4).
	Current bool
}

type querier struct {
	db cqrs.Reader
}

// byPatrolQuery is the one read this package serves.
//
// A package-level constant rather than an inline string so a test can assert on its text. That is
// not merely convenient: the guarantee this projection makes is about a column that is *absent*, and
// no assertion on a result set can show the absence of a column nobody has added yet. See
// TestByPatrolNeverSelectsTheSuccessorTeam.
const byPatrolQuery = `
		SELECT mh.qrId, mh.mapId, mh.firstUts, mh.lastUts,
		       (cur.teamId = mh.teamId) AS current
		FROM maphandout mh
		JOIN (
			SELECT year, qrId, teamId,
			       ROW_NUMBER() OVER (PARTITION BY year, qrId ORDER BY lastUts DESC, teamId DESC) rn
			FROM maphandout
		) cur ON cur.year = mh.year AND cur.qrId = mh.qrId AND cur.rn = 1
		WHERE mh.year = ? AND mh.teamId = ?
		ORDER BY mh.firstUts ASC, mh.qrId ASC`

// ByPatrol returns the team's handout history.
//
// # How "current" is derived without naming the successor
//
// The current holder of a code is its newest binding. Derived on read with a window function rather
// than stored, because "current" is a property of the whole set of a code's bindings, not of any one
// row — a stored flag would have to be flipped on every row of a code whenever it moved.
//
// The subquery picks only the winning `teamId` and compares it to the caller's, so the successor's
// identity is used inside the query and never leaves it. `teamId` breaks ties so the pick is stable
// when two bindings share a timestamp.
//
// The sheet's *name* is deliberately not joined here: this package has no business knowing about
// `kort`, and joining across two projections' tables would couple their rebuild order. The caller
// resolves the name (task 267).
func (q querier) ByPatrol(year string, teamID types.TeamID) ([]Handout, error) {
	rows, err := q.db.Query(byPatrolQuery, year, string(teamID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Handout{}
	for rows.Next() {
		var h Handout
		if err := rows.Scan(&h.QrID, &h.MapID, &h.FirstUts, &h.LastUts, &h.Current); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

var _ Queries = querier{}
