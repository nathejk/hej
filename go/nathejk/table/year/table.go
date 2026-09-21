// Package year projects what this app knows about an event year: where the walk starts and where it ends
// (task 357).
//
// # Copied from hq, narrowed to two columns
//
// hq owns the write side of this entity — its `nathejk/table/year` has a `commander` that publishes
// `NATHEJK:<year>.updated`, a `Filter` for the admin list, and seven columns. This is the **read side only**:
// the same events, folded into the two fields a diploma needs. Nothing in this app publishes a year event, and
// nothing here should: an organizer edits the year in hq.
//
// The copy is deliberate rather than a shared dependency, for the reason recorded in `person`'s doc and
// enforced by the layout: these packages take only the cqrs interfaces and must never import
// `nathejk.dk/internal/...`, because they are bound for shared-go. hq's `filter.go` imports its own
// `internal/validator`, so it could not have come across as-is even if this app had wanted paging.
//
// # What it is for
//
// The route line on a diploma — *"fra Lundby til Glumsø"*. `internal/diploma` renders it only when configured,
// and until this projection existed nothing could configure it: the two place names lived in the sibling
// `diplom` service's source, hardcoded to 2024. See table.sql.
package year

import (
	"fmt"

	"github.com/jrgensen/cqrs"

	_ "embed"
)

//go:embed table.sql
var tableSchema string

// Table is the projection: a cqrs.Consumer that folds the year events, and the querier the app reads through.
type Table struct {
	consumer
	querier
}

// New creates the table if needed and returns the projection.
//
// The publisher is accepted but unused, matching every other entity here: this projection is read-only
// because the events come from upstream. A consistent shape is worth more than dropping a parameter.
func New(_ cqrs.Publisher, w cqrs.Writer, r cqrs.Reader) (*Table, error) {
	if err := w.Consume(tableSchema); err != nil {
		return nil, fmt.Errorf("year: create table: %w", err)
	}
	return &Table{
		consumer: consumer{w: w},
		querier:  querier{db: r},
	}, nil
}

// CreateTableSql exposes the schema, matching the other entities' shape.
func (t *Table) CreateTableSql() string { return tableSchema }
