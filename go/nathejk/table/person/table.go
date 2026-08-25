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
func New(_ cqrs.Publisher, w cqrs.Writer, r cqrs.Reader, n PhoneNormalizer) (*Table, error) {
	if n == nil {
		// Failing here rather than defaulting to "store the raw input" is deliberate:
		// the degraded version of this mistake is a directory that looks populated and
		// never matches a login.
		return nil, fmt.Errorf("person: a PhoneNormalizer is required")
	}
	if err := w.Consume(tableSchema); err != nil {
		return nil, fmt.Errorf("person: create table: %w", err)
	}

	// Additive drift only. Every column here is also in table.sql — the duplication
	// is deliberate: table.sql creates a correct table on a fresh database, and
	// these calls bring an existing one forward. Dropping or narrowing a column is
	// not expressible this way and needs a real migration.
	//
	// Nothing to ensure yet: the table was introduced whole. Columns added in later
	// tasks go here, e.g.
	//
	//	if err := cqrs.EnsureColumn(r, w, "person", "portraitRef",
	//		`portraitRef VARCHAR(64) NOT NULL DEFAULT ""`); err != nil { ... }

	return &Table{
		consumer: consumer{w: w, normalizer: n},
		querier:  querier{db: r, normalizer: n},
	}, nil
}

// CreateTableSql exposes the schema, matching the shared-go entities' shape.
func (t *Table) CreateTableSql() string { return tableSchema }
