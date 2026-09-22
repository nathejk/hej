// Package patrolphoto projects a patrol's photographs and the one chosen to represent it (task 361).
//
// # Where the shape comes from
//
// The `photographed` event is published by **foto**, which owns its shape — it is not in shared-go, because foto
// is the first service to publish it and the projection that consumes it owns the struct. hq has the authoritative
// copy in `nathejk/table/photo`. This package therefore carries a **reader's copy**: the fields this app reads,
// with the same JSON tags, and nothing else.
//
// A partial copy is the honest shape for a consumer. It documents exactly what hej looks at, and a field added
// upstream costs nothing here. What it cannot survive is a field being *renamed* upstream — but neither can hq's
// copy, so the exposure is the same and the fix is the same place.
//
// # The bytes are not here
//
// The event carries refs, never image data. Fetching the bytes into this app's own blob store is
// `internal/photobytes`; this package answers only "which photographs, and which ref represents the patrol".
//
// Read-only, like every upstream-fed projection here: nothing in this app publishes a photograph.
package patrolphoto

import (
	"fmt"

	"github.com/jrgensen/cqrs"

	_ "embed"
)

//go:embed table.sql
var tableSchema string

// Table is the projection: one consumer for three verbs, and the querier the app reads through.
type Table struct {
	consumer
	querier
}

// New creates both tables if needed and returns the projection.
//
// The publisher is accepted and unused, matching every other entity here — the events come from upstream.
func New(_ cqrs.Publisher, w cqrs.Writer, r cqrs.Reader) (*Table, error) {
	if err := w.Consume(tableSchema); err != nil {
		return nil, fmt.Errorf("patrolphoto: create tables: %w", err)
	}
	return &Table{
		consumer: consumer{w: w},
		querier:  querier{db: r},
	}, nil
}

// CreateTableSql exposes the schema, matching the other entities' shape.
func (t *Table) CreateTableSql() string { return tableSchema }
