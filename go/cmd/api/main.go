package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"time"

	bff "nathejk.dk/cmd/api/app"
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
		logger.Error("event stream unavailable, continuing without it", "err", err)
	default:
		defer func() {
			if cerr := ev.close(); cerr != nil {
				logger.Error("closing event stream", "err", cerr)
			}
		}()
	}

	if ev != nil {
		// Step 2 of the three-way registration described in eventing.go. Empty
		// today; PRD 006's person projection is the first member.
		ev.registerProjections(logger)

		// Run subscribes the registered consumers and returns; it does not block.
		if rerr := ev.run(context.Background()); rerr != nil {
			// Not fatal, for the same reason the connection is not: a broker
			// problem must not stop the API from serving reads during an event.
			logger.Error("starting projections", "err", rerr)
		} else {
			// Only arm the dead-letter writer once projections are running, so
			// schema creation above still fails loudly rather than being captured.
			ev.arm()
		}
	}

	app := &application{
		JsonApi:  bff.JsonApi{Logger: logger},
		config:   cfg,
		models:   data.NewModels(users.NewMockDirectory(), scans.NewMockSource()),
		commands: commands.New(publisherFor(ev)),
		db:       db,
		eventing: ev,

		pins: pin.NewStore(),
		sms:  sms.LogSender{Logger: logger},
		sessions: session.NewManager(
			[]byte(cfg.sessionSecret),
			7*24*time.Hour, // ≥ 7-day session per PRD
			cfg.sessionSecure,
		),
		// Allow a modest burst of PIN requests per IP per minute.
		requestPinLimiter: ratelimit.New(5, time.Minute),

		pushStore: push.NewMemoryStore(),
	}

	logger.Info("configuration loaded", "env", cfg.env, "port", cfg.port, "web_root", cfg.webRoot, "version", vcs.Version())

	return app.Serve(app.routes(), cfg.port)
}
