// Package commands is the write-side facade handed to HTTP handlers.
//
// Every state change in this service is published as an event; SQL tables are
// projections of that log and are never written to directly. This package is the
// only write path handlers may use, which is what keeps that rule enforceable:
// a handler holds a Commands value and has no way to reach a *sql.DB or the
// stream, so "just UPDATE the row" is not an option it can reach for.
//
// The facade is deliberately thin. As aggregates are added, each one contributes
// its own command interface here (the mature sibling repos collect those
// interfaces in cmd/api rather than in a package — see go-bff-layout), so this
// type is a collection point, not a home for domain logic.
package commands

import (
	"errors"
	"fmt"

	"github.com/jrgensen/cqrs"
)

// ErrNoPublisher is returned when a command is attempted with no broker
// connection.
//
// Failing the request is the deliberate choice. The read path is designed to
// survive a broker outage by serving existing projections (PRD 008 §5), but a
// *write* that cannot be published has not happened, and reporting success would
// tell a user their data was saved when it was not — for PRD 005's verification
// step that would mean a member believing their guardian number was confirmed
// when nothing recorded it.
var ErrNoPublisher = errors.New("no event publisher configured")

// Commands is the write-side facade passed to handlers.
type Commands struct {
	// publisher is nil when the service runs without a broker.
	publisher cqrs.Publisher
}

// New constructs the write-side facade. A nil publisher is allowed: the service
// still starts and serves reads, and commands fail with ErrNoPublisher.
func New(publisher cqrs.Publisher) Commands {
	return Commands{publisher: publisher}
}

// Available reports whether commands can currently be published. Handlers can use
// it to fail fast with a clearer message than a mid-request error.
func (c Commands) Available() bool { return c.publisher != nil }

// Publish sends one event.
//
// body is marshalled by the message implementation, so callers pass the typed
// message struct from shared-go (or a local equivalent) rather than bytes.
//
// Subjects are named with cqrs.SubjectFromStr by the caller, keeping subject
// vocabulary next to the command that uses it rather than centralised in a
// registry that drifts from reality.
func (c Commands) Publish(subject cqrs.Subject, body any) error {
	if c.publisher == nil {
		return ErrNoPublisher
	}
	msg := c.publisher.MessageFunc()(subject)
	if err := msg.SetBody(body); err != nil {
		// Marshalling failed, so there is nothing worth publishing. Returning here
		// rather than publishing an empty body matters: a subscriber cannot tell an
		// event with a missing body from one that legitimately has no fields set.
		return fmt.Errorf("set event body: %w", err)
	}
	return c.publisher.Publish(msg)
}
