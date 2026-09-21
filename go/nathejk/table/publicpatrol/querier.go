package publicpatrol

import (
	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/types"
)

// Queries is the read API handed to the application.
//
// One read, by the number in the URL. There is deliberately no "list every patrol": the public surface
// has no page that wants one, and a list read is what a scraper would ask for. A visitor arrives with a
// number and gets that patrol or nothing.
type Queries interface {
	// ByNumber returns one patrol's public details.
	//
	// found is false for an unknown number and for a patrol that has not been given one. The handler must
	// answer identically in both cases — and identically to a patrol whose gate is closed (task 330) —
	// because the differences between them are exactly what leaks which numbers are real and which
	// patrols have finished.
	ByNumber(year, number string) (Patrol, bool, error)
}

// Patrol is everything the public page may say about a patrol, and nothing else.
//
// # The type is the boundary
//
// There is no name of a person here, no phone, no email, no contact anything — not because they are
// filtered on the way out but because the table has no such column and the fold never wrote one. A
// reviewer can check this claim by reading four field names instead of auditing every query.
//
// Task 337's structural test walks these fields and fails on anything person-shaped, so the claim is
// enforced rather than asserted in a comment.
type Patrol struct {
	// TeamID is the internal id. Needed to ask the other projections about this patrol — its scans, its
	// tracks — and never rendered: it is not a secret, but it is also not information a visitor has any
	// use for, and an id in a page is an id in somebody's crawler.
	TeamID string

	// Number is the patrol's number, as printed on its sign.
	Number string

	// Name is the *patrol's* name — "Ørnene".
	Name string

	// GroupName is the scout group, e.g. "1. Søllerød Gruppe".
	GroupName string

	// Korps is the korps **slug** (`dds`, `kfuk`, …). Use KorpsLabel for display.
	Korps string
}

// KorpsLabel is the korps as a human reads it, or "" when there is nothing worth showing.
//
// # Why the empty string rather than "Andet"
//
// `andet` ("other") and an unset value are both "unspecified", and a header line reading **"Andet"**
// tells a visitor nothing while looking like a bug. So both collapse to empty and the template omits the
// segment entirely.
//
// The mapping lives in shared-go (`types.CorpsLabels`), which is the same table the organizers' own
// screens render from — so a corrected label reaches this page by upgrading the dependency rather than by
// editing a copy here.
func (p Patrol) KorpsLabel() string {
	if p.Korps == "" || p.Korps == string(types.CorpsSlugOther) {
		return ""
	}
	label := types.CorpsSlug(p.Korps).Label()
	if label == "" {
		// A slug shared-go does not know. Rendering the raw slug would put "dds" in a header; rendering
		// nothing is the honest answer to "we do not know what this korps is called".
		return ""
	}
	return label
}

type querier struct {
	db cqrs.Reader
}

// ByNumber returns one patrol by the number in its URL.
//
// # Why LIMIT 1 rather than a unique key
//
// Nothing upstream guarantees one number per year — the number is assigned by an event, and two events
// could in principle name the same number for different teams. A unique index would make the *projection*
// fail on replay, which would take down every public page over a data problem that affects one. So the
// read takes the first match, ordered by teamId so it is stable across restarts rather than whatever the
// storage engine returns first.
//
// An empty number returns nothing and runs no query. A patrol without a number has no page, and treating
// "" as a filter would match every un-numbered patrol in the event and hand back an arbitrary one.
func (q querier) ByNumber(year, number string) (Patrol, bool, error) {
	if number == "" {
		return Patrol{}, false, nil
	}

	rows, err := q.db.Query(`
		SELECT teamId, teamNumber, name, groupName, korps
		FROM public_patrol
		WHERE year = ? AND teamNumber = ?
		ORDER BY teamId ASC
		LIMIT 1`, year, number)
	if err != nil {
		return Patrol{}, false, err
	}
	defer rows.Close()

	if !rows.Next() {
		return Patrol{}, false, rows.Err()
	}
	var p Patrol
	if err := rows.Scan(&p.TeamID, &p.Number, &p.Name, &p.GroupName, &p.Korps); err != nil {
		return Patrol{}, false, err
	}
	return p, true, rows.Err()
}

var _ Queries = querier{}
