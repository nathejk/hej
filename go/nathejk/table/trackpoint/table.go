// Package trackpoint projects the recorded position points the post-race patrol route is drawn from
// (PRD 011 §6, task 340).
//
// # Why a projection at all, when the PRD asked for a stream read
//
// PRD 011 §8.4 wanted the track read from `TELEMETRY` on demand, to avoid putting a million points in
// MariaDB. Checking rather than reasoning turned up two problems with that: the stream library has no
// bounded fetch-by-subject (only push `Subscribe` and `LastMessage`), and the volume is ~60–100 MB, which
// is the same order as projections this service already carries.
//
// The full argument, including why deduplication makes the projection actively *better* than an on-demand
// read, is in table.sql. The short version: `(person, timestamp)` is the identity of a point, so dedup is
// a primary key here and would have been a million-point in-memory problem there.
//
// What the PRD actually wanted — that a public page must not do bulk work per request — is kept: the
// merge, the gap-breaking and the simplification happen in `internal/patroltrack` and are cached.
//
// # Where it lives
//
// Alongside the other projections under nathejk/table/, so it takes only the cqrs interfaces and must
// never import nathejk.dk/internal/... . It does import `internal/track`'s event type — no: it cannot,
// and does not. The event body is re-declared here for exactly that reason, and the duplication is named
// in the consumer.
package trackpoint

import (
	"fmt"

	"github.com/jrgensen/cqrs"

	_ "embed"
)

//go:embed table.sql
var tableSchema string

// Table is the projection: a cqrs.Consumer that folds telemetry batches into points, and the querier the
// merge reads through.
type Table struct {
	consumer
	querier
}

// New creates the table if needed and returns the projection.
//
// The publisher is accepted but unused: this projection is read-only. The points are published by the
// authenticated `/api/track` endpoint (PRD 002 §11.1), which is a different part of the app entirely.
func New(_ cqrs.Publisher, w cqrs.Writer, r cqrs.Reader) (*Table, error) {
	if err := w.Consume(tableSchema); err != nil {
		return nil, fmt.Errorf("trackpoint: create table: %w", err)
	}
	return &Table{
		consumer: consumer{w: w},
		querier:  querier{db: r},
	}, nil
}

// CreateTableSql exposes the schema, matching the other entities' shape.
func (t *Table) CreateTableSql() string { return tableSchema }
