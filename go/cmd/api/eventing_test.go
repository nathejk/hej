package main

import (
	"io"
	"log/slog"
	"testing"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// openEventing needs a database to project into. Without one there is nothing to
// wire, so it reports ErrNoDSN rather than connecting to a broker it cannot use.
func TestOpenEventingWithoutDatabase(t *testing.T) {
	ev, err := openEventing(config{jetstreamDSN: "nats://127.0.0.1:1"}, nil, quietLogger())
	if err == nil {
		t.Fatal("want an error when there is no database")
	}
	if ev != nil {
		t.Fatalf("want nil eventing, got %v", ev)
	}
}

// The nil-receiver paths matter: main calls these on a value that is nil whenever
// the database is absent, so they must be safe rather than panic. This is the
// degraded mode PRD 008 §5 requires, exercised directly.
func TestEventingNilReceiverIsSafe(t *testing.T) {
	var ev *eventing

	if err := ev.close(); err != nil {
		t.Fatalf("close on nil: %v", err)
	}
	if err := ev.run(t.Context()); err != nil {
		t.Fatalf("run on nil: %v", err)
	}
	ev.arm() // must not panic

	n, err := ev.deadletterCount()
	if err != nil {
		t.Fatalf("deadletterCount on nil: %v", err)
	}
	if n != 0 {
		t.Fatalf("want 0 dead letters on nil, got %d", n)
	}

	// Registering projections with no mux must warn, not panic or error.
	ev.registerProjections(quietLogger())
}

// A populated eventing with no broker (no mux) must also tolerate registration and
// running: this is the "database up, broker down" case, which is the one the app is
// explicitly designed to keep serving in.
func TestEventingWithoutBrokerTolerated(t *testing.T) {
	ev := &eventing{} // no stream, no mux, no writer

	if err := ev.run(t.Context()); err != nil {
		t.Fatalf("run without a broker should be a no-op, got %v", err)
	}
	ev.registerProjections(quietLogger())
	ev.arm()

	if err := ev.close(); err != nil {
		t.Fatalf("close without a broker: %v", err)
	}
}
