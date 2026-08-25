package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	// Registers the "mysql" driver with database/sql. MariaDB speaks the MySQL
	// protocol, so this is the driver for both.
	_ "github.com/go-sql-driver/mysql"
)

// ErrNoDSN is returned by openDB when no DSN is configured. It is not a failure:
// the API is expected to run without a database until PRD 008's projections
// exist, so callers treat it as "no database this run" rather than an error to
// exit on.
var ErrNoDSN = errors.New("no database DSN configured")

// openDB opens a pooled MariaDB connection and verifies it with a bounded retry.
//
// The retry exists because of container start order, not flakiness: in the dev
// stack `api` and `db` come up together and MariaDB takes a few seconds to accept
// connections, so a single ping at t=0 would fail every time `docker compose up`
// is run from cold. depends_on waits for the container, not for the server inside
// it.
//
// It is bounded rather than infinite on purpose. An API that blocks forever on a
// missing database is indistinguishable from a hung deploy, and during an event
// the useful behaviour is to come up and say loudly that the database is absent
// (see the healthcheck) rather than to never come up at all.
func openDB(cfg config, logger *slog.Logger) (*sql.DB, error) {
	if cfg.dbDSN == "" {
		return nil, ErrNoDSN
	}

	db, err := sql.Open("mysql", cfg.dbDSN)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(cfg.dbMaxOpenConns)
	db.SetMaxIdleConns(cfg.dbMaxIdleConns)
	db.SetConnMaxLifetime(cfg.dbConnMaxLifetime)

	if err := pingWithRetry(db, cfg.dbConnectTimeout, logger); err != nil {
		// Close the pool we just opened; leaking it would keep a background
		// connector alive for a database we are not going to use.
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

// pingWithRetry pings db until it answers or the timeout expires.
func pingWithRetry(db *sql.DB, timeout time.Duration, logger *slog.Logger) error {
	deadline := time.Now().Add(timeout)
	const interval = 500 * time.Millisecond

	var lastErr error
	for attempt := 1; ; attempt++ {
		// Each ping gets its own short context so a hanging TCP connect cannot
		// consume the whole budget in one attempt.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := db.PingContext(ctx)
		cancel()

		if err == nil {
			if attempt > 1 {
				logger.Info("database reachable", "attempts", attempt)
			}
			return nil
		}
		lastErr = err

		if time.Now().Add(interval).After(deadline) {
			return fmt.Errorf("database unreachable after %s: %w", timeout, lastErr)
		}
		logger.Warn("database not ready, retrying", "attempt", attempt, "err", err)
		time.Sleep(interval)
	}
}
