package main

import (
	"time"

	"github.com/jrgensen/cqrs"

	"nathejk.dk/internal/commands"
)

// lazyPublisher is a cqrs.Publisher that resolves the real one per call, through
// the PublisherHolder.
//
// # Why this exists
//
// The broker is connected in the **background** (PRD 008 §6): startup must not block
// on it. So at the moment entities are constructed in main.go there is no publisher
// yet, and `ev.publisherOrNil()` returns nil — not "nil for now", but nil *forever*,
// because the value is captured once at wiring time and the entity keeps it.
//
// That is what `commands.PublisherHolder` was introduced for, and handlers using
// `app.commands` have always gone through it. A shared-go entity cannot: its
// constructor takes a `cqrs.Publisher` and stores it. Handing it a bare
// `publisherOrNil()` had two consequences, and the second is worse than the first:
//
//  1. `commander.Register` called `c.p.MessageFunc()` on a nil interface and
//     panicked — a 500 with a stack trace instead of an answer (task 247).
//  2. It could never recover. Construction always precedes connection, so the
//     captured nil is permanent: vehicle registration would have stayed broken for
//     the life of the process, and restarting would not have helped either.
//
// Wrapping the holder fixes both, and means writes start working the moment the
// broker arrives, with no re-wiring — the same property `app.commands` already has.
type lazyPublisher struct {
	holder *commands.PublisherHolder
}

// Publish sends the message, or fails with ErrNoPublisher when no broker has
// arrived yet.
//
// Failing is the required behaviour, not a fallback: a write that could not be
// published has not happened (PRD 008 §5), so this must never report success.
func (l lazyPublisher) Publish(msg cqrs.Message) error {
	p := l.holder.Get()
	if p == nil {
		return commands.ErrNoPublisher
	}
	return p.Publish(msg)
}

// MessageFunc returns the live publisher's message constructor, or one that builds
// a message Publish will refuse.
//
// The fallback is not decoration. A caller does `p.MessageFunc()(subject)` and then
// `p.Publish(msg)`, so returning nil here would move the panic rather than remove
// it — including in the genuinely reachable case where the broker drops between the
// handler's availability check and the publish.
func (l lazyPublisher) MessageFunc() cqrs.MessageFunc {
	if p := l.holder.Get(); p != nil {
		return p.MessageFunc()
	}
	return func(subject cqrs.Subject) cqrs.MutableMessage {
		return &unpublishableMessage{subject: subject}
	}
}

var _ cqrs.Publisher = lazyPublisher{}

// unpublishableMessage is a message that exists only to be refused.
//
// It carries a subject and swallows a body so the command that built it can reach
// its Publish call and get ErrNoPublisher back, instead of dereferencing nil. It
// never reaches a broker: the only Publish that accepts it is lazyPublisher's, and
// that one returns the error.
//
// Deliberately not `streamtest.Message`: that lives in a test package, and product
// code importing it would ship a test double into production for real.
type unpublishableMessage struct {
	subject cqrs.Subject
}

func (m *unpublishableMessage) Subject() cqrs.Subject     { return m.subject }
func (m *unpublishableMessage) Time() time.Time           { return time.Time{} }
func (m *unpublishableMessage) Sequence() uint64          { return 0 }
func (m *unpublishableMessage) Body(any) error            { return commands.ErrNoPublisher }
func (m *unpublishableMessage) Meta(any) error            { return commands.ErrNoPublisher }
func (m *unpublishableMessage) RawBody() any              { return nil }
func (m *unpublishableMessage) RawMeta() any              { return nil }
func (m *unpublishableMessage) SetSubject(s cqrs.Subject) { m.subject = s }
func (m *unpublishableMessage) SetBody(any) error         { return nil }
func (m *unpublishableMessage) SetMeta(any) error         { return nil }
func (m *unpublishableMessage) SetTime(time.Time) error   { return nil }
