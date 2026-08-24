// Package scans exposes a patrol's event registrations — checkpoint scans and
// bandit catches — as a read-only source. Only a seeded mock ships today; the
// real source (Nathejk records / a jetstream projection of scan events) will
// replace it behind this interface without touching handlers.
package scans

import "time"

// Kind distinguishes the two registration types a patrol accumulates during an
// event. The values are part of the API contract (see cmd/api/scans.go).
type Kind string

const (
	// KindCheckpoint is a scan at a manned post.
	KindCheckpoint Kind = "checkpoint"
	// KindBandit is a bandit catch.
	KindBandit Kind = "bandit"
)

// Scan is a single registration.
type Scan struct {
	ID    string
	Kind  Kind
	Label string

	// Lat/Lng are nil when the registration carries no position — a post can
	// register a patrol manually, and such a scan is listable but not plottable.
	Lat *float64
	Lng *float64

	ScannedAt time.Time
}

// Source returns a patrol's registrations. Callers pass the patrol id resolved
// from the session; an unknown or empty patrol yields no scans rather than an
// error, because having no patrol is a legitimate state for personnel users.
type Source interface {
	// ByPatrol returns the patrol's registrations, newest first.
	ByPatrol(patrolID string) []Scan
}
