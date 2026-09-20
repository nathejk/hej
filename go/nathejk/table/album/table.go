// Package album projects the curated photo albums shown on the public frontpage (PRD 011 §6, task 333).
//
// # Why this is not glimt with a flag
//
// Both hold photographs and both end up on a public page, so the obvious move is an `audience` value on
// `glimt`. That would be wrong in three ways that matter:
//
//  1. **Consent works differently.** A public glimt is published by the member who took it, tapping
//     "Offentligt" for their own photograph. An album photograph is published by an organizer who
//     obtained permission out of band (PRD 011 §0b.2). Those are different promises to different people,
//     and one column cannot record both.
//  2. **Albums are editorial.** They have a title, an order, a staging state and a URL. A glimt has
//     none of those and should not grow them.
//  3. **Only albums carry a coordinate.** A glimt's GPS is destroyed and nothing replaces it, precisely
//     because the member who shared a photograph did not agree to share a position (PRD 011 §6). Giving
//     `glimt` a nullable coordinate column would put that field one careless migration away from being
//     populated.
//
// # What it deliberately does not record
//
// **Who curated it.** No uploader, no curator, no person at all. Every read here is served to an
// unauthenticated page, and a `curatorPersonId` column would be a personal identifier sitting on the one
// surface that must name no person (task 337). There is also no audit question it would answer: photo
// permission is settled upstream, so this table's job is to hold what was published and to be able to
// stop holding it.
//
// # Where it lives
//
// Alongside `glimt`, `checkpoint` and `person` under nathejk/table/, so it takes only the cqrs interfaces
// and must never import nathejk.dk/internal/... — see the person package's doc for the full reasoning.
// That constraint is why `validRef` and `validSlug` are duplicated here rather than shared.
package album

import (
	"fmt"

	"github.com/jrgensen/cqrs"

	_ "embed"
)

//go:embed table.sql
var tableSchema string

// Table is the album projection: a cqrs.Consumer that folds album events into the read model, and the
// querier the app reads through.
type Table struct {
	consumer
	querier
}

// New creates the tables if needed and returns the projection.
//
// The publisher is accepted but unused: this projection is read-only, and the events are published by
// cmd/api through internal/commands. It is in the signature because every other entity has it and a
// consistent shape is worth more than dropping one parameter.
func New(_ cqrs.Publisher, w cqrs.Writer, r cqrs.Reader) (*Table, error) {
	if err := w.Consume(tableSchema); err != nil {
		return nil, fmt.Errorf("album: create tables: %w", err)
	}
	return &Table{
		consumer: consumer{w: w},
		querier:  querier{db: r},
	}, nil
}

// CreateTableSql exposes the schema, matching the other entities' shape.
func (t *Table) CreateTableSql() string { return tableSchema }
