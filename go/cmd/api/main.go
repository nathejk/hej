package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"time"

	// Embed the timezone database in the binary. The prod image is bare alpine with
	// no system tzdata, and the person projection needs Europe/Copenhagen to convert
	// upstream birthdays (stored as midnight-local expressed in UTC) to the right
	// calendar date. Without this the conversion silently falls back to UTC and every
	// such birthday lands a day early.
	_ "time/tzdata"

	"github.com/jrgensen/cqrs"

	bff "nathejk.dk/cmd/api/app"
	"nathejk.dk/internal/blob"
	"nathejk.dk/internal/choice"
	"nathejk.dk/internal/commands"
	"nathejk.dk/internal/data"
	"nathejk.dk/internal/pin"
	"nathejk.dk/internal/push"
	"nathejk.dk/internal/ratelimit"
	"nathejk.dk/internal/scans"
	"nathejk.dk/internal/session"
	"nathejk.dk/internal/sms"
	"nathejk.dk/internal/users"
	"nathejk.dk/internal/vcs"
	"nathejk.dk/nathejk/table/checkpoint"
	"nathejk.dk/nathejk/table/person"
)

// application is the root dependency container for the API binary. It embeds
// bff.JsonApi to inherit the transport-layer helpers (WriteJSON, ReadJSON,
// error responses, Serve) so handlers can call them via the `app` receiver.
type application struct {
	bff.JsonApi
	config   config
	models   data.Models
	commands commands.Commands

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

		ev.connectInBackground(ctx, cfg, logger, func() {
			// Step 2 of the three-way registration described in eventing.go.
			var projections []cqrs.Consumer
			if persons != nil {
				projections = append(projections, persons)
			}
			if checkpoints != nil {
				projections = append(projections, checkpoints)
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
		})
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

	app := &application{
		JsonApi:  bff.JsonApi{Logger: logger},
		config:   cfg,
		models:   data.NewModels(directory, scans.NewMockSource(), raceAreasOrNil(checkpoints), peopleOrNil(persons)),
		commands: commands.New(publisherFor(ev)),
		db:       db,
		eventing: ev,
		blobs:    blobs,

		// Five seconds of version caching. The client polls every ~60 s, so this adds at
		// most a few seconds to how stale an answer can be — well inside PRD 007's
		// "without too much delay" — while collapsing several hundred devices' polls into
		// a handful of queries per minute.
		contactsVersions: newVersionCache(5 * time.Second),

		pins: pin.NewStore(),
		sms:  sms.LogSender{Logger: logger},
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
		// Twenty confirmation attempts an hour per IP. Generous against the real use — a
		// member types two digits once, perhaps twice, and may then report the number as
		// wrong — while leaving room for a shared network: a patrol on one hotspot all
		// confirming during the same briefing must not throttle each other.
		confirmLimiter: ratelimit.New(20, time.Hour),

		pushStore: push.NewMemoryStore(),
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

	return app.Serve(app.routes(), cfg.port)
}
