package main

import (
	"log/slog"
	"os"
	"time"

	bff "nathejk.dk/cmd/api/app"
	"nathejk.dk/internal/commands"
	"nathejk.dk/internal/data"
	"nathejk.dk/internal/pin"
	"nathejk.dk/internal/push"
	"nathejk.dk/internal/ratelimit"
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
}

// @title        Hej Nathejk API
// @version      0.1.0
// @description  Backend-for-frontend API for the Hej Nathejk event app.
// @BasePath     /api
func main() {
	cfg := loadConfig()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	app := &application{
		JsonApi:  bff.JsonApi{Logger: logger},
		config:   cfg,
		models:   data.NewModels(users.NewMockDirectory()),
		commands: commands.New(),

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

	if err := app.Serve(app.routes(), cfg.port); err != nil {
		logger.Error("server error", "err", err)
		os.Exit(1)
	}
}
