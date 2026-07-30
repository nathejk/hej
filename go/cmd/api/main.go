package main

import (
	"log/slog"
	"os"

	bff "nathejk.dk/cmd/api/app"
	"nathejk.dk/internal/commands"
	"nathejk.dk/internal/data"
)

// application is the root dependency container for the API binary. It embeds
// bff.JsonApi to inherit the transport-layer helpers (WriteJSON, ReadJSON,
// error responses, Serve) so handlers can call them via the `app` receiver.
type application struct {
	bff.JsonApi
	config   config
	models   data.Models
	commands commands.Commands
}

func main() {
	cfg := loadConfig()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	app := &application{
		JsonApi:  bff.JsonApi{Logger: logger},
		config:   cfg,
		models:   data.NewModels(),
		commands: commands.New(),
	}

	logger.Info("configuration loaded", "env", cfg.env, "port", cfg.port, "web_root", cfg.webRoot)

	if err := app.Serve(app.routes(), cfg.port); err != nil {
		logger.Error("server error", "err", err)
		os.Exit(1)
	}
}
