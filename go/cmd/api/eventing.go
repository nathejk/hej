package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/cqrs/deadletter"
	"github.com/jrgensen/cqrs/sqlpersister"
	"github.com/jrgensen/stream"
	"github.com/jrgensen/stream/jetstream"
	"github.com/jrgensen/stream/metatagger"
	"github.com/jrgensen/stream/xstream"
	"github.com/nathejk/shared-go/messages"

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
	stream    stream.Stream
	publisher cqrs.Publisher
	writer    *deadletter.Writer
	reader    cqrs.Reader

	// mux fans subjects out to the registered projectors. Nil when there is no
	// broker.
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
	}

	if cfg.jetstreamDSN == "" {
		return ev, ErrNoJetstreamDSN
	}

	js, err := jetstream.New(cfg.jetstreamDSN)
	if err != nil {
		return ev, fmt.Errorf("connect jetstream: %w", err)
	}

	publisher, err := metatagger.New(js, messages.Metadata{
		Producer: producerName,
		Version:  vcs.Version(),
	})
	if err != nil {
		return ev, fmt.Errorf("create publisher: %w", err)
	}

	ev.stream = js
	ev.publisher = publisher
	ev.mux = xstream.NewMux(js)

	logger.Info("jetstream connected", "producer", producerName)
	return ev, nil
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
	if ev == nil || ev.mux == nil {
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
	ev.mux.AddConsumer(consumers...)
	logger.Info("projections registered", "count", len(projections))
}

// run subscribes the registered consumers to the stream. It returns as soon as
// the subscriptions are established — it does not block — so the caller can go on
// to serve HTTP.
func (ev *eventing) run(ctx context.Context) error {
	if ev == nil || ev.mux == nil {
		return nil
	}
	return ev.mux.Run(ctx)
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

// close releases the broker connection. The database pool is owned by run().
func (ev *eventing) close() error {
	if ev == nil || ev.stream == nil {
		return nil
	}
	return ev.stream.Close()
}
