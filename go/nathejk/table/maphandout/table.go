// Package maphandout records which printed map sheets have been handed to which team, so a patrol
// can see its own list and so the reveal rule knows which sheets it holds (PRD 016).
//
// # The event is `registered`, not `scanned`
//
// The two `qr` subjects look interchangeable and are not:
//
//   - `NATHEJK.{year}.qr.{qrId}.registered` binds a printed code — and the sheet it sits on — to a
//     team. That *is* the handover, and it is what this package consumes.
//   - `NATHEJK.{year}.qr.{qrId}.scanned` is a scan of the team's code at a post. That is a post
//     visit, and it belongs to the `scan` projection.
//
// Consuming the wrong one produces a handout list that grows at every checkpoint, which would look
// plausible for about one race.
//
// # Where it lives
//
// Alongside `kort`, `checkpoint` and `person` under nathejk/table/, so it takes only the cqrs
// interfaces and must never import nathejk.dk/internal/... — see the person package's doc for the
// full reasoning.
package maphandout

import (
	"fmt"

	"github.com/jrgensen/cqrs"

	_ "embed"
)

//go:embed table.sql
var tableSchema string

// Table is the maphandout projection: a cqrs.Consumer that folds QR registrations into the read
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
		return nil, fmt.Errorf("maphandout: create table: %w", err)
	}
	return &Table{
		consumer: consumer{w: w},
		querier:  querier{db: r},
	}, nil
}

// CreateTableSql exposes the schema, matching the other entities' shape.
func (t *Table) CreateTableSql() string { return tableSchema }
