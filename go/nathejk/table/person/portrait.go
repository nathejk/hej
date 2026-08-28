package person

import (
	"encoding/json"
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

// PortraitThumb is one smaller rendition of a portrait, stored as its own
// content-addressed object.
//
// A list of these rather than one `thumbRef`: more sizes are expected — an identification
// grid wants a different size from an avatar — and this event is on an append-only log, so
// retrofitting a list later would mean two shapes to interpret forever.
//
// Each carries its own Bytes/Width/Height, which is the point of the list. PRD 007 has to
// answer "how much storage does caching 800 faces cost?" before it downloads them, and a
// consumer that must fetch an object to learn its size cannot budget.
type PortraitThumb struct {
	// Name identifies the rendition, e.g. "thumb256", derived from its longest edge.
	// Derived rather than a label like "small", because a label needs a table somewhere
	// to say what it means and that table is what drifts from the pixels.
	Name string `json:"name"`

	// Ref is the blob store's content hash for this rendition.
	Ref string `json:"ref"`

	ContentType string `json:"contentType"`
	Bytes       int    `json:"bytes"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

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

	// ContentType, Bytes, Width and Height describe the **full** stored object, so a
	// consumer can reason about it without fetching it. Each thumbnail carries its own
	// equivalents — see Thumbs.
	ContentType string `json:"contentType"`
	Bytes       int    `json:"bytes"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`

	// Thumbs are the smaller renditions generated at upload (task 104).
	//
	// May be empty: a portrait captured before thumbnails existed has none, and so does
	// a replayed event from that era. Consumers must degrade to the full image rather
	// than treat it as a broken record.
	Thumbs []PortraitThumb `json:"thumbs"`

	// ThumbRef is the single thumbnail hash the first version of this event carried.
	//
	// DEPRECATED, and kept only for reading. Events published on 2026-08-28 before the
	// Thumbs list existed are on the log permanently, and a replay must still be able to
	// find their thumbnail — dropping this field would silently lose it. Nothing writes
	// it any more (see handlePortraitCaptured, which promotes it into Thumbs).
	//
	// Removable once no captured event predating the list remains on the stream, i.e.
	// after a purge or a fresh event store.
	ThumbRef string `json:"thumbRef,omitempty"`

	// CapturedAt is when the photo was taken/accepted, in UTC. It is the clock the
	// retention job works from (task 109): "the portrait does not outlive the event"
	// needs a timestamp on the row, and deriving one from the message's delivery time
	// would change on every replay.
	CapturedAt time.Time `json:"capturedAt"`
}

// PortraitPurged says that a person's portrait has been deleted under the retention
// policy (task 109).
//
// A separate event rather than a `PortraitCaptured` with an empty Ref: a replay must be
// able to tell "deleted" from "malformed message", and an empty-ref capture is
// indistinguishable from the latter. It also means the log records *why* the portrait went
// — retention, not replacement — which is the question anyone auditing a deletion of a
// minor's photograph will actually ask.
type PortraitPurged struct {
	PersonID string `json:"personId"`
	Year     string `json:"year"`

	// Refs are the objects that were deleted — the full image and every rendition.
	// Recorded for the audit trail, not for the projection, which simply clears the row.
	Refs []string `json:"refs"`

	// Reason is free text for the log, e.g. "retention". Deliberately not an enum: this
	// is a note to a human reading the stream a year later, and the set of reasons is
	// not something to pin down before there is a second one.
	Reason string `json:"reason"`

	PurgedAt time.Time `json:"purgedAt"`
}

// PortraitPurgeSubject builds the subject a purge is published on:
//
//	NATHEJK.<year>.portrait.<personId>.purged
//
// Same stream and same per-person shape as PortraitSubject — see that function for why.
func PortraitPurgeSubject(year, personID string) (cqrs.Subject, error) {
	if err := validSubjectToken(year, "year"); err != nil {
		return nil, err
	}
	if err := validSubjectToken(personID, "person id"); err != nil {
		return nil, err
	}
	return cqrs.SubjectFromStr(
		fmt.Sprintf("NATHEJK.%s.portrait.%s.purged", year, personID)), nil
}

// handlePortraitPurged clears the person's portrait columns.
//
// Ordering on replay takes care of itself: `captured` then `purged` in stream order
// converges on "no portrait", which is the truth. During the window in between, the row
// briefly references bytes that are already gone — which serves as a 404, i.e. "no
// photo", exactly the degradation PRD 008 §8 requires.
func (c consumer) handlePortraitPurged(msg cqrs.Message, year string) error {
	var body PortraitPurged
	if err := msg.Body(&body); err != nil {
		return err
	}

	personID := body.PersonID
	if personID == "" {
		personID = subjectEntityID(msg.Subject())
	}
	if personID == "" {
		return fmt.Errorf("portrait purged with no personId")
	}

	// Unconditional on the ref: if a newer portrait exists, its own `captured` event
	// comes *later* in the stream and re-sets these columns. Comparing refs here would
	// make the outcome depend on replay timing rather than on stream order.
	return c.w.Consume(fmt.Sprintf(
		`UPDATE person SET portraitRef="", portraitThumbRef="", portraitThumbs="", portraitCapturedAt=NULL `+
			"WHERE personId=%s AND year=%s",
		quote(personID), quote(year),
	))
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

	thumbs := body.thumbnails()
	encoded, err := encodeThumbs(thumbs)
	if err != nil {
		return err
	}

	// The smallest thumbnail is denormalized into its own column so the common read —
	// "serve this person's thumbnail" — needs no JSON parsing. Same reasoning as teamName
	// and sectionName being denormalized here: one hot value beside the set.
	defaultRef := ""
	if len(thumbs) > 0 {
		defaultRef = smallestThumb(thumbs).Ref
	}

	columns := fmt.Sprintf(
		"portraitRef=%s, portraitThumbRef=%s, portraitThumbs=%s",
		quote(body.Ref), quote(defaultRef), quote(encoded),
	)

	capturedAt := body.CapturedAt
	if capturedAt.IsZero() {
		// Nothing sensible to fall back to that is replay-stable — time.Now() would
		// differ on every rebuild, which is the one thing the retention job must not
		// depend on. NULL means "unknown age", and the purge job treats that as
		// purgeable rather than as immortal.
		return c.w.Consume(fmt.Sprintf(
			"UPDATE person SET %s, portraitCapturedAt=NULL WHERE personId=%s AND year=%s",
			columns, quote(personID), quote(year),
		))
	}

	return c.w.Consume(fmt.Sprintf(
		"UPDATE person SET %s, portraitCapturedAt=%s WHERE personId=%s AND year=%s",
		columns,
		quote(capturedAt.UTC().Format("2006-01-02 15:04:05")),
		quote(personID), quote(year),
	))
}

// thumbnails returns the event's thumbnails, tolerating both shapes the event has had.
//
// A malformed ref costs that one rendition, not the portrait: readers fall back to the
// full image, whereas failing the event would lose the photo over a secondary artefact.
func (p PortraitCaptured) thumbnails() []PortraitThumb {
	out := make([]PortraitThumb, 0, len(p.Thumbs)+1)
	for _, t := range p.Thumbs {
		if !validPortraitRef(t.Ref) {
			continue
		}
		if t.Name == "" {
			// A rendition nothing can ask for by name is still worth keeping, because the
			// purge has to know its ref exists. Named by its own size so it is at least
			// addressable.
			t.Name = fmt.Sprintf("thumb%d", maxInt(t.Width, t.Height))
		}
		out = append(out, t)
	}

	// The deprecated single-ref shape (see ThumbRef). Promoted into the list so the rest
	// of the code has exactly one thing to understand. Dimensions are unknown for these,
	// which is precisely the gap the list closed — a consumer sees zeros and knows it must
	// fetch to find out.
	if len(out) == 0 && validPortraitRef(p.ThumbRef) {
		out = append(out, PortraitThumb{Name: "thumb", Ref: p.ThumbRef, ContentType: "image/jpeg"})
	}
	return out
}

// encodeThumbs renders the set for storage.
//
// JSON in a column rather than a side table, deliberately. The set is small, always read
// with its person, and written only by this one event, so a table would add a join and a
// second delete-on-replace path for no read this app makes. **If something ever needs to
// query across renditions** — "total thumbnail bytes for the year" — that is the moment to
// normalize it, and the JSON is a faithful record to migrate from.
func encodeThumbs(thumbs []PortraitThumb) (string, error) {
	if len(thumbs) == 0 {
		// Empty string rather than "[]", so "no thumbnails" reads the same in SQL as it
		// does for a row that predates the column.
		return "", nil
	}
	encoded, err := json.Marshal(thumbs)
	if err != nil {
		return "", fmt.Errorf("encode portrait thumbnails: %w", err)
	}
	return string(encoded), nil
}

// smallestThumb returns the rendition with the smallest longest edge.
//
// "Smallest" rather than "first": the default is served where a thumbnail is wanted, and
// the cheapest one is the right default. A rendition with unknown dimensions (the
// deprecated shape) sorts last rather than winning by comparing zero.
func smallestThumb(thumbs []PortraitThumb) PortraitThumb {
	best := thumbs[0]
	bestEdge := maxInt(best.Width, best.Height)
	for _, t := range thumbs[1:] {
		edge := maxInt(t.Width, t.Height)
		if bestEdge == 0 || (edge > 0 && edge < bestEdge) {
			best, bestEdge = t, edge
		}
	}
	return best
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
