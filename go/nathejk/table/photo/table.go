// Package photo projects the year's photograph library — "the bulk" (PRD 022 §8.3, task 363).
//
// # What this is, next to album
//
// An `album` is an *arrangement*; a `photo` is a *photograph*. Before PRD 022 there was only the former:
// `album_item` carried the blob refs, the caption, the coordinate and the verdict inline, and a
// photograph came into existence by being put in an album.
//
// That was a sound design for the feature PRD 011 described and it is worth being explicit that this
// package is not a correction of it. It is a response to a workflow PRD 011 did not describe: our
// photographers hand in a card, and the editing happens afterwards. See table.sql for the three specific
// places the old shape breaks under that — a photograph in no album, a photograph in several, and a
// location set on forty at once.
//
// # What this is, next to glimt
//
// The distinction `album`'s package doc draws applies unchanged and is the reason this is not an
// `audience` column on `glimt`. Briefly: consent works differently (a glimt is published by the member
// who took it; a library photograph is published by an organizer who obtained permission out of band,
// PRD 011 §0b.2), and **only these carry a coordinate**, precisely because the member who shared a
// photograph did not agree to share a position.
//
// # Nothing here is publishable
//
// There is no `published` column and no visibility flag of any kind. A photograph reaches the open web
// only by being referenced from a published `album`, so this projection *cannot* put one there. That is
// the safety property PRD 011 §0b relies on, expressed as an absence rather than as a flag somebody has
// to remember to leave alone — and it is why the curator's reads (task 366) can be permissive about
// drafts without that permissiveness being reachable from a public handler.
//
// # What it deliberately does not record
//
// **Who uploaded it and who curated it.** No uploader, no curator, no person at all. `album`'s reason is
// inherited — its reads serve unauthenticated pages and a `curatorPersonId` would be a personal
// identifier on the one surface that must name no person (task 337) — and this package adds its own:
// PRD 022 §8.2 takes a shared credential, so there is no honest answer to "who", and a column that could
// only ever hold a guess is worse than none because the next reader will believe it.
//
// # Where it lives
//
// Alongside `glimt`, `album`, `checkpoint` and `person` under nathejk/table/, so it takes only the cqrs
// interfaces and must never import nathejk.dk/internal/... — see the person package's doc for the full
// reasoning. That constraint is why `validRef` is duplicated here rather than shared with blob.Ref.Valid.
package photo

import (
	"fmt"

	"github.com/jrgensen/cqrs"

	_ "embed"
)

//go:embed table.sql
var tableSchema string

// Table is the photograph projection: a cqrs.Consumer that folds photograph events into the read model, and
// the queriers the app reads through.
//
// Both queriers are embedded, so `*Table` satisfies `Queries` and `CuratorQueries` — but they are separate
// *types* (see curator.go), so the wiring in cmd/api has to choose which interface it hands to whom. That
// choice is the boundary PRD 022 §8.8 draws, and it is made once, visibly, in main.go.
type Table struct {
	consumer
	querier
	curatorQuerier
}

// New creates the tables if needed and returns the projection.
//
// The publisher is accepted but unused: this projection is read-only, and the events are published by
// cmd/api through internal/commands. It is in the signature because every other entity has it and a
// consistent shape is worth more than dropping one parameter.
func New(_ cqrs.Publisher, w cqrs.Writer, r cqrs.Reader) (*Table, error) {
	if err := w.Consume(tableSchema); err != nil {
		return nil, fmt.Errorf("photo: create tables: %w", err)
	}
	return &Table{
		consumer:       consumer{w: w},
		querier:        querier{db: r},
		curatorQuerier: curatorQuerier{db: r},
	}, nil
}

// CreateTableSql exposes the schema, matching the other entities' shape.
func (t *Table) CreateTableSql() string { return tableSchema }
