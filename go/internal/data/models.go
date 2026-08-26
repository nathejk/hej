// Package data is the read-side facade handed to HTTP handlers. As the domain
// grows, SQL-backed read models (one per aggregate) are exposed here so
// handlers never touch SQL directly.
package data

import (
	"nathejk.dk/internal/scans"
	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/checkpoint"
)

// Models is the read-only facade passed to handlers.
type Models struct {
	// Users resolves a normalized phone number to a user + role. The concrete
	// implementation is injected in main.go; today that is a mock directory,
	// later the real Nathejk-records lookup — handlers do not care which.
	Users users.Directory

	// Scans returns a patrol's event registrations (checkpoint scans, bandit
	// catches). Mocked today; a jetstream-fed projection later.
	Scans scans.Source

	// RaceAreas derives the region the client caches map tiles for, from the checkpoint
	// projection (PRD 002 §11.2).
	//
	// **May be nil**, unlike the two above: it needs a database, and running without one is
	// a supported mode (PRD 008 §5). Handlers must check rather than assume — a nil here
	// means "map data unavailable", which is a different answer from "no area derived yet".
	//
	// Typed as checkpoint.Queries rather than a local interface so the return type does not
	// have to be duplicated. Note the interface exposes only the derived *area*, never the
	// checkpoint positions it came from — that boundary is the projection's, not this
	// facade's, so it cannot be widened by accident here.
	RaceAreas checkpoint.Queries
}

// NewModels constructs the read-side facade with the given read sources.
// Additional read models will be wired in here as aggregates are added.
//
// raceAreas may be nil; see the field's doc.
func NewModels(usersDir users.Directory, scanSource scans.Source, raceAreas checkpoint.Queries) Models {
	return Models{Users: usersDir, Scans: scanSource, RaceAreas: raceAreas}
}
