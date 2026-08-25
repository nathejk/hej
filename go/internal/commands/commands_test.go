package commands

import (
	"errors"
	"testing"

	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/cqrs/cqrstest"
)

const (
	// The canonical string form accepts ':' or '.' between domain and type
	// (cqrs.SubjectFromStr), but parsing normalises it to '.'. Both forms are
	// spelled out here because the difference is invisible until you assert on a
	// round-tripped subject, as this test does.
	testSubject     = "NATHEJK:2026.member.abc123.verified"
	testSubjectWire = "NATHEJK.2026.member.abc123.verified"
)

// A facade with no publisher must refuse rather than silently succeed. This is the
// asymmetry PRD 008 §5 draws: reads survive a broker outage, writes must not
// pretend to.
func TestPublishWithoutPublisher(t *testing.T) {
	c := New(NewPublisherHolder())

	if c.Available() {
		t.Fatal("want Available() == false with no publisher")
	}
	err := c.Publish(cqrs.SubjectFromStr(testSubject), map[string]string{"a": "b"})
	if !errors.Is(err, ErrNoPublisher) {
		t.Fatalf("want ErrNoPublisher, got %v", err)
	}
}

// A nil holder must behave like an empty one rather than panic: it is what a
// handler ends up with when the service runs with no database at all.
func TestPublishWithNilHolder(t *testing.T) {
	c := New(nil)

	if c.Available() {
		t.Fatal("want Available() == false with a nil holder")
	}
	if err := c.Publish(cqrs.SubjectFromStr(testSubject), struct{}{}); !errors.Is(err, ErrNoPublisher) {
		t.Fatalf("want ErrNoPublisher, got %v", err)
	}
}

// The point of the holder: a publisher installed *after* handlers were wired must
// start working without re-wiring. This is the broker-comes-up-late case that task
// 058's background connect creates.
func TestPublisherArrivingLate(t *testing.T) {
	holder := NewPublisherHolder()
	c := New(holder)

	if err := c.Publish(cqrs.SubjectFromStr(testSubject), struct{}{}); !errors.Is(err, ErrNoPublisher) {
		t.Fatalf("want ErrNoPublisher before connect, got %v", err)
	}

	pub := &cqrstest.Publisher{}
	holder.Set(pub)

	if !c.Available() {
		t.Fatal("want Available() == true once a publisher is installed")
	}
	if err := c.Publish(cqrs.SubjectFromStr(testSubject), struct{}{}); err != nil {
		t.Fatalf("Publish after late connect: %v", err)
	}
	if len(pub.Messages) != 1 {
		t.Fatalf("want 1 message, got %d", len(pub.Messages))
	}

	// Clearing must return to refusing, not keep a stale publisher.
	holder.Set(nil)
	if c.Available() {
		t.Fatal("want Available() == false after the holder is cleared")
	}
}

func TestPublishSendsOneMessageOnTheGivenSubject(t *testing.T) {
	pub := &cqrstest.Publisher{}
	holder := NewPublisherHolder()
	holder.Set(pub)
	c := New(holder)

	if !c.Available() {
		t.Fatal("want Available() == true with a publisher")
	}
	if err := c.Publish(cqrs.SubjectFromStr(testSubject), map[string]string{"phone": "1122"}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if len(pub.Messages) != 1 {
		t.Fatalf("want 1 published message, got %d", len(pub.Messages))
	}
	if got := pub.Messages[0].Subject().Subject(); got != testSubjectWire {
		t.Fatalf("want subject %q, got %q", testSubjectWire, got)
	}
	if got := pub.Messages[0].Subject().Domain(); got != "NATHEJK" {
		t.Fatalf("want domain NATHEJK, got %q", got)
	}
}

// A publish failure must reach the caller: the handler has to be able to fail the
// request rather than report success for an event that never landed.
func TestPublishPropagatesPublisherError(t *testing.T) {
	wantErr := errors.New("broker gone")
	pub := &cqrstest.Publisher{Err: wantErr}
	holder := NewPublisherHolder()
	holder.Set(pub)
	c := New(holder)

	if err := c.Publish(cqrs.SubjectFromStr(testSubject), struct{}{}); !errors.Is(err, wantErr) {
		t.Fatalf("want the publisher error, got %v", err)
	}
}

// An unmarshallable body must fail instead of publishing an event with no body:
// a subscriber cannot distinguish that from one with no fields set.
func TestPublishRejectsUnmarshallableBody(t *testing.T) {
	pub := &cqrstest.Publisher{}
	holder := NewPublisherHolder()
	holder.Set(pub)
	c := New(holder)

	// Channels cannot be JSON-encoded.
	err := c.Publish(cqrs.SubjectFromStr(testSubject), make(chan int))
	if err == nil {
		t.Fatal("want an error for an unmarshallable body")
	}
	if errors.Is(err, ErrNoPublisher) {
		t.Fatalf("want a marshalling error, got ErrNoPublisher")
	}
	if len(pub.Messages) != 0 {
		t.Fatalf("nothing should have been published, got %d messages", len(pub.Messages))
	}
}
