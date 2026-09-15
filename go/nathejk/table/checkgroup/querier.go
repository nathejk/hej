package checkgroup

import (
	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/types"
)

// Queries is the read API handed to the application.
type Queries interface {
	// ByYear returns the year's checkgroups in route order.
	//
	// Empty is a normal answer early in the year, not an error.
	ByYear(year string) ([]Checkgroup, error)
}

// Checkgroup is one postlinje group.
type Checkgroup struct {
	ID   types.CheckgroupID
	Name string

	// SortOrder is the outer half of route order; the inner half is the checkpoint's.
	SortOrder int

	// Scheme is how this group's open windows are expressed: fixed, relative or none.
	//
	// May be "" when no update has been seen yet, or a value we do not recognise. Both yield no
	// verdict, which is the same outcome as `none` — inventing a verdict from a scheme we do not
	// understand is the failure worth avoiding, since a wrong "for sent" is worse than a missing one.
	Scheme types.CheckgroupScheme

	// RelativeCheckgroupID is the group whose scan starts the window, with Scheme == relative.
	//
	// **Unresolved.** It may name a group that has since been deleted, and nothing re-publishes this
	// one when that happens. The caller resolves it and treats an unresolvable anchor as "no window
	// yet" rather than measuring from a group that will never be reached.
	RelativeCheckgroupID types.CheckgroupID
}

type querier struct {
	db cqrs.Reader
}

// ByYear returns the year's groups in route order.
//
// `checkgroupId` is the tiebreak after sortOrder so two groups sharing an order — easy to produce
// mid-reorder — come back stably rather than in whatever order the storage engine feels like. Two
// consecutive map loads disagreeing about which post is "next" would look like a bug in the arrows.
func (q querier) ByYear(year string) ([]Checkgroup, error) {
	rows, err := q.db.Query(`
		SELECT checkgroupId, name, sortOrder, scheme, relativeCheckgroupId
		FROM checkgroup
		WHERE year = ? AND deleted = 0
		ORDER BY sortOrder ASC, checkgroupId ASC`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Checkgroup{}
	for rows.Next() {
		var cg Checkgroup
		if err := rows.Scan(&cg.ID, &cg.Name, &cg.SortOrder, &cg.Scheme, &cg.RelativeCheckgroupID); err != nil {
			return nil, err
		}
		out = append(out, cg)
	}
	return out, rows.Err()
}

var _ Queries = querier{}
