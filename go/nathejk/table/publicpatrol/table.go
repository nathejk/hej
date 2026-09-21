// Package publicpatrol projects what a patrol's public page may say about it (PRD 011 §8, task 338).
//
// # Why this is not a read of shared-go's `patrulje`
//
// The obvious implementation is to register shared-go's `patrulje` projection and read through its
// `Queries`. That would be fewer moving parts and it is the wrong shape, for one reason: shared-go's
// `Patrulje` struct carries `ContactName`, `ContactPhone`, `ContactEmail` and `ContactRole`, because the
// organizers' screens need them. Reading the public page through it would put a leader's name and mobile
// number in memory inside the handler that renders an unauthenticated page — one field access from a
// template, on the surface whose entire claim is that it names no person (PRD 011 §0b.1).
//
// So this folds the *same upstream events* and writes four columns. The contact fields arrive in the event
// body and are dropped. "It is not here" is a property; "we do not select it" is a habit.
//
// The duplication is real and is the price. See table.sql for the full argument, including why reading
// shared-go's table instead would not be a refactor.
//
// # What it does not answer
//
// Whether the page may be *shown* — that is `internal/publicgate`, and it is a question about scans and
// checkpoints rather than about the patrol. Also the distance (task 339) and the route (task 340), which
// are derived rather than projected. This package answers only "who is this patrol, publicly?".
//
// # Where it lives
//
// Alongside `album`, `glimt`, `checkpoint` and `person` under nathejk/table/, so it takes only the cqrs
// interfaces and must never import nathejk.dk/internal/... — see the person package's doc for the full
// reasoning. It does import shared-go's `messages` and `types`, as the upstream-fed projections here
// already do: those are the published contract, not another service's internals.
package publicpatrol

import (
	"fmt"

	"github.com/jrgensen/cqrs"

	_ "embed"
)

//go:embed table.sql
var tableSchema string

// Table is the projection: a cqrs.Consumer that folds the team events into the narrow read model, and the
// querier the app reads through.
type Table struct {
	consumer
	querier
}

// New creates the table if needed and returns the projection.
//
// The publisher is accepted but unused: this projection is read-only, and nothing in this app publishes a
// team event — they come from upstream. It is in the signature because every other entity has it and a
// consistent shape is worth more than dropping one parameter.
func New(_ cqrs.Publisher, w cqrs.Writer, r cqrs.Reader) (*Table, error) {
	if err := w.Consume(tableSchema); err != nil {
		return nil, fmt.Errorf("publicpatrol: create table: %w", err)
	}
	return &Table{
		consumer: consumer{w: w},
		querier:  querier{db: r},
	}, nil
}

// CreateTableSql exposes the schema, matching the other entities' shape.
func (t *Table) CreateTableSql() string { return tableSchema }
