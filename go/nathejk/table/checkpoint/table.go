// Package checkpoint projects the event's checkpoints, for one purpose: deriving the
// race area that the map's offline tile cache is scoped to.
//
// # Why this exists
//
// The tile cache needs to know which region to hold (PRD 002 §11.2, task 087), and the
// race area is defined as the convex hull of the year's checkpoints plus a 3 km buffer —
// every checkpoint is inside the race area by definition. The client cannot compute that
// without the positions, and the positions live only on the event stream.
//
// # Why it is not a constant
//
// Pasting a bounding box into the frontend would be cheaper and wrong. The checkpoint set
// is edited up to the event, and the area moves from year to year, so a hardcoded region
// silently under-caches — and the symptom is a blank map in a forest at 03:00. What *is*
// stable is the area's approximate size, which is why the storage budget can be fixed once
// (PRD 009) while the polygon cannot.
//
// # What it deliberately does not do
//
// It does not serve checkpoint positions to participants. The event area is deliberately
// not fully known to them (PRD 002), so only the hull is exposed — see Queries.RaceArea and
// the handler that wraps it. Keeping the raw positions inside this package is the whole
// reason the read API returns an area rather than a list of points.
//
// # Where it lives
//
// Alongside `person` under nathejk/table/, bound for shared-go eventually, so it takes only
// the cqrs interfaces and must never import nathejk.dk/internal/... — see the person
// package's doc for the full reasoning.
package checkpoint

import (
	"fmt"

	"github.com/jrgensen/cqrs"

	_ "embed"
)

//go:embed table.sql
var tableSchema string

// Table is the checkpoint projection: a cqrs.Consumer that folds checkpoint events into
// the read model, and the querier the app reads through.
type Table struct {
	consumer
	querier
}

// New creates the table if needed and returns the projection.
//
// The publisher is accepted but unused: this projection is read-only. It is in the
// signature because every other entity has it and a consistent shape is worth more than
// dropping one parameter.
func New(_ cqrs.Publisher, w cqrs.Writer, r cqrs.Reader, opts ...Option) (*Table, error) {
	if err := w.Consume(tableSchema); err != nil {
		return nil, fmt.Errorf("checkpoint: create table: %w", err)
	}

	t := &Table{
		consumer: consumer{w: w},
		querier:  querier{db: r},
	}
	for _, opt := range opts {
		opt(t)
	}
	return t, nil
}

// Option configures the projection.
//
// Options exist so the package can report conditions the application cares about without
// importing the application's logger — see the person package for the same pattern.
type Option func(*Table)

// ReportPositionless installs a sink for the count of checkpoints that carry no position.
//
// Deliberately a **count**, reported when the area is computed, rather than an event per
// checkpoint. Individual gaps are expected and fine (maintainer, 2026-08-26): organizers
// add posts before siting them, and the 3 km buffer absorbs a few. What is worth noticing is
// the *systematic* case — a year where the field stops being filled in, leaving an area
// derived from two points — and that is only visible in aggregate.
func ReportPositionless(report func(year string, positionless, total int)) Option {
	return func(t *Table) { t.querier.positionless = report }
}

// CreateTableSql exposes the schema, matching the other entities' shape.
func (t *Table) CreateTableSql() string { return tableSchema }
