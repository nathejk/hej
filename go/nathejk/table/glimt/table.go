// Package glimt records the moments participants shared: a caption, an ordered list of media, and
// who it was shared with (PRD 019).
//
// # What this projection is for
//
// Three reads, in descending order of how much the design bends for them:
//
//  1. **One hold's collection**, oldest first. The post-race browse (PRD 019 §0a) is the load peak
//     of the whole feature — a thousand people at the finish line, on the worst network of the
//     weekend, paging through thousands of items. `year_team_created` exists for it.
//  2. **The feed**, newest first, filtered by audience.
//  3. **The moderation queue**, reported first, for the Team section.
//
// # Visibility is not decided here
//
// This package stores `audience`, `authorGroup` and `hiddenAt`; it does not interpret them. Who may
// see a glimt is answered by `internal/users.MaySeeGlimt`, which is the single definition, and the
// queriers here take an already-derived filter. That split is deliberate: a projection that also
// knew the access rules would be a second place for them to live, and the failure mode of two
// definitions is a group-scoped photograph of a child served to a stranger.
//
// # Where it lives
//
// Alongside `person`, `kort` and `maphandout` under nathejk/table/, so it takes only the cqrs
// interfaces and must never import nathejk.dk/internal/... — see the person package's doc for the
// full reasoning. That constraint is why the event types are owned here (events.go) rather than in
// an internal package.
package glimt

import (
	"fmt"

	"github.com/jrgensen/cqrs"

	_ "embed"
)

//go:embed table.sql
var tableSchema string

// Table is the glimt projection: a cqrs.Consumer that folds the Glimt events into the read model,
// and the querier the app reads through.
type Table struct {
	consumer
	querier
}

// New creates the tables if needed and returns the projection.
//
// The publisher is accepted but unused, matching every other entity's shape: cmd/api publishes
// through internal/commands, not through the projection.
func New(_ cqrs.Publisher, w cqrs.Writer, r cqrs.Reader) (*Table, error) {
	if err := w.Consume(tableSchema); err != nil {
		return nil, fmt.Errorf("glimt: create tables: %w", err)
	}
	return &Table{
		consumer: consumer{w: w},
		querier:  querier{db: r},
	}, nil
}

// CreateTableSql exposes the schema, matching the other entities' shape.
func (t *Table) CreateTableSql() string { return tableSchema }
