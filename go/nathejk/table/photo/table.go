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

	// Additive drift only, following `checkpoint` and `person`. Every column here is **also** in table.sql, and
	// the duplication is deliberate: table.sql builds a correct table on a fresh database, and these calls bring
	// an existing one forward. `CREATE TABLE IF NOT EXISTS` is a no-op against a database that has already
	// booted once, so without this an existing deployment keeps the old table and every read that names the new
	// column fails at its first query.
	//
	// Found exactly that way: the credit line's column was added to table.sql, the suite passed — the stubs in
	// `cmd/api` never touch a real schema — and the dev database simply did not have the column after a rebuild.
	for _, col := range []struct{ name, ddl string }{
		// The photographer's credit line (task 393). See table.sql for why this is the one column in this
		// projection that names a person, and what bounds that.
		{"credit", `credit VARCHAR(160) NOT NULL DEFAULT ""`},
		// The 800px rendition (task 409, PRD 023 §7.9) — the only schema change in that PRD.
		//
		// No backfill accompanies it, and none is needed: the column defaults to "", and "" means "fall
		// back to the full image" everywhere it is read. So an existing library keeps working the moment
		// this column appears, and photographs uploaded afterwards get the rendition.
		{"mediumRef", `mediumRef VARCHAR(64) NOT NULL DEFAULT ""`},
		// The photographer's file (PRD 027 R3). See table.sql for what these are, why the metadata is **not**
		// stripped from them, and what enforces that no reader can reach them.
		//
		// No backfill accompanies these and none is possible: the uploaded bytes left with the HTTP request, so
		// every existing row keeps ""/0 forever and readers fall back to `blobRef`. That is the same no-migration
		// property the 800px rendition had, for a different and permanent reason.
		//
		// Five columns that the fold writes as one group — a ref without its dimensions would describe a
		// photograph that does not exist, and an original is the one thing here with nothing to check it against.
		{"originalRef", `originalRef VARCHAR(64) NOT NULL DEFAULT ""`},
		{"originalContentType", `originalContentType VARCHAR(80) NOT NULL DEFAULT ""`},
		{"originalBytes", `originalBytes INT NOT NULL DEFAULT 0`},
		{"originalWidth", `originalWidth INT NOT NULL DEFAULT 0`},
		{"originalHeight", `originalHeight INT NOT NULL DEFAULT 0`},
	} {
		if err := cqrs.EnsureColumn(r, w, "photo", col.name, col.ddl); err != nil {
			return nil, fmt.Errorf("photo: ensure column %s: %w", col.name, err)
		}
	}

	// And the indexes for those columns, which `EnsureColumn` does not add.
	//
	// Easy to miss, and it matters here specifically: every ref column is interrogated by `RefsInUse`
	// inside a **delete path**, so an unindexed one turns a purge check into a scan of the year's
	// photographs. On a fresh database table.sql declares the key; on an existing one only this adds it.
	for _, idx := range []struct{ name, ddl string }{
		// The 800px rendition's ref (task 409).
		{"medium_lookup", "ALTER TABLE photo ADD KEY medium_lookup (mediumRef)"},
		// The original's ref (PRD 027). More at stake than the renditions': `RefsInUse` consults it inside the
		// library takedown, where the two ways to be wrong are destroying the only copy of somebody's file and
		// leaving an EXIF-bearing photograph on disk after it was taken down.
		{"original_lookup", "ALTER TABLE photo ADD KEY original_lookup (originalRef)"},
	} {
		if err := cqrs.EnsureIndex(r, w, "photo", idx.name, idx.ddl); err != nil {
			return nil, fmt.Errorf("photo: ensure index %s: %w", idx.name, err)
		}
	}

	return &Table{
		consumer:       consumer{w: w},
		querier:        querier{db: r},
		curatorQuerier: curatorQuerier{db: r},
	}, nil
}

// CreateTableSql exposes the schema, matching the other entities' shape.
func (t *Table) CreateTableSql() string { return tableSchema }
