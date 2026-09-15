package kort

import (
	"fmt"

	"github.com/jrgensen/cqrs"

	_ "embed"
)

//go:embed table.sql
var kortSchema string

//go:embed kortsaet.sql
var kortsaetSchema string

// Table is the kort projection: a cqrs.Consumer that folds sheet and set events into the read
// model, and the querier the app reads through.
type Table struct {
	consumer
	querier
}

// New creates the tables if needed and returns the projection.
//
// The publisher is accepted but unused: this projection is read-only, and everything it holds is
// owned by another service. It is in the signature because every other entity has it and a
// consistent shape is worth more than dropping one parameter.
func New(_ cqrs.Publisher, w cqrs.Writer, r cqrs.Reader, opts ...Option) (*Table, error) {
	// Two statements rather than one file: the sheets and the sets are separate tables with
	// separate reasons to exist, and keeping the schemas apart keeps each one's comments next to
	// the columns they explain.
	for _, schema := range []string{kortsaetSchema, kortSchema} {
		if err := w.Consume(schema); err != nil {
			return nil, fmt.Errorf("kort: create table: %w", err)
		}
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
// Options exist so the package can report conditions the application cares about without importing
// the application's logger — see the person and checkpoint packages for the same pattern.
type Option func(*Table)

// ReportUnknownBody installs a sink for an event body this projection could not decode.
//
// Worth a report rather than a silent skip, and worth *only* a report rather than a returned error.
// These shapes are mirrored from another repo's deliberately-unstable types (see messages.go), so a
// body that will not decode is the single signal that our mirror has drifted from the contract — and
// the alternative, failing the handler, makes the stream library drop the message, which loses a
// sheet whose checkpoints a patrol is entitled to see.
//
// Unknown *fields* never reach this: they are ignored by design, because an additive upstream field
// must not break a consumer in the middle of an event.
func ReportUnknownBody(report func(subject string, err error)) Option {
	return func(t *Table) { t.consumer.unknownBody = report }
}

// ReportCounts installs a sink for the aggregate shape of the year's map definitions.
//
// A count, reported once, rather than an event per sheet — the same reasoning as
// checkpoint.ReportPositionless. This feature's characteristic failure is silence: a year whose
// sheets are all in sets with no team type, or a set marked for the wrong one, yields an app that
// works perfectly and shows an empty map. "12 sheets, 0 in a patrulje set" is the shape of that
// disaster, and it is only visible in aggregate.
//
// Wired to the application's logger by task 260.
func ReportCounts(report func(year string, sets, patrolSets, sheets, patrolSheets int)) Option {
	return func(t *Table) { t.querier.counts = report }
}

// CreateTableSql exposes the schema, matching the other entities' shape.
func (t *Table) CreateTableSql() string { return kortsaetSchema + "\n" + kortSchema }
