// Package pagereport records a report from the open web that a public page should come down
// (PRD 011 §6, task 343).
//
// # Why this exists when PRD 011 §0b.1 settled that the pages may be public
//
// Not because the design is in doubt. A patrol's merged route may be published and the right to publish
// it was obtained. This exists because a page can be **wrong** — a scan attributed to the wrong patrol, a
// track that is not theirs — and because a patrol may have a reason nobody anticipated. The public
// surface already promises "Skriv til os, så tager vi det ned" in its footer; this is what makes the
// promise true.
//
// # What it does not do
//
// It does not hide anything. A report is a notification, not an action — see cmd/api/patrolreport.go for
// the reasoning, which is that a patrol page is not a photograph and taking one down on a single
// anonymous request is trivially abusable.
//
// # No querier
//
// Every other table here has one; this one does not, deliberately. There is no in-app moderation surface
// for these reports — organizers read the table out of band — and a query nothing calls is dead code that
// invites somebody to build the surface it implies. Add the querier with the surface, not before.
//
// # Where it lives
//
// Alongside the other projections under nathejk/table/, so it takes only the cqrs interfaces and must
// never import nathejk.dk/internal/... (see the person package's doc). Unlike the upstream-fed ones it
// does not import shared-go's messages either: this app both publishes and consumes this event, so the
// shape is owned here, in events.go.
package pagereport

import (
	"fmt"

	"github.com/jrgensen/cqrs"

	_ "embed"
)

//go:embed table.sql
var tableSchema string

// Table is the projection: a cqrs.Consumer folding reports into the audit table.
type Table struct {
	consumer
}

// New creates the table if needed and returns the projection.
//
// The publisher is accepted and unused, matching every other entity's signature. Reports are published by
// cmd/api through the app's own command publisher, which is the same stream by a different handle.
func New(_ cqrs.Publisher, w cqrs.Writer, _ cqrs.Reader) (*Table, error) {
	if err := w.Consume(tableSchema); err != nil {
		return nil, fmt.Errorf("pagereport: create table: %w", err)
	}
	return &Table{consumer: consumer{w: w}}, nil
}

// CreateTableSql exposes the schema, matching the other entities' shape.
func (t *Table) CreateTableSql() string { return tableSchema }
