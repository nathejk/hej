package person

import (
	"fmt"
	"strings"
	"time"

	"github.com/jrgensen/cqrs"
)

// The portrait write path (PRD 003, task 103).
//
// # Why the event type lives HERE and not in internal/
//
// Every other event this projection consumes is defined in shared-go, because another
// service publishes it. The portrait is the first event **this app publishes itself**,
// so somebody has to own the shape. It is owned by the projection that consumes it, and
// `cmd/api` imports this package to publish — which it already does.
//
// The alternative was a struct in `internal/portrait` plus a private copy here, since
// this package may not import `internal/...` (see the package doc: it is bound for
// shared-go). Two structs that must agree on JSON tags, with nothing to catch it when
// they stop agreeing, is a worse trade than one exported type.
//
// # What is on the stream and what is not
//
// The event carries a **reference**, never bytes (PRD 008 §8): image payloads make
// replay expensive and put an opaque blob in a log optimised for small messages. The
// bytes live in the content-addressed blob store, which is consequently the only thing
// in this service that cannot be rebuilt from the log — and therefore the only thing
// that must be backed up.
//
// Content addressing is what makes the whole thing idempotent: a replay re-publishes
// the same Ref, `blob.Put` of identical bytes is a no-op, and this projection converges
// on the same row without anybody re-uploading anything.

// PortraitCaptured says that a person now has this portrait.
//
// "Captured", not "uploaded": the event records a fact about the person, not the
// success of an HTTP request.
//
// Replacing a portrait is another PortraitCaptured with a different Ref — there is no
// separate "replaced" event. The projection keeps the latest, which means the previous
// object is left in the blob store; that is deliberate and is the retention job's
// business (task 109), not this event's. A deletion, if PRD 003 ever allows one, needs
// its own event rather than an empty Ref, or a replay could not tell "deleted" from a
// malformed message.
type PortraitCaptured struct {
	PersonID string `json:"personId"`
	Year     string `json:"year"`

	// Ref is the blob store's content hash. Untrusted on the way in — see
	// blob.Ref.Valid, which this projection's handler enforces before writing.
	Ref string `json:"ref"`

	// ContentType, Bytes, Width and Height describe the stored object so a consumer
	// (PRD 007's thumbnail sync, an audit) can reason about it without fetching it.
	ContentType string `json:"contentType"`
	Bytes       int    `json:"bytes"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`

	// CapturedAt is when the photo was taken/accepted, in UTC. It is the clock the
	// retention job works from (task 109): "the portrait does not outlive the event"
	// needs a timestamp on the row, and deriving one from the message's delivery time
	// would change on every replay.
	CapturedAt time.Time `json:"capturedAt"`
}

// PortraitSubject builds the subject a portrait event is published on:
//
//	NATHEJK.<year>.portrait.<personId>.captured
//
// # Why on NATHEJK and not a sibling stream
//
// This is a small, low-frequency domain fact about a member, which is exactly what
// `NATHEJK` is for — and `NATHEJK.>` already claims the subject, so no broker topology
// change is needed (contrast task 081, where the position track's *volume* forced a
// sibling stream: 9–18× the entire domain history, replayed on every boot).
// PRD 008 §3 anticipated this: app-owned writes get "a real home", and §8's table lists
// portrait metadata as a projection of the domain log.
//
// Provenance is not lost by sharing the stream: the metatagger stamps every message this
// service publishes with producer `hej-api`.
//
// # Why per person
//
// Same reason as the track: `nats stream purge --subject` can then erase one
// individual's portrait history and nothing else.
func PortraitSubject(year, personID string) (cqrs.Subject, error) {
	if err := validSubjectToken(year, "year"); err != nil {
		return nil, err
	}
	if err := validSubjectToken(personID, "person id"); err != nil {
		return nil, err
	}
	return cqrs.SubjectFromStr(
		fmt.Sprintf("NATHEJK.%s.portrait.%s.captured", year, personID)), nil
}

// validSubjectToken rejects anything that would not survive as a single NATS subject
// token.
//
// Not cosmetic: an id containing a dot would split into extra tokens, still match
// `NATHEJK.>` and publish successfully — while quietly no longer matching the
// per-person purge pattern, making that person's portrait unerasable.
func validSubjectToken(s, what string) error {
	if s == "" {
		return fmt.Errorf("%s is empty", what)
	}
	if strings.ContainsAny(s, ". \t\r\n*>") {
		return fmt.Errorf("%s %q is not a valid subject token", what, s)
	}
	return nil
}

// validPortraitRef reports whether ref looks like a content hash from the blob store.
//
// Duplicates blob.Ref.Valid rather than calling it, because this package may not import
// `internal/...`. The check is 64 lowercase hex characters — a sha256 in hex — and it is
// here because the Ref arrives in an event body from outside this process and ends up in
// a SQL statement and later in a URL path. "../../etc/passwd" is a Ref-shaped string.
func validPortraitRef(ref string) bool {
	if len(ref) != 64 {
		return false
	}
	for _, c := range ref {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}

// handlePortraitCaptured records the person's current portrait.
//
// An UPDATE and not an upsert, matching the other handlers that react to something
// happening *to* a known person: a portrait event must not invent a row whose details
// have not arrived. On a replay the details event is applied first anyway, because the
// person had to exist for the app to accept their photo.
//
// Idempotent by construction: the same event re-applied writes the same two values.
func (c consumer) handlePortraitCaptured(msg cqrs.Message, year string) error {
	var body PortraitCaptured
	if err := msg.Body(&body); err != nil {
		return err
	}

	personID := body.PersonID
	if personID == "" {
		personID = subjectEntityID(msg.Subject())
	}
	if personID == "" {
		return fmt.Errorf("portrait captured with no personId")
	}
	if !validPortraitRef(body.Ref) {
		// Failing rather than writing it: a bad ref would put a value in the row that
		// no blob can satisfy, and every later read would degrade to "no photo" while
		// the row insisted there was one. Dead-lettering it keeps the disagreement
		// visible.
		return fmt.Errorf("portrait ref %q is not a content hash", body.Ref)
	}

	capturedAt := body.CapturedAt
	if capturedAt.IsZero() {
		// Nothing sensible to fall back to that is replay-stable — time.Now() would
		// differ on every rebuild, which is the one thing the retention job must not
		// depend on. NULL means "unknown age", and the purge job treats that as
		// purgeable rather than as immortal.
		return c.w.Consume(fmt.Sprintf(
			"UPDATE person SET portraitRef=%s, portraitCapturedAt=NULL WHERE personId=%s AND year=%s",
			quote(body.Ref), quote(personID), quote(year),
		))
	}

	return c.w.Consume(fmt.Sprintf(
		"UPDATE person SET portraitRef=%s, portraitCapturedAt=%s WHERE personId=%s AND year=%s",
		quote(body.Ref),
		quote(capturedAt.UTC().Format("2006-01-02 15:04:05")),
		quote(personID), quote(year),
	))
}
