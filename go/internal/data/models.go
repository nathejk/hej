// Package data is the read-side facade handed to HTTP handlers. As the domain
// grows, SQL-backed read models (one per aggregate) are exposed here so
// handlers never touch SQL directly.
package data

import (
	"github.com/nathejk/shared-go/tables/vehicle"

	"nathejk.dk/internal/reveal"
	"nathejk.dk/internal/scans"
	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/album"
	"nathejk.dk/nathejk/table/checkpoint"
	"nathejk.dk/nathejk/table/glimt"
	"nathejk.dk/nathejk/table/patrolphoto"
	"nathejk.dk/nathejk/table/person"
	"nathejk.dk/nathejk/table/photo"
	"nathejk.dk/nathejk/table/publicpatrol"
	"nathejk.dk/nathejk/table/year"
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

	// Glimt is the Glimt read model (PRD 019): the feed, one hold's collection, the moderation
	// queue and the public feed.
	//
	// **May be nil**, for the same reason as Maps, and with the same obligation on handlers: a nil
	// means "the feature is unavailable", which must be reported as such rather than as an empty
	// feed. "Nobody shared anything tonight" is a legitimate state that the client caches, so
	// serving it because the database is down would leave a device showing an empty feed for the
	// rest of the event.
	//
	// Note what this facade deliberately cannot do: it holds only the *read* API, so nothing
	// reachable through it can publish a glimt event or reach the blob store. Creating and hiding
	// go through internal/commands, and the bytes through app.blobs.
	Glimt glimt.Queries

	// Albums is the curated photo albums shown on the public frontpage (PRD 011 §6 section 1).
	//
	// **May be nil**, like Glimt and Maps, and the handling differs from theirs in one place that
	// matters. A public page with no albums simply shows its empty state — fine, and indistinguishable
	// from "nobody has curated anything yet", which is the honest reading either way.
	//
	// But `RefsInUse` is also consulted by the **glimt delete path**, where nil and error are not
	// interchangeable: nil means there are no album items to protect, while an error means we cannot
	// tell — and deleting content-addressed bytes on "cannot tell" is how a takedown in one feature
	// blanks a page in another. See cmd/api/glimtdelete.go's refsUsedElsewhere.
	//
	// Read-only here by construction, like Glimt: creating and removing go through internal/commands,
	// and the bytes through app.blobs.
	Albums album.Queries

	// PublicPatrols is what a patrol's public page may say about it (PRD 011 §8).
	//
	// **May be nil**, like the projections above. A nil must be treated as "closed", not as "unavailable":
	// the patrol page's refusal has to be indistinguishable from a patrol that has not finished and from
	// one that does not exist (PRD 011 §6), and a 503 here would confirm that a number is real.
	//
	// Note what the *type* guarantees rather than the handler: `publicpatrol.Patrol` carries a number, a
	// patrol name, a group and a korps, and has no field for a person. The upstream event this is folded
	// from carries a leader's name, phone and email — they reach no column. See the package doc.
	PublicPatrols publicpatrol.Queries

	// Years is what this app knows about the event year: where the walk starts and ends (task 357).
	//
	// **May be nil**, and a nil means "no route line" rather than an error. The only caller is the diploma,
	// which omits the line when there is nothing to print — so an absent projection degrades to exactly the
	// behaviour that shipped before this table existed.
	Years year.Queries

	// PatrolPhotos is a patrol's photographs and the one chosen to represent it (task 361).
	//
	// **May be nil**, and a nil means "no photograph" rather than an error: the diploma omits the picture and
	// renders, which is also what happens for a patrol nobody photographed.
	PatrolPhotos patrolphoto.Queries

	// Photos is the year's photograph library — "the bulk" the photographers hand in (PRD 022 §8.3).
	//
	// **May be nil**, and the handling matches Albums' rather than Glimt's, for the same reason plus one
	// that is specific to this field.
	//
	// Albums' reason: `RefsInUse` is consulted by delete paths in *other* features, where nil and error are
	// emphatically not interchangeable — nil means there are no library photographs to protect, while an
	// error means we cannot tell, and deleting content-addressed bytes on "cannot tell" is how a takedown in
	// one feature blanks a page in another. See cmd/api/glimtdelete.go and task 368.
	//
	// Its own reason: **no public page reads this**. A photograph reaches the open web only by being
	// referenced from a published album, which resolves through Albums. So a nil here cannot degrade a
	// public surface at all — it can only take the curator's tool away, which is the safe direction.
	//
	// Read-only here by construction, like Glimt and Albums: uploading, editing and deleting go through
	// internal/commands, and the bytes through app.blobs.
	Photos photo.Queries

	// AlbumCurator and PhotoCurator are the **admin tool's** reads: drafts, removals, and photographs no
	// album references (PRD 022 §8.8).
	//
	// # Why these are separate fields rather than flags on the two above
	//
	// Because a public handler must be *structurally unable* to serve a draft album, not merely unlikely to.
	// `Albums` is publication-filtered in SQL and `BySlug` returns one "not found" for unknown, unpublished
	// and deleted so the open web cannot enumerate drafts. An `includeUnpublished bool` on those reads would
	// have been cheaper and would have put every public handler one argument away from defeating that.
	//
	// The rule this follows is `publicpatrol`'s: *"It is not here" is a property; "we do not select it" is a
	// habit.* A handler that never receives these fields cannot misuse them, and the only code that holds
	// them is behind the admin credential.
	//
	// **May be nil**, and a nil here is the safest of all the projections: nothing public reads them, so a
	// nil takes the curator's tool away (a 503 on the admin surface) and cannot affect a single public page.
	//
	// Read-only here, like the rest: creating, editing and deleting go through internal/commands.
	AlbumCurator album.CuratorQueries
	PhotoCurator photo.CuratorQueries
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

// WithGlimt supplies the Glimt read model (PRD 019). Omit it and Models.Glimt is nil, which handlers
// must treat as "the feature is unavailable" rather than as an empty feed — an empty feed is a
// legitimate state a client will cache, and caching "nothing was shared tonight" because the
// database is down is worse than an honest error.
func WithGlimt(q glimt.Queries) Option {
	return func(mo *Models) { mo.Glimt = q }
}

// WithAlbums supplies the album read model (PRD 011). Omit it and Models.Albums is nil.
//
// Unlike WithGlimt, a nil here is benign for the pages that read it — an empty albums section is the
// truth when there are none. The field's doc records the one caller for which nil and error must stay
// distinct.
func WithAlbums(q album.Queries) Option {
	return func(mo *Models) { mo.Albums = q }
}

// WithPhotos supplies the photograph library read model (PRD 022). Omit it and Models.Photos is nil.
//
// A nil takes the curator's tool away and cannot affect a public page, because no public read goes through
// this projection. The field's doc records the one caller for which nil and error must stay distinct.
func WithPhotos(q photo.Queries) Option {
	return func(mo *Models) { mo.Photos = q }
}

// WithCuratorReads supplies the admin tool's draft-visible reads (PRD 022 §8.8). Omit it and both
// Models.AlbumCurator and Models.PhotoCurator are nil, which the admin handlers must treat as a 503.
//
// One option supplying both, because they are always wanted together — the tool needs albums and
// photographs to do anything at all — and because a single call site makes the boundary easy to audit:
// grep for this function and you have found everywhere curator reads enter the application.
func WithCuratorReads(a album.CuratorQueries, p photo.CuratorQueries) Option {
	return func(mo *Models) {
		mo.AlbumCurator = a
		mo.PhotoCurator = p
	}
}

// WithPublicPatrols supplies the public patrol read model (PRD 011). Omit it and Models.PublicPatrols is
// nil, which handlers must treat as "closed" rather than "unavailable" — see the field's doc.
func WithPublicPatrols(q publicpatrol.Queries) Option {
	return func(mo *Models) { mo.PublicPatrols = q }
}

// WithYears supplies the event-year read model (task 357). Omit it and Models.Years is nil, which means the
// diploma prints no route line — the behaviour before the projection existed.
func WithYears(q year.Queries) Option {
	return func(mo *Models) { mo.Years = q }
}

// WithPatrolPhotos supplies the patrol photograph read model (task 361). Omit it and Models.PatrolPhotos is nil,
// which means no diploma carries a photograph.
func WithPatrolPhotos(q patrolphoto.Queries) Option {
	return func(mo *Models) { mo.PatrolPhotos = q }
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
