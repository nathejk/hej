package checkpoint

import (
	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/types"
)

// The curator's checkpoint read (PRD 022 §6, task 376).
//
// # Why this is a separate interface and not a method on Queries
//
// `Queries`' doc comment states the property this package exists to hold: **there is no way to ask it for all
// checkpoints.** `ByIDs` and `ByCheckgroups` are each bounded by what the caller already named, so a
// patrol-scoped handler "cannot leak a position it did not already have grounds to know about". The event area
// is deliberately not fully known to participants (PRD 002), and that interface is the mechanism.
//
// The photographer admin tool genuinely needs the opposite. A curator setting a position on forty photographs
// knows "Post 3", not a coordinate, and making them read a number off a map is asking them to do a conversion
// the service can do (PRD 022 §6). So the list has to exist.
//
// Adding `All(year)` to `Queries` would have been three lines and would have handed every patrol-scoped handler
// in the app a way to enumerate every position in the event — the exact mistake task 366 avoided for albums. So
// it goes on its own interface, wired onto its own field, and the reasoning is the one `publicpatrol/table.go`
// records: *"It is not here" is a property; "we do not select it" is a habit.*
//
// # What it still does not carry
//
// No open window, no scheme, no checkgroup timing. A picker needs a name, a position and an order to sort by.
// The rest of `Checkpoint` exists for the reveal rule, and a wider type here would be a second reason for these
// rows to be in memory in a handler that has no business with them.

// CuratorQueries is the checkpoint read API for the admin tool.
//
// Deliberately not embedding, and not embedded in, `Queries`: either direction would mean a handler with one has
// the other, which is the boundary this exists to draw.
type CuratorQueries interface {
	// Positioned returns the year's sited checkpoints, in route order.
	//
	// Only checkpoints that **have a position**, because a picker offering one that cannot supply a coordinate
	// is a row that does nothing when clicked. Unpositioned checkpoints are an ordinary state — organizers add
	// posts before siting them — so they are absent rather than an error, exactly as `ByIDs` treats them.
	//
	// Empty is a normal answer, and an important one: early in the season no checkpoint has a position, which is
	// also why a curator-placed coordinate gets the `unknown` verdict rather than `outside` in that period. The
	// UI must say so rather than showing an empty list that reads as a fault.
	Positioned(year string) ([]PickerCheckpoint, error)
}

// PickerCheckpoint is one checkpoint as the curator's picker shows it.
//
// A narrower type than `Checkpoint` on purpose — see the package note above. A name, a place, and enough to sort
// by.
type PickerCheckpoint struct {
	ID   types.CheckpointID
	Name string

	// Checkgroup and SortOrder give the route order the picker lists in, so "Post 3" is where a curator expects
	// to find it rather than wherever the id sorts.
	Checkgroup types.CheckgroupID
	SortOrder  int

	Lat float64
	Lng float64
}

// curatorQuerier is a distinct type from `querier`, so that `checkpoint.Table` satisfying one interface cannot
// hand a patrol-scoped handler the other.
type curatorQuerier struct {
	db cqrs.Reader
}

// Positioned returns the year's sited checkpoints in route order.
//
// `latitude IS NOT NULL` is not enough on its own: the projection writes 0,0 for a checkpoint whose position was
// never set, and 0,0 is a real place in the Atlantic. The same exclusion `RaceArea` applies is applied here, for
// the same reason — otherwise the picker offers a post in the Gulf of Guinea and the bounds check dutifully
// judges it `outside`.
func (q curatorQuerier) Positioned(year string) ([]PickerCheckpoint, error) {
	rows, err := q.db.Query(`
		SELECT c.checkpointId, c.name, c.checkgroupId, c.sortOrder, c.latitude, c.longitude
		FROM checkpoint c
		WHERE c.year = ? AND c.deleted = 0
		  AND c.latitude IS NOT NULL AND c.longitude IS NOT NULL
		  AND NOT (c.latitude = 0 AND c.longitude = 0)
		ORDER BY c.sortOrder ASC, c.name ASC`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []PickerCheckpoint{}
	for rows.Next() {
		var c PickerCheckpoint
		if err := rows.Scan(&c.ID, &c.Name, &c.Checkgroup, &c.SortOrder, &c.Lat, &c.Lng); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

var _ CuratorQueries = curatorQuerier{}
