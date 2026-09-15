// Package data is the read-side facade handed to HTTP handlers. As the domain
// grows, SQL-backed read models (one per aggregate) are exposed here so
// handlers never touch SQL directly.
package data

import (
	"github.com/nathejk/shared-go/tables/vehicle"

	"nathejk.dk/internal/reveal"
	"nathejk.dk/internal/scans"
	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/checkpoint"
	"nathejk.dk/nathejk/table/person"
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
	// Typed as checkpoint.AreaQueries rather than the projection's whole read API: this field exists
	// for the race area, and the handler that uses it has no business being able to read checkpoint
	// positions. Narrowed in task 255, when the projection gained the two bounded reads the reveal
	// rule needs — keeping this field wide would have handed every handler a capability one of them
	// wanted.
	//
	// Note the interface exposes only the derived *area*, never the checkpoint positions it came
	// from — that boundary is the projection's, not this facade's, so it cannot be widened by
	// accident here.
	RaceAreas checkpoint.AreaQueries

	// People is the person projection's read API, for the one thing `Users` cannot
	// express: a field that is not part of "who is this and what do they do".
	//
	// It exists for the portrait (task 105), which needs `portraitRef` for the caller
	// alone. Deliberately NOT added to `users.User`: that struct is handed to the login
	// chooser, which shows one holder of a shared phone number something about the
	// others, so every field on it is a field that has to be safe to show a stranger.
	//
	// **May be nil**, like RaceAreas and for the same reason: it needs a database, and
	// running without one is a supported mode (PRD 008 §5). Handlers must check.
	People person.Queries

	// Vehicles is shared-go's vehicle projection (PRD 010): the cars and trailers
	// associated with the race.
	//
	// **May be nil**, like RaceAreas and People, and the distinction matters more here
	// than for either of them. A nil means "we cannot tell what is registered", which a
	// handler must not report as "you have nothing registered" — that answer invites a
	// member to register a second row for a car that is already in the inventory, which is
	// the exact duplicate the coordinator then has to reconcile by hand (PRD 010 §5).
	//
	// Read-only here by construction: the write side is the entity's own `Commands`,
	// held separately on the application, so nothing reachable through this facade can
	// publish a vehicle event.
	Vehicles vehicle.Queries

	// Maps answers the two patrol-scoped map questions: which checkpoints this patrol has earned sight
	// of, and which map sheets it has been handed (PRD 016).
	//
	// **May be nil**, like RaceAreas and for the same reason: it needs a database and the projections
	// behind it. A nil means "we cannot tell what this patrol may see", which a handler must report as
	// unavailable rather than as "nothing" — an empty reveal is a legitimate state early in the race, and
	// the client caches it offline, so conflating the two would leave a patrol holding a cached empty map
	// for the rest of the night.
	//
	// Typed as an interface declared here rather than as the concrete rule, so this facade keeps stating
	// what handlers may ask rather than what the implementation happens to offer.
	Maps MapReads
}

// MapReads is the patrol-scoped map read API.
//
// Both methods take the patrol id, and that is the whole shape of the thing: there is no "all
// checkpoints" and no "all handouts" to ask for. A handler cannot widen the question, which is where the
// reveal rule's guarantee becomes unavoidable rather than merely intended (PRD 016 §6).
type MapReads interface {
	// Revealed returns the checkpoints this patrol may see and the line it is heading for. Empty is normal.
	//
	// `hasStarted` is passed in because "has begun the event" is a fact about a person and has exactly one
	// definition (`person.HasStarted`); the map read is patrol-scoped and must not grow a second one.
	Revealed(year string, patrolID string, hasStarted bool) (reveal.RevealedMap, error)
	// Handouts returns the map sheets this patrol has been given, oldest first. Empty is normal.
	Handouts(year string, patrolID string) ([]reveal.Handout, error)
}

// Option configures optional read models.
//
// Options rather than more constructor parameters, following the projections' own shape
// (checkpoint.Option, kort.Option). NewModels already takes five sources, three of which may be nil, and
// a sixth positional nil would make every one of its seventeen call sites read a little worse while
// telling the reader nothing. An option names what it supplies.
type Option func(*Models)

// WithMapReads supplies the patrol-scoped map reads. Omit it and Models.Maps is nil, which handlers must
// treat as "map data unavailable".
func WithMapReads(m MapReads) Option {
	return func(mo *Models) { mo.Maps = m }
}

// NewModels constructs the read-side facade with the given read sources.
// Additional read models will be wired in here as aggregates are added.
//
// raceAreas, people and vehicles may be nil; see the fields' docs.
func NewModels(
	usersDir users.Directory,
	scanSource scans.Source,
	raceAreas checkpoint.AreaQueries,
	people person.Queries,
	vehicles vehicle.Queries,
	opts ...Option,
) Models {
	m := Models{
		Users:     usersDir,
		Scans:     scanSource,
		RaceAreas: raceAreas,
		People:    people,
		Vehicles:  vehicles,
	}
	for _, opt := range opts {
		opt(&m)
	}
	return m
}
