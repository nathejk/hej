// Package scan projects the QR scans of teams' codes, and the personnel shifts that say where each
// scan happened (PRD 016).
//
// # A scan does not carry a checkpoint
//
// This is the fact the package is built around. `qr.scanned` carries who scanned and when, and nothing
// about place. A scan counts for a post if the scanner was on a registered shift there at that moment,
// so the checkpoint is *recovered* by a join against `checkpersonnel` rather than read from the event.
//
// The predicate is copied from hq's live implementation on purpose: the app and the organizers' own
// screens must agree about which post a scan happened at and who was on time. Two systems inferring the
// same thing differently would be worse than either inference being imperfect.
//
// # The rota is load-bearing, and it is somebody else's data
//
// With no shifts recorded, no scan can be attributed: post names vanish, on-time verdicts vanish, and
// the reveal rule's "scanning a checkpoint reveals its checkgroup" never fires. The app looks like it is
// working. hq's code carries the same warning about its own equivalent.
//
// Because that failure is invisible from in here, the querier exposes UnattributedCount for task 260 to
// log. A number before the race is the only defence.
//
// # Two tables, one package
//
// `checkpersonnel` has no consumer of its own — it exists solely to attribute scans, and the join is the
// interesting part. Splitting it into its own package would put the one query that matters in neither.
// `person` owns a second `section` table for the same kind of reason.
//
// # Where it lives
//
// Alongside `checkpoint`, `checkgroup`, `kort` and `person` under nathejk/table/, so it takes only the
// cqrs interfaces and must never import nathejk.dk/internal/... — see the person package's doc for the
// full reasoning.
package scan

import (
	"fmt"

	"github.com/jrgensen/cqrs"

	_ "embed"
)

//go:embed table.sql
var scanSchema string

//go:embed checkpersonnel.sql
var checkpersonnelSchema string

// Table is the scan projection: a cqrs.Consumer that folds scans and shifts into the read model, and the
// querier the app reads through.
type Table struct {
	consumer
	querier
}

// New creates the tables if needed and returns the projection.
//
// The publisher is accepted but unused: this projection is read-only. It is in the signature because
// every other entity has it and a consistent shape is worth more than dropping one parameter.
func New(_ cqrs.Publisher, w cqrs.Writer, r cqrs.Reader) (*Table, error) {
	for _, schema := range []string{scanSchema, checkpersonnelSchema} {
		if err := w.Consume(schema); err != nil {
			return nil, fmt.Errorf("scan: create table: %w", err)
		}
	}
	return &Table{
		consumer: consumer{w: w},
		querier:  querier{db: r},
	}, nil
}

// CreateTableSql exposes the schema, matching the other entities' shape.
func (t *Table) CreateTableSql() string { return scanSchema + "\n" + checkpersonnelSchema }
