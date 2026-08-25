package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/cqrs/deadletter"
	"github.com/jrgensen/cqrs/sqlpersister"
	"github.com/jrgensen/stream"
	"github.com/jrgensen/stream/jetstream"
	"github.com/jrgensen/stream/metatagger"
	"github.com/jrgensen/stream/xstream"
	"github.com/nathejk/shared-go/messages"

	"nathejk.dk/internal/commands"
	"nathejk.dk/internal/vcs"
)

// ErrNoJetstreamDSN is returned by openEventing when no broker DSN is configured.
// Like ErrNoDSN it is a mode, not a fault: PRD 008 §5 requires the API to serve
// reads with no broker, because during an event degraded-and-serving beats
// correct-and-dead.
var ErrNoJetstreamDSN = errors.New("no JetStream DSN configured")

// producerName identifies this service in the metadata attached to every event it
// publishes. It is what tells someone reading the log which service wrote a
// message, so it must not be a copy-paste of another repo's value.
const producerName = "hej-api"

// eventing bundles the CQRS seam: the three interfaces every shared-go entity
// constructor takes, plus the pieces main needs to run projections.
//
// The split is the one described in PRD 008 §8:
//
//	cqrs.Reader    query side       — *sql.DB directly
//	cqrs.Writer    projection side  — deadletter wrapping sqlpersister
//	cqrs.Publisher command side     — metatagger over JetStream
//
// Entity constructors take these interfaces and never a *sql.DB or a concrete
// stream, which is what keeps a projection liftable to shared-go later (PRD 006).
type eventing struct {
	// mu guards the fields the background connector installs.
	mu        sync.Mutex
	stream    stream.Stream
	publisher cqrs.Publisher

	writer *deadletter.Writer
	reader cqrs.Reader

	// holder is what handlers see. It exists so the publisher can arrive after
	// handlers are wired (task 058) without re-wiring them.
	holder *commands.PublisherHolder

	// mux fans subjects out to the registered projectors. Nil until connected.
	mux interface {
		AddConsumer(...stream.Consumer)
		Run(ctx context.Context) error
	}
}

// openEventing builds the CQRS seam.
//
// It needs a database: without one there is nowhere to project to, so there is no
// point connecting to a broker. With a database but no broker it returns
// ErrNoJetstreamDSN and a partially populated value — reader and writer are still
// usable, which is what lets handlers read existing projections during an outage.
func openEventing(cfg config, db *sql.DB, logger *slog.Logger) (*eventing, error) {
	if db == nil {
		return nil, ErrNoDSN
	}

	// The writer is wrapped in a dead-letter writer so that one failing statement
	// (a value that overflows its column, say) is captured rather than killing the
	// consumer loop and stalling every projection behind it.
	//
	// It stays a transparent pass-through until Arm() is called, which is
	// deliberate: table creation and any seeding happen before arming, so a broken
	// schema still fails loudly at startup instead of being quietly dead-lettered.
	writer := deadletter.New(sqlpersister.New(db), db)
	if err := writer.Consume(writer.CreateTableSql()); err != nil {
		return nil, fmt.Errorf("create deadletter table: %w", err)
	}
	// Projections are rebuilt by replaying the stream on every boot, so
	// dead-letters captured during the previous run's replay are stale.
	if err := writer.Reset(); err != nil {
		return nil, fmt.Errorf("reset deadletter table: %w", err)
	}

	ev := &eventing{
		writer: writer,
		reader: db,
		holder: commands.NewPublisherHolder(),
	}

	if cfg.jetstreamDSN == "" {
		return ev, ErrNoJetstreamDSN
	}

	return ev, nil
}

// connect establishes the broker connection, installs the publisher and returns
// the mux to register projections on.
func (ev *eventing) connect(cfg config, logger *slog.Logger) error {
	js, err := jetstream.New(cfg.jetstreamDSN)
	if err != nil {
		return fmt.Errorf("connect jetstream: %w", err)
	}

	publisher, err := metatagger.New(js, messages.Metadata{
		Producer: producerName,
		Version:  vcs.Version(),
	})
	if err != nil {
		_ = js.Close()
		return fmt.Errorf("create publisher: %w", err)
	}

	ev.mu.Lock()
	ev.stream = js
	ev.publisher = publisher
	ev.mux = xstream.NewMux(js)
	ev.mu.Unlock()

	// Handlers reach the publisher through the holder, so this is the moment writes
	// start succeeding — no re-wiring needed.
	ev.holder.Set(publisher)

	logger.Info("jetstream connected", "producer", producerName)
	return nil
}

// connectInBackground keeps trying to reach the broker, with capped exponential
// backoff, until it succeeds or ctx is cancelled.
//
// This is what makes "startup does not block on the broker" true in the case that
// actually matters: not "the broker is missing forever" (that degrades fine) but
// "the broker comes up thirty seconds after we do", which is the normal case when a
// whole stack starts at once. Without the retry the API would run publish-less
// until someone restarted it.
//
// onConnect runs once, after a successful connection, so the caller can register
// projections and arm the dead-letter writer at the right moment.
func (ev *eventing) connectInBackground(ctx context.Context, cfg config, logger *slog.Logger, onConnect func()) {
	go func() {
		const (
			initialDelay = time.Second
			maxDelay     = 30 * time.Second
		)
		delay := initialDelay

		for attempt := 1; ; attempt++ {
			if err := ev.connect(cfg, logger); err == nil {
				if onConnect != nil {
					onConnect()
				}
				return
			} else if attempt == 1 {
				// Log the first failure at warning level and the rest at debug: a
				// broker that is down for an hour should not produce an hour of
				// identical error lines that bury everything else.
				logger.Warn("broker unreachable, retrying in the background", "err", err)
			} else {
				logger.Debug("broker still unreachable", "attempt", attempt, "err", err)
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			if delay < maxDelay {
				delay *= 2
				if delay > maxDelay {
					delay = maxDelay
				}
			}
		}
	}()
}

// registerProjections wires the projectors onto the mux.
//
// The pattern every entity follows — see PRD 008 §8 and go-bff-layout — is a
// three-way registration of one value:
//
//  1. construct it:            persontable := person.New(publisher, writer, reader)
//  2. register it here:        projections = append(projections, persontable)
//  3. expose its read API:     data.NewModels(..., persontable)
//
// Miss step 2 and the entity's tables simply never fill: reads work, return
// nothing, and nothing errors. That silence is why the slice exists in one place
// rather than each entity subscribing itself.
//
// The set is empty today; PRD 006's person projection is the first member.
func (ev *eventing) registerProjections(logger *slog.Logger, projections ...cqrs.Consumer) {
	if ev == nil {
		return
	}
	ev.mu.Lock()
	mux := ev.mux
	ev.mu.Unlock()

	if mux == nil {
		if len(projections) > 0 {
			logger.Warn("no broker: projections registered but will not receive events",
				"count", len(projections))
		}
		return
	}

	consumers := make([]stream.Consumer, 0, len(projections))
	for _, p := range projections {
		consumers = append(consumers, p)
	}
	mux.AddConsumer(consumers...)
	logger.Info("projections registered", "count", len(projections))
}

// run subscribes the registered consumers to the stream. It returns as soon as
// the subscriptions are established — it does not block — so the caller can go on
// to serve HTTP.
func (ev *eventing) run(ctx context.Context) error {
	if ev == nil {
		return nil
	}
	ev.mu.Lock()
	mux := ev.mux
	ev.mu.Unlock()
	if mux == nil {
		return nil
	}
	return mux.Run(ctx)
}

// arm switches the dead-letter writer from pass-through to capturing.
//
// Called only once projections are running. Before this point a failing statement
// is a broken schema or a bad seed and should crash the boot; after it, a failing
// statement is one bad event that must not be allowed to stall every projection
// behind it.
func (ev *eventing) arm() {
	if ev == nil || ev.writer == nil {
		return
	}
	ev.writer.Arm()
}

// deadletterCount reports how many statements have been captured. Zero is the
// expected value; anything else means a projection is quietly incomplete, which is
// why task 060 surfaces this rather than leaving it in the table.
func (ev *eventing) deadletterCount() (int, error) {
	if ev == nil || ev.writer == nil {
		return 0, nil
	}
	return ev.writer.Count()
}

// watchDeadletters logs a warning whenever captured statements are present.
//
// Three things already exist: the deadletter package logs each capture as it
// happens, the count is in the healthcheck (task 059), and the rows are in the
// table. This adds the one thing missing — a *recurring* signal. A capture that
// happened during a replay at 02:00 scrolls out of view, and nobody polls a
// healthcheck they have no reason to suspect. Repeating it means an operator who
// looks at the log at any later point still learns that a projection is incomplete.
//
// Deliberately quiet when the count is zero, which is the normal case: a periodic
// "everything is fine" line trains people to ignore the channel.
func (ev *eventing) watchDeadletters(ctx context.Context, logger *slog.Logger, every time.Duration) {
	if ev == nil || ev.writer == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n, err := ev.deadletterCount()
				if err != nil {
					logger.Error("reading dead-letter count", "err", err)
					continue
				}
				if n > 0 {
					logger.Warn("dead-lettered statements present: a projection is incomplete",
						"count", n,
						"table", "deadletter",
					)
				}
			}
		}
	}()
}

// publisherFor returns the holder to hand to the write facade. It is never nil, so
// handlers work identically whether or not a broker has been reached yet — they see
// ErrNoPublisher until one arrives.
func publisherFor(ev *eventing) *commands.PublisherHolder {
	if ev == nil {
		return commands.NewPublisherHolder()
	}
	return ev.holder
}

// close releases the broker connection. The database pool is owned by run().
func (ev *eventing) close() error {
	if ev == nil {
		return nil
	}
	ev.mu.Lock()
	s := ev.stream
	ev.mu.Unlock()
	if s == nil {
		return nil
	}
	return s.Close()
}

// connected reports whether a broker connection is currently established.
// Informational only — the healthcheck must never fail readiness on it (task 059).
func (ev *eventing) connected() bool {
	if ev == nil {
		return false
	}
	ev.mu.Lock()
	defer ev.mu.Unlock()
	return ev.publisher != nil
}
