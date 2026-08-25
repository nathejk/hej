package person

import (
	"github.com/jrgensen/cqrs"
)

// consumer folds member events into the read model.
//
// The subject lists and handlers arrive with tasks 072-075 (spejder, bandit,
// crewmember/section, gøgler). This file lands the shape so the mux registration in
// cmd/api is real from the start rather than being retrofitted.
type consumer struct {
	w          cqrs.Writer
	normalizer PhoneNormalizer
}

// Consumes lists the subjects this projection subscribes to.
//
// Empty today. Note what an empty list means in practice: the projection is
// registered on the mux, the table exists, reads work and return nothing, and
// nothing errors. That silence is exactly the failure mode the registration comment
// in cmd/api/eventing.go warns about, so it is worth being explicit that it is
// intentional at this point rather than a wiring bug.
func (c consumer) Consumes() []cqrs.Subject {
	return nil
}

// HandleMessage folds one event into the read model.
//
// Must be idempotent: projections are rebuilt by replaying the stream from the
// beginning on every boot, so every event is handled again, and an event may be
// redelivered within a single run (cqrs.Consumer's contract).
func (c consumer) HandleMessage(msg cqrs.Message) error {
	// No subjects are consumed yet, so this is unreachable in practice. Returning
	// nil rather than an error is the right default for an unrecognised subject
	// anyway: a projection that errors on a message it does not care about would
	// dead-letter half the stream.
	_ = msg
	return nil
}

var _ cqrs.Consumer = consumer{}
