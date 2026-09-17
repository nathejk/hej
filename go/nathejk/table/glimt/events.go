package glimt

import (
	"time"
)

// The Glimt event shapes (PRD 019 §8, task 301).
//
// # Why they live here
//
// These events are published by **this app**, not by another service, so somebody has to own the
// shape. The portrait established the convention (see person/portrait.go): the type is owned by the
// projection that consumes it, and cmd/api imports this package to publish. The alternative — a
// struct in internal/ plus a private copy here, because this package may not import internal/... —
// is two structs that must agree on JSON tags with nothing to catch it when they stop.
//
// # References, never bytes
//
// Every media field is a content hash into the blob store (PRD 008 §8). Putting image or video
// bytes on the log would make replay ruinous and, for 30-second video, absurd. Content addressing
// is also what makes the fold idempotent: a replay re-publishes the same refs and the projection
// converges on the same rows.
//
// # One event per fact, never a sentinel
//
// There is a separate event for created, deleted, reported, hidden, unhidden and purged rather than
// one "changed" event with nullable fields. The reason is the same one portrait.go gives for not
// encoding a deletion as an empty ref: a replay must be able to tell a deliberate takedown from a
// malformed message, and the log should record *why* a child's photograph disappeared — which is
// the question anyone auditing it will actually ask.

// Media is one item in a glimt, as stored.
//
// Carries its own dimensions and byte count so a consumer can reason about it — and budget cache
// space for it — without fetching it. The post-race browse downloads thumbnails by the thousand,
// and a client that must fetch an object to learn its size cannot decide what to keep.
type Media struct {
	// Ordinal is the position the author arranged this item in, and therefore the order it must
	// be displayed in. Explicit rather than implied by array index, because the array is JSON on
	// a permanent log and a reordering bug would otherwise be invisible.
	Ordinal int `json:"ordinal"`

	// Ref is the blob store's content hash for the full stored item. Untrusted on the way in —
	// the consumer validates it with blob.Ref.Valid before writing, as the portrait fold does.
	Ref string `json:"ref"`

	// ThumbRef is the thumbnail. May be "" when one could not be produced; the client falls back
	// to the full item rather than rendering a gap.
	ThumbRef string `json:"thumbRef,omitempty"`

	// Kind is MediaKindImage or MediaKindVideo.
	Kind string `json:"kind"`

	ContentType string `json:"contentType"`
	Bytes       int    `json:"bytes"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`

	// DurationMs is 0 for a still. For video it is measured from the parsed container rather
	// than believed from the client (task 322) — the 30-second cap is a server-side rule, and a
	// recorder UI's own limit is not a limit.
	DurationMs int `json:"durationMs,omitempty"`
}

// Media kinds. Strings rather than an int enum because they travel on a permanent log and in JSON
// responses, where a number would need a table somewhere to say what it means.
const (
	MediaKindImage = "image"
	MediaKindVideo = "video"
)

// Created says a participant shared a moment.
//
// The attribution fields are **frozen copies**, not references (PRD 019 §6, task 302). Resolving
// them at read time would make a glimt's caption change when a hold is renamed or when the author
// moves to another patrulje, which is precisely what "a moment" must not do.
type Created struct {
	GlimtID string `json:"glimtId"`
	Year    string `json:"year"`

	// AuthorPersonID owns the glimt: only they may delete it. It is on the event because
	// ownership and moderation need it — not so it can be displayed. Every response but the
	// moderation queue projects it out.
	AuthorPersonID string `json:"authorPersonId"`

	// AuthorGroup is spejder, bandit or crew: who a `group`-scoped glimt reaches.
	AuthorGroup string `json:"authorGroup"`

	// TeamNumber and TeamName are the **hold's** — a patrulje or klan (PRD 019 §0b). Nothing to
	// do with the Team section that moderates. Number is "" for crew, who have a section rather
	// than a numbered hold.
	TeamNumber string `json:"teamNumber,omitempty"`
	TeamName   string `json:"teamName,omitempty"`

	// Audience is group | nathejk | public, chosen once. There is no event that changes it:
	// widening after the fact would retroactively expose a photo shared under a narrower
	// promise, so deleting and reposting is the honest path (PRD 019 §6).
	Audience string `json:"audience"`

	Caption string `json:"caption"`

	// Media in the author's order, at least one.
	Media []Media `json:"media"`

	// CreatedAt is when the author posted, in UTC. Stored rather than derived from delivery
	// time because retention (task 310) works from it, and delivery time changes on every
	// replay — a purge window measured against it would move.
	CreatedAt time.Time `json:"createdAt"`
}

// Deleted says the author destroyed their own glimt.
//
// Only ever the author: moderators hide (see Hidden), authors delete. That asymmetry is the whole
// difference between the two events, and it is why this one carries the refs — a delete purges
// blobs, a hide never does.
type Deleted struct {
	GlimtID string `json:"glimtId"`
	Year    string `json:"year"`

	// Refs are the objects deleted, recorded for the audit trail rather than for the
	// projection, which simply tombstones the row.
	Refs []string `json:"refs,omitempty"`

	// DeletedAt is when, in UTC.
	DeletedAt time.Time `json:"deletedAt"`
}

// Reported says a member flagged a glimt.
//
// The projection hides it in the same fold. That is not an optimisation: the public scope publishes
// with no approval queue in front of it (PRD 019 §0), so a report is the only fast mechanism there
// is, and leaving a window between "reported" and "hidden" would be a window in which the thing
// somebody objected to is still on the open web.
type Reported struct {
	GlimtID string `json:"glimtId"`
	Year    string `json:"year"`

	// ReporterPersonID is who reported it. Recorded so one person cannot inflate the count by
	// tapping twice, and so abuse of the report button is traceable.
	ReporterPersonID string `json:"reporterPersonId"`

	// Reason is free text, for a human reading the queue.
	Reason string `json:"reason,omitempty"`

	ReportedAt time.Time `json:"reportedAt"`
}

// Hidden says the Team section took a glimt down (PRD 019 §0).
//
// Distinct from Deleted in the one way that matters: the media survives, so Unhidden can put it
// back. A moderator who wanted it gone forever would need the author, which is deliberate — the
// person who took the photograph is the only one who gets to destroy it.
type Hidden struct {
	GlimtID string `json:"glimtId"`
	Year    string `json:"year"`

	// HiddenBy is the Team-section member who did it. On the event because a takedown of a
	// participant's photo should be attributable to a person, not to "the system".
	HiddenBy string `json:"hiddenBy"`

	Reason   string    `json:"reason,omitempty"`
	HiddenAt time.Time `json:"hiddenAt"`
}

// Unhidden restores a glimt a report or a moderator had taken down.
//
// Its existence is what makes hiding cheap enough to be the default response to a report. If a
// takedown were irreversible, the safe reaction to an ambiguous report would be to wait for a
// human — and waiting is the thing the public scope cannot afford.
type Unhidden struct {
	GlimtID string `json:"glimtId"`
	Year    string `json:"year"`

	// UnhiddenBy is the Team-section member who restored it.
	UnhiddenBy string `json:"unhiddenBy"`

	Reason     string    `json:"reason,omitempty"`
	UnhiddenAt time.Time `json:"unhiddenAt"`
}

// Purged says retention deleted a glimt (task 310).
//
// A separate event from Deleted for the same reason PortraitPurged is separate: the log should say
// whether a photograph went because its owner removed it or because the retention window closed.
// Those are different facts about the same absence, and only one of them is a decision somebody
// made about that photo.
type Purged struct {
	GlimtID string `json:"glimtId"`
	Year    string `json:"year"`

	Refs []string `json:"refs,omitempty"`

	// Reason is free text for a human reading the stream a year later, e.g. "retention".
	Reason   string    `json:"reason,omitempty"`
	PurgedAt time.Time `json:"purgedAt"`
}
