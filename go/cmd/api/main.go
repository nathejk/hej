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

	// The member directory. Starts on the mock and moves to the real projection once
	// that exists, which happens in a background callback below — see
	// switchableDirectory for why the indirection is unavoidable rather than merely
	// convenient. A run with no database or no broker simply stays on the mock, which is
	// the documented degraded mode (PRD 006 §6) and keeps `hej` runnable at every step.
	directory := newSwitchableDirectory(users.NewMockDirectory())

	// The CQRS seam. Both degraded modes are deliberate and distinct:
	//
	//   no database — nothing to project into, so no eventing at all
	//   no broker   — reader/writer still usable, so handlers keep serving
	//                 whatever the last run projected (PRD 008 §5)
	ev, err := openEventing(cfg, db, logger)
	switch {
	case errors.Is(err, ErrNoDSN):
		logger.Warn("no database: skipping event stream wiring")
	case errors.Is(err, ErrNoJetstreamDSN):
		logger.Warn("no JETSTREAM_DSN configured: running without a broker, reads served from existing projections")
	case err != nil:
		logger.Error("event stream setup failed, continuing without it", "err", err)
	default:
		defer func() {
			if cerr := ev.close(); cerr != nil {
				logger.Error("closing event stream", "err", cerr)
			}
		}()

		// Connect in the background so a broker that is slow or not yet up cannot
		// delay the API. Projections are registered and the dead-letter writer armed
		// from the callback, i.e. only once there is something to consume from.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ev.connectInBackground(ctx, cfg, logger, func() {
			// Step 2 of the three-way registration described in eventing.go.
			//
			// The person projection (PRD 006) is constructed here rather than before
			// the connect because it creates its table through the cqrs.Writer, and
			// doing that only once there is a broker keeps a database-only run from
			// creating tables nothing will ever fill.
			var projections []cqrs.Consumer
			// phoneNormalizer adapts internal/phone to the interface the projection
			// declares. Passing the same implementation the login handler uses is the
			// point: a second implementation, or the same rules re-typed, would make
			// lookups silently miss (PRD 006 §2).
			persons, perr := person.New(ev.publisherOrNil(), ev.writer, ev.reader, phoneNormalizer{},
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
				projections = append(projections, persons)
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

			// Step 3 of the three-way registration: expose the projection's read API.
			//
			// Installed as soon as the projections are running, not after the replay
			// finishes, and that is safe for a specific reason: nothing truncates the
			// person table on boot. A restart therefore serves the *previous* run's rows
			// while the replay re-upserts them, so there is no window in which the
			// directory is empty and a member is wrongly told their number is unknown.
			// This is PRD 008 §5's "reads served from existing projections" in practice.
			//
			// If a future change ever does truncate on boot, this must move behind a
			// caught-up signal — the stream library has one (CatchupListener).
			if persons != nil {
				directory.set(newPersonDirectory(persons, cfg.eventYear, logger))
				logger.Info("member directory now reading the person projection",
					"year", cfg.eventYear)
			}

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
		models:   data.NewModels(directory, scans.NewMockSource()),
		commands: commands.New(publisherFor(ev)),
		db:       db,
		eventing: ev,
		blobs:    blobs,

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

		pushStore: push.NewMemoryStore(),
	}

	logger.Info("configuration loaded", "env", cfg.env, "port", cfg.port, "web_root", cfg.webRoot, "version", vcs.Version())

	return app.Serve(app.routes(), cfg.port)
}
