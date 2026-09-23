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
	// The shape change has to happen before the CREATE, because CREATE TABLE IF NOT EXISTS will not
	// alter an existing table. See dropLegacyAlbumItem.
	if err := dropLegacyAlbumItem(w, r); err != nil {
		return nil, err
	}
	if err := w.Consume(tableSchema); err != nil {
		return nil, fmt.Errorf("album: create tables: %w", err)
	}
	return &Table{
		consumer: consumer{w: w},
		querier:  querier{db: r},
	}, nil
}

// dropLegacyAlbumItem rebuilds `album_item` once, if it still has its pre-PRD-022 shape.
//
// # Why this exists at all
//
// `CREATE TABLE IF NOT EXISTS` creates new tables and never changes existing ones, which is exactly what
// the projections want almost always — but PRD 022 §8.3 did not add a column to `album_item`, it replaced
// nine of them with one. On a database that already holds the old table the CREATE is a no-op, the fold's
// INSERT names a `photoId` column that does not exist, and **every album item fails to write** while the
// service otherwise looks healthy. PRD 022 §8.11 requires this to be handled deliberately rather than
// discovered, which is what this function is.
//
// # Why a DROP here does not contradict the person projection's rule
//
// `person/table.go` states the house rule plainly: its column list is additive by design, and "a DROP
// COLUMN on every boot is exactly the kind of destructive statement that pattern exists to keep out". It
// keeps `verifiedAgainstPhone` forever rather than dropping it.
//
// That rule is about **columns holding data somebody might still want**, and two things make this
// different:
//
//  1. `album_item` is a pure projection with no independent truth in it. Every row is derived from an
//     `itemadded` event on a stream that is never rewritten, so dropping the table destroys nothing a
//     replay does not immediately rebuild. `person` carries columns whose source events predate the
//     column, which is why *it* cannot be so casual.
//  2. This runs **once**. After the rebuild `blobRef` no longer exists, the guard finds nothing, and no
//     further boot touches the table. It is a migration that happens to live in code, not a destructive
//     statement on a schedule.
//
// What is genuinely lost is the items of any album created by the old dev fixture, because the new fold
// skips legacy `itemadded` events (see consumer.go). That is accounted for: no album has ever existed
// outside `cmd/api/devalbum.go`, which is the whole reason PRD 022 could change the event shape.
//
// # Why it keys on `blobRef`
//
// One column that existed before and cannot exist after — so its presence is an unambiguous "this is the
// old table". Keying on the *absence* of `photoId` would be subtly wrong: a half-applied migration could
// leave a table with neither, and this way such a table is rebuilt rather than left broken.
func dropLegacyAlbumItem(w cqrs.Writer, r cqrs.Reader) error {
	var n int
	// The same INFORMATION_SCHEMA lookup cqrs.EnsureColumn uses, and the same dialect assumption: this
	// repository is MariaDB, and there is no dialect-neutral way to ask.
	err := r.QueryRow(`
		SELECT COUNT(*)
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = 'album_item'
		  AND COLUMN_NAME = 'blobRef'`).Scan(&n)
	if err != nil {
		return fmt.Errorf("album: check album_item shape: %w", err)
	}
	if n == 0 {
		// Either a fresh database, or one already rebuilt. Both are the normal case.
		return nil
	}
	if err := w.Consume("DROP TABLE album_item"); err != nil {
		return fmt.Errorf("album: rebuild album_item for PRD 022: %w", err)
	}
	return nil
}

// CreateTableSql exposes the schema, matching the other entities' shape.
func (t *Table) CreateTableSql() string { return tableSchema }
