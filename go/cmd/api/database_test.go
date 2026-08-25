package main

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

// discardLogger keeps test output readable; openDB logs a warning per retry.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestOpenDBNoDSN(t *testing.T) {
	db, err := openDB(config{}, discardLogger())
	if !errors.Is(err, ErrNoDSN) {
		t.Fatalf("want ErrNoDSN, got %v", err)
	}
	if db != nil {
		t.Fatalf("want nil pool when no DSN is configured, got %v", db)
	}
}

// TestOpenDBUnreachableGivesUp is the behaviour PRD 008 asks for: the ping is
// retried (container start order) but bounded, so a missing database yields an
// error the caller can degrade on rather than hanging the process forever.
func TestOpenDBUnreachableGivesUp(t *testing.T) {
	cfg := config{
		// Port 1 on loopback: nothing listens, and connections are refused fast.
		dbDSN:             "hej:hej@tcp(127.0.0.1:1)/hej",
		dbMaxOpenConns:    1,
		dbMaxIdleConns:    1,
		dbConnMaxLifetime: time.Minute,
		dbConnectTimeout:  700 * time.Millisecond,
	}

	start := time.Now()
	db, err := openDB(cfg, discardLogger())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("want an error for an unreachable database")
	}
	if errors.Is(err, ErrNoDSN) {
		t.Fatalf("want a connection error, got ErrNoDSN")
	}
	if db != nil {
		t.Fatal("want nil pool when the database is unreachable")
	}
	// Generous upper bound: we only care that it is bounded, not that it is fast.
	if elapsed > 10*time.Second {
		t.Fatalf("openDB took %s; the retry budget is not being honoured", elapsed)
	}
}

func TestOpenDBInvalidDSN(t *testing.T) {
	cfg := config{
		dbDSN:            "this is not a dsn",
		dbConnectTimeout: time.Second,
	}
	if _, err := openDB(cfg, discardLogger()); err == nil {
		t.Fatal("want an error for a malformed DSN")
	}
}
