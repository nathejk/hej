// Package checkgroup projects the event's checkgroups — the postlinje groups checkpoints belong to —
// for the reveal rule and the on-time verdict (PRD 016).
//
// # Why a whole projection for four columns
//
// Because two of the three reveal rules are expressed in terms of a group rather than a checkpoint:
// scanning any checkpoint reveals its whole checkgroup, and a sheet handed out at a post reveals its
// checkpoints once that group is reached. Neither can be evaluated from the checkpoint table alone.
//
// And because a relative open window is anchored on the patrol's own scan at *another* group, which
// means the verdict logic needs `scheme` and `relativeCheckgroupId` to know what it is holding.
//
// # What it deliberately does not project
//
// `showOnMap`. See the note at the foot of table.sql — the short version is that nothing upstream
// reads it, its intent is unverified, and our reveal rule is grounded in physical possession instead,
// which cannot over-reveal whatever the flag means (PRD 016 §11.3).
//
// # Where it lives
//
// Alongside `checkpoint`, `kort` and `person` under nathejk/table/, so it takes only the cqrs
// interfaces and must never import nathejk.dk/internal/... — see the person package's doc for the full
// reasoning.
package checkgroup

import (
	"fmt"

	"github.com/jrgensen/cqrs"

	_ "embed"
)

//go:embed table.sql
var tableSchema string

// Table is the checkgroup projection: a cqrs.Consumer that folds checkgroup events into the read
// model, and the querier the app reads through.
type Table struct {
	consumer
	querier
}

// New creates the table if needed and returns the projection.
//
// The publisher is accepted but unused: this projection is read-only. It is in the signature because
// every other entity has it and a consistent shape is worth more than dropping one parameter.
func New(_ cqrs.Publisher, w cqrs.Writer, r cqrs.Reader) (*Table, error) {
	if err := w.Consume(tableSchema); err != nil {
		return nil, fmt.Errorf("checkgroup: create table: %w", err)
	}
	return &Table{
		consumer: consumer{w: w},
		querier:  querier{db: r},
	}, nil
}

// CreateTableSql exposes the schema, matching the other entities' shape.
func (t *Table) CreateTableSql() string { return tableSchema }
