package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"sync"
	"time"

	// Embed the timezone database in the binary. The prod image is bare alpine with
	// no system tzdata, and the person projection needs Europe/Copenhagen to convert
	// upstream birthdays (stored as midnight-local expressed in UTC) to the right
	// calendar date. Without this the conversion silently falls back to UTC and every
	// such birthday lands a day early.
	_ "time/tzdata"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/tables/vehicle"

	bff "nathejk.dk/cmd/api/app"
	"nathejk.dk/internal/blob"
	"nathejk.dk/internal/choice"
	"nathejk.dk/internal/commands"
	"nathejk.dk/internal/data"
	"nathejk.dk/internal/photobytes"
	"nathejk.dk/internal/pin"
	"nathejk.dk/internal/publicgate"
	"nathejk.dk/internal/push"
	"nathejk.dk/internal/ratelimit"
	"nathejk.dk/internal/reveal"
	"nathejk.dk/internal/session"
	"nathejk.dk/internal/sms"
	"nathejk.dk/internal/users"
	"nathejk.dk/internal/vcs"
	"nathejk.dk/nathejk/table/album"
	"nathejk.dk/nathejk/table/checkgroup"
	"nathejk.dk/nathejk/table/checkpoint"
	"nathejk.dk/nathejk/table/glimt"
	"nathejk.dk/nathejk/table/kort"
	"nathejk.dk/nathejk/table/maphandout"
	"nathejk.dk/nathejk/table/pagereport"
	"nathejk.dk/nathejk/table/patrolphoto"
	"nathejk.dk/nathejk/table/person"
	"nathejk.dk/nathejk/table/photo"
	"nathejk.dk/nathejk/table/publicpatrol"
	"nathejk.dk/nathejk/table/scan"
	"nathejk.dk/nathejk/table/trackpoint"
	"nathejk.dk/nathejk/table/year"
)

// application is the root dependency container for the API binary. It embeds
// bff.JsonApi to inherit the transport-layer helpers (WriteJSON, ReadJSON,
// error responses, Serve) so handlers can call them via the `app` receiver.
type application struct {
	bff.JsonApi
	config config
	// workableYears are the years the admin tool can work in, read once when the routes are built (task 392).
	// See adminyear.go.
	workableYears []string
	models        data.Models
	commands      commands.Commands

	// vehicles is the vehicle entity's command side (PRD 010), or nil when there is
	// no database or no broker.
	//
	// Separate from `commands` above rather than folded into it, which is what
	// go-bff-layout describes as the mature shape: the entity owns its subject
	// vocabulary and its delta pruning (`UpdateFields.diff`), so a handler reaching
	// for the generic publisher would end up re-implementing both — and would be able
	// to publish a vehicle event this entity would never emit.
	vehicles vehicle.Commands

	// publicGate decides when a patrol's public page may exist (PRD 011 §0b.3, task 330).
	//
	// **May be nil**, like the other projection-backed reads, and the handling differs from theirs in a
	// way worth stating: a nil here fails *closed* rather than answering 503. The other nil-projection
	// paths tell an authenticated member "we cannot serve this right now", which is the honest and
	// recoverable answer. This one faces the open web, where a distinguishable "exists but unavailable"
	// confirms a patrol number is real and that the race is still running — see publicgate.go.
	publicGate *publicgate.Gate

	// photos fetches a patrol photograph's bytes into this app's own blob store (task 361).
	//
	// **May be nil**, when `PHOTO_BASEURL` is unset or there is no blob store. A nil means diplomas carry no
	// photograph, which is the same outcome as a patrol nobody photographed — so no caller has to distinguish
	// them, and an unconfigured environment renders certificates rather than failing them.
	photos *photobytes.Fetcher

	// patrolTracks composes a patrol's merged, unattributed route and caches it (PRD 011 §6, task 340).
	//
	// **May be nil**, when either the person or the telemetry projection is missing. A nil means "we
	// cannot tell what was recorded", which is not the same as "nothing was recorded" — and since an empty
	// track is the *common* case (task 082 measured 2% coverage), conflating them would hide an outage
	// behind a legitimate state forever.
	patrolTracks *patrolTrackReader

	// Auth infrastructure.
	pins              *pin.Store
	sms               sms.Sender
	sessions          *session.Manager
	requestPinLimiter *ratelimit.Limiter
	// trackLimiter throttles position-batch uploads, keyed by user rather than by IP
	// (task 084). Participants share networks — a patrol on one phone's hotspot, a klan
	// behind one carrier NAT — so an IP limit would punish groups while still letting a
	// single runaway client flood the broker.
	trackLimiter *ratelimit.Limiter
	// photoLimiter throttles portrait uploads, keyed by user for the same reason as
	// trackLimiter: participants share networks, so an IP limit would throttle a whole
	// patrol because one member is retrying.
	//
	// Unlike the others this protects *work* rather than a broker: each upload decodes up
	// to ~20 MP, resamples it several times, and writes two or three objects into the one
	// directory that must be backed up. Taking a portrait is a once-or-twice-per-event
	// action, so a low ceiling costs nobody anything.
	photoLimiter *ratelimit.Limiter
	// glimtMediaLimiter throttles Glimt media uploads, keyed by user like the two above.
	//
	// Separate from photoLimiter, and much more generous, because the actions are nothing
	// alike: a portrait is taken once or twice an event, while a glimt carries up to ten items
	// and a member may post several during a night. A shared ceiling would mean two posts
	// exhausting someone's portrait budget, or a portrait limit loose enough to be pointless.
	//
	// May be nil, in which case no limit applies — which is what the test harness and a
	// zero-config run get. The handler checks.
	glimtMediaLimiter *ratelimit.Limiter
	// glimtLimiter throttles glimt *creation*, separately from the media uploads each one
	// carries. Two limiters because they protect different things: the media limiter protects
	// CPU and disk, this one protects the broker from a looping client publishing events.
	//
	// May be nil, in which case no limit applies.
	glimtLimiter *ratelimit.Limiter
	// glimtReportLimiter throttles reports, deliberately much more loosely than creation.
	//
	// The asymmetry is the design: a spurious report costs a moderator a glance, while a
	// throttled one costs a photograph somebody objected to staying up. The limit exists only so
	// the endpoint cannot be hammered, not to ration reporting.
	//
	// May be nil, in which case no limit applies.
	glimtReportLimiter *ratelimit.Limiter
	// glimtMediaBudget is the *byte* half of the upload limit (task 311), keyed by member like
	// glimtMediaLimiter and checked next to it.
	//
	// Two limiters on one endpoint because a count and a size answer different questions: sixty
	// thumbnails and sixty 12 MiB videos are the same number of events and nowhere near the same
	// cost, while a byte cap on its own would let a client hammer the decode path with tiny
	// images. Each lets through precisely what the other exists to stop.
	//
	// May be nil, and a nil Budget allows everything — so the handler needs no nil check here.
	glimtMediaBudget *ratelimit.Budget
	// glimtReadLimiter throttles the Glimt *read* endpoints, deliberately two orders of magnitude
	// looser than the write limiters and measured per minute rather than per hour.
	//
	// This separation is the point rather than an optimisation (PRD 019 §0a.3, task 311). The
	// post-race browse is a **legitimate flood** — a thousand people at the finish line, each
	// pulling a grid of thumbnails per screen — and it is the use the whole feature was built
	// for. A read limit anywhere near the upload numbers would throttle exactly that, and an
	// hourly budget would be spent by somebody scrolling for two minutes and then locked out for
	// fifty-eight.
	//
	// So it exists to stop a script hammering the endpoint and for nothing else. If it ever fires
	// for a real member, it is set wrong.
	//
	// May be nil, in which case no limit applies.
	glimtReadLimiter *ratelimit.Limiter
	// publicGlimtReadLimiter and publicReportLimiter throttle the **unauthenticated** public page
	// and its API (task 323), keyed by **IP** — the only key available, since there is no member.
	//
	// Separate from the member-keyed limiters rather than shared, because an IP is a much worse key:
	// a school, a workplace or a parents' group behind one NAT shares a single budget. That forces
	// the read ceiling to be correspondingly generous, and mixing the two would drag the
	// member-keyed limit down to match.
	//
	// The report limit is tighter than the authenticated one — an anonymous write should be — but
	// still generous, for the reason glimtreport.go records: the cost of a spurious report is a
	// moderator's glance, and the cost of a throttled one is a photograph somebody objected to
	// staying on the open web.
	//
	// Both may be nil, in which case no limit applies.
	publicGlimtReadLimiter *ratelimit.Limiter
	publicReportLimiter    *ratelimit.Limiter
	// publicMediaReadLimiter throttles the public **media** routes — album photographs and public glimt
	// thumbnails — also by IP, and on its own budget (task 347).
	//
	// # Why media does not share the page budget
	//
	// Because the two differ by an order of magnitude in count and by more than that in cost. Opening one
	// album page is 1 HTML request and up to 60 thumbnail requests; the HTML does database reads while a
	// thumbnail is a blob read with an ETag and a year-long cache. Sharing one budget therefore means the
	// **cheap and numerous starve the expensive and few** — a visitor scrolling two albums could spend the
	// allowance their next page load needs, and the failure would look like the site being broken rather
	// than like a limit being hit.
	//
	// Generous for the same NAT reason as the others, and more so because of the arithmetic above: see
	// env.go for the numbers.
	//
	// May be nil, in which case no limit applies.
	publicMediaReadLimiter *ratelimit.Limiter
	// The diploma thumbnail, rendered once from the artwork embedded in the binary (task 345).
	//
	// Held on the application rather than in a package-level variable so a test gets a fresh one, and so the
	// error is remembered too: a broken asset should not be retried on every request of a burst.
	diplomaThumbOnce  sync.Once
	diplomaThumbBytes []byte
	diplomaThumbErr   error
	// confirmLimiter throttles the guardian-number confirmation and report endpoints
	// (PRD 005, tasks 135/136), keyed by IP like the PIN limiter.
	//
	// Explicitly **not** a secrecy measure: the digits it protects are not a secret —
	// /api/me/profile returns the whole number to its owner by design. It exists so the
	// endpoint cannot be hammered, which is a different and much smaller job.
	//
	// Keyed by IP rather than by user, unlike the track and photo limiters: a member
	// confirms once, so there is no legitimate per-user burst to accommodate, and the thing
	// worth blunting is one client looping — not one member retrying twice.
	confirmLimiter *ratelimit.Limiter
	// contactChecks counts failed recall attempts per login session, so a member who cannot
	// remember their contact number is let out of the check after three tries instead of being
	// stuck in it (PRD 015, task 227). Also the idempotency guard for the give-up outcome.
	//
	// Distinct from confirmLimiter and must stay that way: that one blunts a hammering client
	// by IP, this one is a product rule about one member's attempts. A whole patrol shares one
	// campsite wifi, so tightening the IP limiter to enforce a per-member rule would lock out
	// the members who did nothing wrong.
	contactChecks *contactCheck
	// choices issues the short-lived token that carries a user from "PIN verified" to
	// "which of you is this?" when a phone number is shared (task 079).
	choices *choice.Manager

	// Push subscription storage.
	pushStore push.Store

	// db is the MariaDB pool, or nil when no DSN is configured. Handlers must not
	// use it directly — reads go through models, writes through commands (see
	// go-bff-layout). It is held here so shutdown can close it and the healthcheck
	// can report on it.
	db *sql.DB

	// eventing is the CQRS seam (reader/writer/publisher + projection mux), or nil
	// when there is no database. Held for the healthcheck and dead-letter
	// reporting; handlers still go through models and commands.
	eventing *eventing

	// blobs stores binary objects that cannot be rebuilt from the event log —
	// portrait bytes (PRDs 003/007). Never nil: it falls back to memory.
	blobs blob.Store

	// contactsVersions caches the contacts directory version per permitted role set, for a
	// few seconds (PRD 007's freshness poll, task 155).
	//
	// Held on the application rather than computed per request because this is the first
	// endpoint the app polls continuously during the race: every device with the pane open
	// asks every ~60 s, and the answer is identical for everyone with the same permitted
	// set. Nil is safe — the cache degrades to computing every time.
	contactsVersions *versionCache

	// checkpointsVersions, handoutsVersions and scansVersions cache the map datasets' versions per
	// patrol, for the same reason and on the same terms as contactsVersions (task 269, feeding PRD 017's
	// foreground-sync check). Separate caches rather than one keyed by dataset so each can take its own
	// TTL later — contacts.go's reference warns that two datasets on one number cannot be tuned apart,
	// and these will not change at the same rate. Nil is safe: each degrades to computing every time.
	checkpointsVersions *versionCache
	handoutsVersions    *versionCache
	scansVersions       *versionCache

	// profileVersions and raceAreaVersions cache the remaining two sync datasets (task 283). Their
	// keys sit at the two extremes of the same rule: profile is keyed by *user*, because its permitted
	// set is one person and there is nothing to share; race area is keyed by *event year*, because
	// every device in the event holds the identical hull and keying it per user would multiply one
	// answer by the device count. Nil is safe for both.
	profileVersions  *versionCache
	raceAreaVersions *versionCache

	// syncMetrics accumulates the freshness check's own numbers (task 293): the aggregate unchanged
	// ratio, and per-dataset version churn from a bounded witness sample. Nil is safe — every method on
	// it tolerates a nil receiver — so tests need not wire it.
	syncMetrics *syncMetrics
}

// @title        Hej Nathejk API
// @version      0.1.0
// @description  Backend-for-frontend API for the Hej Nathejk event app.
// @BasePath     /api
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// run() owns everything that needs cleanup, because os.Exit skips deferred
	// calls — closing the database pool has to happen before we exit, not after.
	if err := run(logger); err != nil {
		logger.Error("server error", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg := loadConfig()

	// The database is optional today: every read model is still a mock, so a
	// missing or unreachable DSN degrades to the previous behaviour instead of
	// preventing startup. Once PRD 006's projection lands on the login path this
	// becomes a real dependency — the healthcheck (task 059) is what makes the
	// difference visible rather than silent.
	db, err := openDB(cfg, logger)
	switch {
	case errors.Is(err, ErrNoDSN):
		logger.Warn("no DB_DSN configured, running without a database")
	case err != nil:
		logger.Error("database unavailable, continuing without it", "err", err)
		// db is nil here; openDB closed the pool it opened.
	default:
		logger.Info("database connected",
			"max_open_conns", cfg.dbMaxOpenConns,
			"conn_max_lifetime", cfg.dbConnMaxLifetime.String(),
		)
		defer func() {
			if cerr := db.Close(); cerr != nil {
				logger.Error("closing database", "err", cerr)
			}
		}()
	}

	// The member directory. Starts on the mock only so that a run with no database at
	// all is still usable; anything with a database moves to the real projection below,
	// broker or no broker.
	directory := newSwitchableDirectory(users.NewMockDirectory())

	// The CQRS seam. Both degraded modes are deliberate and distinct:
	//
	//   no database — nothing to project into, so no eventing at all
	//   no broker   — reader/writer still usable, so handlers keep serving
	//                 whatever the last run projected (PRD 008 §5)
	ev, err := openEventing(cfg, db, logger)
	noBroker := errors.Is(err, ErrNoJetstreamDSN)
	switch {
	case errors.Is(err, ErrNoDSN):
		logger.Warn("no database: skipping event stream wiring")
	case noBroker:
		logger.Warn("no JETSTREAM_DSN configured: running without a broker, reads served from existing projections")
	case err != nil:
		logger.Error("event stream setup failed, continuing without it", "err", err)
	}

	// The person projection (PRD 006).
	//
	// Constructed here — as soon as there is a *database* — rather than inside the
	// broker's connect callback, which is where it used to live. That was a real bug, and
	// the reasoning that put it there ("only create tables once there is a broker, so a
	// database-only run does not create tables nothing will fill") turned out to be worth
	// far less than what it cost:
	//
	// With the broker unreachable, the callback never ran, so the directory stayed on the
	// mock — while a fully populated `person` table sat in the database. Verified: a mock
	// number was issued a PIN and a real member was not. In production that inverts the
	// two things that matter. Real participants cannot log in during a broker outage even
	// though their data is present and needs no broker to read, and the mock's phone
	// numbers *can*, which is a set of usable fake accounts appearing precisely when
	// nobody is watching closely. It also directly contradicted PRD 008 §5, which promises
	// reads are served from existing projections when the broker is down.
	//
	// Splitting it this way follows the actual dependencies: the read path needs only the
	// database, and the broker decides only whether the projection keeps *updating*.
	// Creating an empty table on a fresh database is a trivial cost by comparison.
	var persons *person.Table
	if ev != nil && (err == nil || noBroker) {
		// phoneNormalizer adapts internal/phone to the interface the projection
		// declares. Passing the same implementation the login handler uses is the
		// point: a second implementation, or the same rules re-typed, would make
		// lookups silently miss (PRD 006 §2).
		p, perr := person.New(ev.publisherOrNil(), ev.writer, ev.reader, phoneNormalizer{},
			// Warn, not error: an unrecognised section slug is a routine data
			// condition (organizers rename sections and nothing validates the
			// values), but it means a crew member is stuck on the generic role, so
			// it has to be visible somewhere other than a SQL query.
			person.ReportUnmappedSlug(func(slug string) {
				logger.Warn("unmapped crew section slug", "slug", slug)
			}),
			// A number that arrived and could not be used. Warn rather than error for
			// the same reason — it is upstream data, not a fault here — but it must
			// not be silent: for `phoneParent` it means an emergency contact the app
			// believes does not exist. Only a digit count is logged, never the number.
			person.ReportUnusablePhone(func(personID, field string, digits int) {
				logger.Warn("unusable phone number",
					"personId", personID, "field", field, "digits", digits)
			}),
		)
		if perr != nil {
			// Not fatal: the API still serves reads from the mock directory. A
			// schema failure here is a bug to fix, not a reason to take the app
			// down mid-event.
			logger.Error("person projection unavailable", "err", perr)
		} else {
			persons = p
			// Step 3 of the three-way registration: expose the projection's read API.
			//
			// Installed before any replay has run, and that is safe for a specific
			// reason: nothing truncates the person table on boot. A restart serves the
			// *previous* run's rows while the replay re-upserts them, so there is no
			// window in which the directory is empty and a member is wrongly told their
			// number is unknown.
			//
			// If a future change ever does truncate on boot, this must move behind a
			// caught-up signal — the stream library has one (CatchupListener).
			directory.set(newPersonDirectory(persons, cfg.eventYear, logger))
			// Reports whether a broker was *configured*, not whether the projection is
			// live: the connection is attempted in the background and has not been made
			// yet at this point. An earlier version of this line said "live", which read
			// as true while the broker was demonstrably unreachable.
			logger.Info("member directory reading the person projection",
				"year", cfg.eventYear, "broker_configured", !noBroker)
		}
	}

	// The checkpoint projection (PRD 002 §11.2), which the race area is derived from.
	//
	// Constructed alongside the person projection and for the same reason: the read path needs
	// only a database, so tying it to the broker's arrival would leave the map unable to name
	// its own race area during a broker outage (see fix(058)).
	var checkpoints *checkpoint.Table
	if ev != nil && (err == nil || noBroker) {
		c, cerr := checkpoint.New(ev.publisherOrNil(), ev.writer, ev.reader,
			// Reported in aggregate when the area is computed, not per checkpoint.
			// Individual gaps are expected — organizers add posts before siting them — so
			// the signal worth having is the systematic case: a year where the field stops
			// being filled in, leaving an area derived from two points.
			checkpoint.ReportPositionless(func(year string, positionless, total int) {
				if positionless == 0 {
					return
				}
				logger.Warn("checkpoints without a position",
					"year", year, "positionless", positionless, "total", total)
			}),
		)
		if cerr != nil {
			// Not fatal: the map still works, it just cannot scope an offline tile cache.
			logger.Error("checkpoint projection unavailable", "err", cerr)
		} else {
			checkpoints = c
		}
	}

	// The map projections (PRD 016): the printed sheets and sets, the handout history, the checkgroups,
	// and the scans with the personnel shifts that place them.
	//
	// Same construction condition as the two projections above, for the same reason: every read here
	// needs only a database, so tying them to the broker's arrival would leave a patrol unable to see
	// its own map page during a broker outage — which is exactly when it would be reaching for it.
	//
	// Each failure is logged and non-fatal, and each degrades to a specific absence rather than a broken
	// page: no sheets means no handout list, no scans means no registrations.
	var sheets *kort.Table
	var handouts *maphandout.Table
	var checkgroups *checkgroup.Table
	var scanProjection *scan.Table
	// glimts is the Glimt projection (PRD 019). Declared with the others because it shares
	// their construction condition, but note it is the only one whose blobs are not
	// rebuildable from the stream — the rows here replay, the media in the blob store do not.
	var glimts *glimt.Table
	// albums is the curated-album projection (PRD 011, task 333). Shares glimt's construction
	// condition and its one caveat: the rows replay from the stream, the media in the blob store do
	// not. It also shares glimt's *objects* — content addressing means an album photograph and a
	// glimt can be the same bytes — which is why the delete path consults both (glimtdelete.go).
	var albums *album.Table
	// photos is the photograph library — "the bulk" (PRD 022, task 363). The album projection above holds
	// the *arrangements*; this one holds the photographs they arrange. Same construction condition and the
	// same non-rebuildable caveat, and it shares the other two's objects for the same reason: content
	// addressing means a library photograph, an album's photograph and a glimt can be one set of bytes.
	var photos *photo.Table
	// publicPatrols is the narrow public patrol projection (PRD 011, task 338). Folds the same upstream
	// team events shared-go's `patrulje` projection folds, and writes four columns — deliberately dropping
	// the contact block, so the public page cannot name a person even by accident. See the package doc for
	// why that is worth a second consumer.
	var publicPatrols *publicpatrol.Table
	// years is the event-year projection, copied from hq and narrowed to the two cities (task 357). It exists
	// for one line on a diploma — "fra Lundby til Glumsø" — which had no source in this repo before it.
	var years *year.Table
	// patrolPhotos is the patrol photograph projection, copied from hq and narrowed (task 361). Metadata only:
	// the bytes live in foto and are fetched into this app's blob store on demand — see internal/photobytes.
	var patrolPhotos *patrolphoto.Table
	// trackPoints is the recorded position points (PRD 011, task 340). The first reader the TELEMETRY
	// stream has ever had — it has been published to since task 084 and consumed by nothing.
	var trackPoints *trackpoint.Table
	// pageReports is the takedown reports filed from the public pages (PRD 011, task 343). Write-only from
	// this app's point of view — there is no in-app moderation surface, so the projection exists to make
	// the footer's "skriv til os" promise land somewhere an organizer can read out of band.
	var pageReports *pagereport.Table
	// consent is the photo-refusal reaction (task 397) — see photoconsent.go. Nil without a broker.
	var consent *consentReactor
	if ev != nil && (err == nil || noBroker) {
		if t, cerr := kort.New(ev.publisherOrNil(), ev.writer, ev.reader,
			// A body we cannot decode is the one signal that our mirrored copy of hq's event shapes
			// has drifted from the contract (PRD 016 §11.11). Logged rather than swallowed, because
			// the alternative is a sheet quietly missing from a patrol's map.
			kort.ReportUnknownBody(func(subject string, derr error) {
				logger.Warn("kort event could not be decoded; the vendored contract may be stale",
					"subject", subject, "err", derr)
			}),
			// Whether this year's sheets can reach a patrol at all. See mapreadiness.go: every part
			// of this feature degrades to "nothing to show", which is also a legitimate state, so the
			// aggregate is the only thing that distinguishes a quiet event from a misconfigured one.
			kort.ReportCounts(mapCountsReporter(logger)),
		); cerr != nil {
			logger.Error("kort projection unavailable", "err", cerr)
		} else {
			sheets = t
		}

		if t, cerr := maphandout.New(ev.publisherOrNil(), ev.writer, ev.reader); cerr != nil {
			logger.Error("maphandout projection unavailable", "err", cerr)
		} else {
			handouts = t
		}

		if t, cerr := checkgroup.New(ev.publisherOrNil(), ev.writer, ev.reader); cerr != nil {
			logger.Error("checkgroup projection unavailable", "err", cerr)
		} else {
			checkgroups = t
		}

		if t, cerr := scan.New(ev.publisherOrNil(), ev.writer, ev.reader); cerr != nil {
			logger.Error("scan projection unavailable", "err", cerr)
		} else {
			scanProjection = t
		}

		// Glimt (PRD 019). Same construction condition as the projections above, and for
		// the same reason: reading the feed needs only the database, while the broker
		// decides whether it keeps *updating*. A degraded Glimt is a feed that stops
		// growing, which is visibly stale rather than wrong.
		if t, cerr := glimt.New(ev.publisherOrNil(), ev.writer, ev.reader); cerr != nil {
			logger.Error("glimt projection unavailable", "err", cerr)
		} else {
			glimts = t
		}

		// Albums (PRD 011). Same condition again: the public pages need only the database, and the
		// broker decides whether new albums appear.
		if t, cerr := album.New(ev.publisherOrNil(), ev.writer, ev.reader); cerr != nil {
			logger.Error("album projection unavailable", "err", cerr)
		} else {
			albums = t
		}

		// The photograph library (PRD 022). Same condition once more, with one difference worth naming:
		// unlike the projections above, **nothing public reads this one**, so a failure here cannot degrade
		// a public page — it takes the curator's tool away and leaves the published albums untouched.
		if t, cerr := photo.New(ev.publisherOrNil(), ev.writer, ev.reader); cerr != nil {
			logger.Error("photo projection unavailable", "err", cerr)
		} else {
			photos = t
		}

		if t, cerr := publicpatrol.New(ev.publisherOrNil(), ev.writer, ev.reader); cerr != nil {
			logger.Error("public patrol projection unavailable", "err", cerr)
		} else {
			publicPatrols = t
		}

		if t, cerr := year.New(ev.publisherOrNil(), ev.writer, ev.reader); cerr != nil {
			logger.Error("year projection unavailable", "err", cerr)
		} else {
			years = t
		}

		if t, cerr := patrolphoto.New(ev.publisherOrNil(), ev.writer, ev.reader); cerr != nil {
			logger.Error("patrol photo projection unavailable", "err", cerr)
		} else {
			patrolPhotos = t
		}

		if t, cerr := trackpoint.New(ev.publisherOrNil(), ev.writer, ev.reader); cerr != nil {
			logger.Error("track point projection unavailable", "err", cerr)
		} else {
			trackPoints = t
		}

		if t, cerr := pagereport.New(ev.publisherOrNil(), ev.writer, ev.reader); cerr != nil {
			logger.Error("page report projection unavailable", "err", cerr)
		} else {
			pageReports = t
		}
	}

	// The vehicle entity (PRD 010), imported whole from shared-go.
	//
	// Same construction condition as the two projections above, for the same reason: a
	// member must be able to *see* the car they registered during a broker outage, since
	// that read needs only the database. Registering a new one does need the broker, and
	// fails at the command side rather than being prevented here.
	//
	// Note two shape differences from `person.New`/`checkpoint.New`. `vehicle.New` returns
	// no error — it logs a schema failure internally and hands back a usable value — so
	// there is no error branch to write, and a failed CREATE TABLE surfaces only in
	// shared-go's log line here.
	//
	// And it is given a **lazyPublisher, not `ev.publisherOrNil()`**. This is the only
	// entity here that publishes, and the broker connects in the background, so the value
	// `publisherOrNil()` returns at this point is nil — permanently, since it is captured
	// once and kept. That panicked on the first registration and could never have
	// recovered (task 247). See lazypublisher.go.
	var vehicles vehicleTable
	if ev != nil && (err == nil || noBroker) {
		vehicles = vehicle.New(lazyPublisher{holder: publisherFor(ev)}, ev.writer, ev.reader)
	}

	// One process-scoped context for the background workers: the broker connector and
	// projections below, and the portrait purge further down. Hoisted out of the eventing
	// block so both share a single cancellation point rather than one of them running with
	// a context nothing can cancel.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err == nil {
		defer func() {
			if cerr := ev.close(); cerr != nil {
				logger.Error("closing event stream", "err", cerr)
			}
		}()

		// Connect in the background so a broker that is slow or not yet up cannot
		// delay the API. Projections are registered and the dead-letter writer armed
		// from the callback, i.e. only once there is something to consume from.

		// Takes a refusing patrol's photographs out of albums as hq records the refusal (task 397). Created here so
		// the callback can register it; it is handed the application once that exists, below.
		consent = newConsentReactor(logger)

		ev.connectInBackground(ctx, cfg, logger, func() {
			// Step 2 of the three-way registration described in eventing.go.
			var projections []cqrs.Consumer
			if persons != nil {
				projections = append(projections, persons)
			}
			if checkpoints != nil {
				projections = append(projections, checkpoints)
			}
			if sheets != nil {
				projections = append(projections, sheets)
			}
			if handouts != nil {
				projections = append(projections, handouts)
			}
			if checkgroups != nil {
				projections = append(projections, checkgroups)
			}
			if scanProjection != nil {
				projections = append(projections, scanProjection)
			}
			if vehicles != nil {
				projections = append(projections, vehicles)
			}
			if glimts != nil {
				projections = append(projections, glimts)
			}
			if albums != nil {
				projections = append(projections, albums)
			}
			if photos != nil {
				projections = append(projections, photos)
			}
			if publicPatrols != nil {
				projections = append(projections, publicPatrols)
			}
			if years != nil {
				projections = append(projections, years)
			}
			if patrolPhotos != nil {
				projections = append(projections, patrolPhotos)
			}
			if trackPoints != nil {
				projections = append(projections, trackPoints)
			}
			if pageReports != nil {
				projections = append(projections, pageReports)
			}
			if patrolPhotos != nil && photos != nil && albums != nil {
				projections = append(projections, consent)
			}

			ev.registerProjections(logger, projections...)

			if rerr := ev.run(ctx); rerr != nil {
				// Not fatal, for the same reason the connection is not: a broker
				// problem must not stop the API serving reads during an event.
				logger.Error("starting projections", "err", rerr)
				return
			}
			// Only arm the dead-letter writer once projections are running, so
			// schema creation still fails loudly rather than being captured.
			ev.arm()

			// Report the count once, now: any rows still here after Reset() are
			// from this run's replay, so this is the first honest reading.
			if n, cerr := ev.deadletterCount(); cerr != nil {
				logger.Error("reading dead-letter count", "err", cerr)
			} else if n > 0 {
				logger.Warn("replay produced dead-lettered statements", "count", n)
			} else {
				logger.Info("projections running, dead-letter queue empty")
			}

			// Keep reporting a non-zero count: a capture logged during a replay at
			// 02:00 scrolls out of view otherwise.
			ev.watchDeadletters(ctx, logger, 5*time.Minute)

			// Whether this year's map data can actually reveal anything. Reported here, from the
			// callback, rather than at construction: the numbers are only meaningful once the
			// projections have replayed, and this is the first moment we know they have.
			reportMapReadiness(logger, sheets, scanProjection, cfg.eventYear)
		})
	}

	// The reveal rule (PRD 016), which decides what a patrol may see on the map.
	//
	// Composed from four projections, so it exists only when all four do. Partial construction is
	// deliberately refused rather than degraded: a rule missing its handout projection would evaluate to
	// "nothing revealed" for every patrol, which the client would cache offline and keep for the night. An
	// honest 503 is recoverable; a cached empty map is not.
	var mapReads *reveal.Rule
	if sheets != nil && handouts != nil && scanProjection != nil && checkpoints != nil && checkgroups != nil {
		mapReads = reveal.New(sheets, handouts, scanProjection, checkpoints, checkgroups)
	} else if ev != nil {
		logger.Warn("map reveal rule unavailable: one of its projections did not build",
			"sheets", sheets != nil, "handouts", handouts != nil, "scans", scanProjection != nil,
			"checkpoints", checkpoints != nil, "checkgroups", checkgroups != nil)
	}

	// The public patrol page's gate (PRD 011 §0b.3, task 330). Composed from three projections, and —
	// like the reveal rule above — refused rather than degraded when one is missing. A partial gate fails
	// in two directions and neither is worth guessing at: without the checkgroups nothing is the finish
	// line, which is closed and therefore safe; without the closing instant every non-finishing patrol's
	// page is withheld indefinitely, which is silent. So all three, or nothing.
	var publicGate *publicgate.Gate
	if checkgroups != nil && scanProjection != nil && checkpoints != nil {
		publicGate = publicgate.New(checkgroups, scanProjection, checkpoints,
			publicgate.WithOverride(publicGateOverride(cfg.publicPagePatrolOverride, logger)))
	} else if ev != nil {
		logger.Warn("public patrol gate unavailable: one of its projections did not build",
			"checkgroups", checkgroups != nil, "scans", scanProjection != nil,
			"checkpoints", checkpoints != nil)
	}

	// Binary objects. This is the one store whose contents cannot be rebuilt by
	// replaying the log, so an in-memory fallback is a real limitation rather than
	// a convenience — log it plainly instead of letting it look configured.
	blobs := blob.Store(blob.NewMemoryStore())
	if cfg.blobPath == "" {
		logger.Warn("no BLOB_PATH configured: binary objects are in memory and will not survive a restart")
	} else if fs, berr := blob.NewFileStore(cfg.blobPath); berr != nil {
		// Not fatal: nothing writes blobs yet (PRD 003 is what starts), so refusing
		// to boot would trade a working API for a feature that does not exist.
		logger.Error("blob store unavailable, falling back to memory", "path", cfg.blobPath, "err", berr)
	} else {
		blobs = fs
		logger.Info("blob store ready", "path", cfg.blobPath)
	}

	// The freshness check's metrics, constructed before the caches so each can be handed the same
	// instance (task 293).
	syncMetrics := newSyncMetrics()

	// SMS delivery (login PINs). Fatal on a bad DSN: the alternative is booting with
	// a sender nobody configured, which in production means PINs are written to the
	// log and no one receives one — a silent, total failure of login.
	smsSender, err := sms.NewSender(cfg.smsDSN, logger)
	if err != nil {
		return err
	}
	if cfg.smsDSN == "" {
		logger.Warn("no SMS_DSN configured: login PINs are logged, not sent")
	} else {
		logger.Info("sms sender ready", "provider", smsProviderName(cfg.smsDSN))
	}

	// Say plainly, once, whether the photographer admin tool is served (PRD 022, task 370).
	//
	// Logged either way rather than only when absent, because both states are worth being able to confirm
	// from a log: the tool is used hard for a week and then not at all for a year, and "is /admin live on this
	// deployment?" is a question somebody will ask months later about a service they have not thought about.
	//
	// Info rather than Warn for the absent case — unlike a missing BLOB_PATH or SMS_DSN, no admin password is a
	// perfectly good production posture for most of the year, and warning about it would train people to
	// ignore the warning.
	if adminRoutesEnabled(cfg) {
		// The password's **length** rather than the password, because that is the only control on this surface
		// since task 388 dropped the rate limiter — so "is it a generated secret or is it `foto2026`?" is a
		// question the boot log should let somebody answer without asking anybody. The value itself is never
		// logged, obviously.
		logger.Info("photographer admin tool served at /admin",
			"year", cfg.eventYear, "user", cfg.adminUser,
			"password_length", len(cfg.adminPassword))
	} else {
		logger.Info("photographer admin tool absent: no ADMIN_PASSWORD, so /admin and /api/admin/* are not registered")
	}

	app := &application{
		JsonApi: bff.JsonApi{Logger: logger},
		config:  cfg,
		models: data.NewModels(directory, scanSourceFor(scanProjection, peopleOrNil(persons), cfg.eventYear, logger),
			raceAreasOrNil(checkpoints), peopleOrNil(persons), vehiclesOrNil(vehicles),
			data.WithMapReads(mapReadsFor(mapReads, ev != nil, logger)),
			data.WithGlimt(glimtQueriesOrNil(glimts)),
			data.WithAlbums(albumQueriesOrNil(albums)),
			data.WithPhotos(photoQueriesOrNil(photos)),
			// The admin tool's draft-visible reads. Deliberately a separate option from the two above, so
			// that this one line is the entire answer to "what can see an unpublished album?" (PRD 022 §8.8).
			data.WithCuratorReads(albumCuratorOrNil(albums), photoCuratorOrNil(photos)),
			// The admin tool's checkpoint list. A separate option from the reads above because it crosses a
			// different boundary — those widen publication visibility, this widens what can be enumerated about
			// the event's geography (PRD 002). See data.WithCheckpointCurator.
			data.WithCheckpointCurator(checkpointCuratorOrNil(checkpoints)),
			data.WithPublicPatrols(publicPatrolQueriesOrNil(publicPatrols)),
			data.WithYears(yearQueriesOrNil(years)),
			data.WithPatrolPhotos(patrolPhotoQueriesOrNil(patrolPhotos))),
		commands: commands.New(publisherFor(ev)),
		vehicles: vehicleCommandsOrNil(vehicles),
		db:       db,
		eventing: ev,
		blobs:    blobs,

		// The public patrol page's gate (task 330). Nil fails closed — see the field's doc.
		publicGate: publicGate,

		// The patrol photograph fetcher (task 361). Nil when PHOTO_BASEURL is unset or there is no blob store,
		// which means diplomas carry no photograph — the same outcome as a patrol nobody photographed.
		photos: photobytes.New(cfg.photoBaseURL, blobs, logger),

		// The patrol's merged route (task 340), composed from the member list and the telemetry points,
		// and cached per patrol. Nil when either projection is missing — see newPatrolTrackReader.
		patrolTracks: newPatrolTrackReader(peopleOrNil(persons),
			trackPointQueriesOrNil(trackPoints), cfg.eventYear),

		// The freshness check's own numbers (task 293). Attached to each cache below, so a derivation is
		// counted where it happens rather than at whichever endpoint asked for it.
		syncMetrics: syncMetrics,

		// Five seconds of version caching. The client polls every ~60 s, so this adds at
		// most a few seconds to how stale an answer can be — well inside PRD 007's
		// "without too much delay" — while collapsing several hundred devices' polls into
		// a handful of queries per minute.
		contactsVersions: newVersionCache(5*time.Second).observedAs("contacts", syncMetrics),

		// The map datasets get their own caches (task 269). Same 5 s bound as contacts for now; PRD 017
		// gives the whole check one served interval instead.
		checkpointsVersions: newVersionCache(5*time.Second).observedAs("checkpoints", syncMetrics),
		handoutsVersions:    newVersionCache(5*time.Second).observedAs("handouts", syncMetrics),
		scansVersions:       newVersionCache(5*time.Second).observedAs("scans", syncMetrics),
		profileVersions:     newVersionCache(5*time.Second).observedAs("profile", syncMetrics),
		raceAreaVersions:    newVersionCache(5*time.Second).observedAs("race_area", syncMetrics),

		pins: pinStoreFor(cfg),
		sms:  smsSender,
		sessions: session.NewManager(
			[]byte(cfg.sessionSecret),
			7*24*time.Hour, // ≥ 7-day session per PRD
			cfg.sessionSecure,
		),
		// Same secret as the session manager on purpose: both are server-side signing
		// keys with the same blast radius, and a second secret to configure is a second
		// secret to forget to set in production.
		choices: choice.NewManager([]byte(cfg.sessionSecret), choice.DefaultTTL),
		// Allow a modest burst of PIN requests per IP per minute.
		requestPinLimiter: ratelimit.New(5, time.Minute),
		// Position batches arrive every 2 minutes per client (PRD 002 §11.1), so 20 a
		// minute is ~40× the expected rate. The headroom is not slack: a client that has
		// been offline ships its backlog in several chunked requests in quick succession
		// (task 083), which is exactly when a tight limit would throttle the data it is
		// most important not to lose.
		trackLimiter: ratelimit.New(20, time.Minute),
		// Ten portrait uploads an hour, per member (maintainer's number). Generous
		// against the real use — take a photo, dislike it, retake it a few times — and
		// far below what it would take to fill a disk or keep a CPU busy.
		photoLimiter: ratelimit.New(10, time.Hour),
		// Sixty Glimt media items an hour, per member — now configurable (task 311). Six full
		// ten-item posts in an hour is already an unusual night, and the ceiling is there to stop
		// a looping client rather than to ration sharing: the feature exists to be used.
		//
		// Paired with glimtMediaBudget below, because a count alone cannot tell sixty thumbnails
		// from sixty 12 MiB videos.
		glimtMediaLimiter: limiterOrNil(cfg.glimtMediaPerHour, time.Hour),
		// The byte half of the upload limit. 200 MiB an hour by default — roughly sixty full-size
		// photographs, or a handful of videos, per member per hour.
		glimtMediaBudget: ratelimit.NewBudget(cfg.glimtBytesPerHour, time.Hour),
		// Twenty glimt an hour, per member. A busy night for an enthusiastic patrulje is a
		// handful of posts; twenty leaves room for that and for a few retries, while still
		// bounding what one looping client can put on the stream.
		glimtLimiter: limiterOrNil(cfg.glimtPerHour, time.Hour),
		// Reads, and the number that matters most in this block: **3000 per minute per member**,
		// two orders of magnitude looser than the write limits and measured per minute rather
		// than per hour.
		//
		// Both of those are deliberate. The post-race browse is a legitimate flood (PRD 019
		// §0a.3) — a thousand people at the finish line, each pulling a grid of thumbnails per
		// screen — and it is the use this whole feature was built for. A limiter tuned anywhere
		// near the upload numbers would throttle exactly that, and an *hourly* budget would be
		// spent by somebody scrolling for two minutes and then locked out for fifty-eight.
		//
		// It was 600 until task 324 measured a hold page at ~43 requests — fourteen pages a
		// minute, which a member flicking through grids beats. See env.go for the arithmetic.
		//
		// So this exists to stop a script hammering the endpoint and for nothing else. If it ever
		// fires for a real member, it is set wrong.
		glimtReadLimiter: limiterOrNil(cfg.glimtReadsPerMinute, time.Minute),
		// The public page, by IP. Deliberately looser again than the member read limit: one IP may
		// legitimately be a whole school, and this is the surface a link in a parents' group chat
		// lands on — the worst possible moment to start answering 429.
		publicGlimtReadLimiter: limiterOrNil(cfg.glimtPublicReadsPerMinute, time.Minute),
		// Public media, by IP, on its own budget: thumbnails outnumber page loads by up to sixty to
		// one, so sharing would let a scroll through an album exhaust what the next page needs.
		publicMediaReadLimiter: limiterOrNil(cfg.publicMediaReadsPerMinute, time.Minute),
		// Anonymous reports, by IP. Tighter than the authenticated 100/hour, because an
		// unauthenticated write should be, but still far beyond honest use.
		publicReportLimiter: limiterOrNil(cfg.glimtPublicReportsPerHour, time.Hour),
		// A hundred reports an hour per member. Far beyond any honest use, which is the
		// point: reporting is the safety mechanism for an unmoderated public scope
		// (PRD 019 §0), so the ceiling is set to stop a script rather than to shape
		// behaviour.
		glimtReportLimiter: ratelimit.New(100, time.Hour),
		// Twenty confirmation attempts an hour per IP. Generous against the real use — a
		// member types two digits once, perhaps twice, and may then report the number as
		// wrong — while leaving room for a shared network: a patrol on one hotspot all
		// confirming during the same briefing must not throttle each other.
		confirmLimiter: ratelimit.New(20, time.Hour),
		// There is no admin limiter. Task 371 added one, task 388 removed it — the reasoning is at
		// `requireAdmin` in middleware.go, and the consequence (the password's entropy is the only control on
		// that surface) is at `config.adminPassword`.
		//
		// Long enough that a member who leaves the check open, locks their phone and comes back
		// still has the same budget, short enough that the map does not accumulate one entry per
		// login for the life of the process. Forgetting is the generous direction: it hands back
		// attempts rather than taking them away.
		contactChecks: newContactCheck(6 * time.Hour),

		pushStore: push.NewMemoryStore(),
	}
	if consent != nil {
		consent.app.Store(app)
	}

	logger.Info("configuration loaded", "env", cfg.env, "port", cfg.port, "web_root", cfg.webRoot, "version", vcs.Version())

	// Portrait retention (task 109). Started here rather than as a second binary: per the
	// BFF conventions, extra work belongs in this process, and this one needs exactly the
	// dependencies the app already holds.
	//
	// Six-hourly is deliberately unhurried. Retention is measured in days, so the only
	// thing a shorter interval would buy is more log noise and more load; the only thing a
	// longer one would cost is a few hours of a portrait outliving its window.
	app.runPortraitPurge(ctx, 6*time.Hour, logger)
	// Glimt retention (PRD 019 §6, task 310). Six-hourly like the portrait purge and for the same
	// reason: retention is measured in days, so a shorter interval buys nothing but queries. Note
	// this sweep enforces `glimtRetention` only — `glimtPublicRetention` is a read-time cutoff in
	// the public feed, not a job.
	app.runGlimtPurge(ctx, 6*time.Hour, logger)

	return app.Serve(app.routes(), cfg.port)
}
