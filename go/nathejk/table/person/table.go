// Package person is the app's member directory: one row per person per event year,
// keyed for lookup by normalized phone number.
//
// # Why this exists
//
// The app needs one question answered on the login hot path — "who owns this phone
// number, and what do they do at this event?" — and a survey of shared-go, hq and
// tilmelding (PRD 006 §2) found that lookup exists nowhere and cannot be composed
// from what does:
//
//   - No phone lookup anywhere: no ByPhone query, no Filter accepting a phone, no
//     index on any phone column, in any of the three repos.
//   - Gøgler people are not in shared-go at all; they live in hq's local personnel
//     table, which this module cannot import.
//   - Crew function is an organizer-authored section slug validated by nothing.
//   - The obvious fan-out read is not viable: spejder.GetByID is a stub returning
//     nil, nil and senior.GetByID's SQL is broken.
//
// So this projection consumes the events directly rather than reading other
// projections, which also means it depends on no other repo's read model.
//
// # Why it lives here and not in internal/
//
// This package is destined for shared-go once the classification rules settle
// (PRD 006 §8). Go forbids importing another module's internal tree, so being born
// under internal/ would block that move. It therefore takes only the cqrs
// interfaces and must never import nathejk.dk/internal/... — if it needs something
// from the application, declare an interface here and let cmd/api satisfy it.
package person

import (
	"fmt"

	"github.com/jrgensen/cqrs"

	_ "embed"
)

//go:embed table.sql
var tableSchema string

// person_section is a slug → label lookup, kept alongside the person table.
//
// It exists to make section events **order-independent**. A crew member's section can
// be assigned before the event that names that section arrives, or after; the
// projection has to converge either way, and a projector cannot read the database
// (cqrs.Writer takes statements, not queries). Holding the labels in a table lets the
// assignment handler resolve a name with a JOIN instead of a Go-side read.
//
// It is deliberately not shared-go's `section` entity: this needs two columns, and
// consuming the same events into a two-column table is cheaper than depending on a
// projection this package would then have to be given.
//
//go:embed section.sql
var sectionSchema string

// Table is the person projection: a cqrs.Consumer that folds member events into the
// read model, and the querier the app reads through.
type Table struct {
	consumer
	querier
}

// New creates the table if needed, applies any additive schema drift, and returns
// the projection.
//
// It takes the three cqrs interfaces and not a *sql.DB or a concrete stream, which
// is what keeps the eventual lift to shared-go mechanical.
//
// The publisher is accepted but unused for now: this projection is read-only today.
// It is in the signature because every shared-go entity has it, and because
// invalidating a verification on a guardian-number change (task 076) may need to
// publish. Keeping the shape consistent is worth more than dropping one parameter.
//
// The normalizer is required: the projector must fold phone numbers with the *same*
// implementation the login handler uses, or lookups silently miss (see
// interfaces.go).
func New(_ cqrs.Publisher, w cqrs.Writer, r cqrs.Reader, n PhoneNormalizer, opts ...Option) (*Table, error) {
	if n == nil {
		// Failing here rather than defaulting to "store the raw input" is deliberate:
		// the degraded version of this mistake is a directory that looks populated and
		// never matches a login.
		return nil, fmt.Errorf("person: a PhoneNormalizer is required")
	}
	if err := w.Consume(tableSchema); err != nil {
		return nil, fmt.Errorf("person: create table: %w", err)
	}
	if err := w.Consume(sectionSchema); err != nil {
		return nil, fmt.Errorf("person: create section table: %w", err)
	}

	// Additive drift only. Every column here is also in table.sql — the duplication
	// is deliberate: table.sql creates a correct table on a fresh database, and
	// these calls bring an existing one forward. Dropping or narrowing a column is
	// not expressible this way and needs a real migration.
	//
	// The section columns arrived after the table did (task 079 needed them so the
	// login chooser can tell two crew members on one phone apart), so an existing
	// deployment gets them here rather than by being recreated.
	for _, col := range []struct{ name, ddl string }{
		{"sectionSlug", `sectionSlug VARCHAR(99) NOT NULL DEFAULT ""`},
		{"sectionName", `sectionName VARCHAR(199) NOT NULL DEFAULT ""`},
	} {
		if err := cqrs.EnsureColumn(r, w, "person", col.name, col.ddl); err != nil {
			return nil, fmt.Errorf("person: ensure column %s: %w", col.name, err)
		}
	}
	if err := cqrs.EnsureIndex(r, w, "person", "year_section",
		"ALTER TABLE person ADD INDEX year_section (year, sectionSlug)"); err != nil {
		return nil, fmt.Errorf("person: ensure index year_section: %w", err)
	}

	t := &Table{
		consumer: consumer{w: w, normalizer: n},
		querier:  querier{db: r, normalizer: n},
	}
	for _, opt := range opts {
		opt(t)
	}
	return t, nil
}

// Option configures the projection.
//
// Options exist so the package can report a condition the application cares about
// without importing the application's logger — see the package doc.
type Option func(*Table)

// ReportUnmappedSlug installs a sink for crew section slugs the classifier does not
// recognise.
//
// This is not an error path. Sections are organizer-authored free text validated by
// nothing (PRD 006 §2), so a slug this package has no rule for is expected: the member
// keeps the least-privileged role and the slug is still recorded. But it is exactly the
// signal that says "the classification table is out of date", which is invisible unless
// something says so out loud.
func ReportUnmappedSlug(report func(slug string)) Option {
	return func(t *Table) { t.consumer.unmapped = report }
}

// CreateTableSql exposes the schema, matching the shared-go entities' shape.
func (t *Table) CreateTableSql() string { return tableSchema }
