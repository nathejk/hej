// Package track validates position batches and names the subject they are published
// on (PRD 002 §11.1, task 084).
//
// It is deliberately separate from the HTTP handler. What is worth getting right here —
// which points are junk, and what a subject looks like — is worth testing without a
// request, a session or a broker.
package track

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jrgensen/cqrs"
)

// MaxPointsPerBatch bounds one request.
//
// 2,000 is chosen against the worst realistic case rather than picked round: at 30 s
// sampling a full 12-hour race is ~1,440 points, so a participant who was offline for the
// entire event still ships their backlog in one request. A batch is ~60 bytes a point, so
// this is ~120 KB — comfortably inside the 1 MiB body cap the transport already enforces.
const MaxPointsPerBatch = 2000

// The window a timestamp must fall in to be believable.
//
// A phone with a wrong clock is a real occurrence, not a hypothetical, and this stream is
// retained indefinitely — so a point stamped 1970 or 2049 would be permanent junk sitting
// in the middle of someone's route.
var (
	earliestPlausible = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	maxClockSkew      = 24 * time.Hour
)

// maxAccuracyMetres rejects a fix that is not a position at all. A cell-tower fix of a few
// kilometres is poor but real and is kept — 086 can filter on accuracy, and discarding it
// here would throw away the only evidence of where someone was. 100 km is not a fix.
const maxAccuracyMetres = 100_000

// ErrEmptyBatch is returned when a request carries no points at all. Distinct from "every
// point was junk", which is not an error — see Clean.
var ErrEmptyBatch = errors.New("no points in batch")

// ErrBatchTooLarge is returned when a batch exceeds MaxPointsPerBatch.
var ErrBatchTooLarge = fmt.Errorf("batch exceeds %d points", MaxPointsPerBatch)

// Point is one recorded position. Field names and units match what the client stores in
// IndexedDB (task 082), so nothing is converted on the way through.
type Point struct {
	// TS is epoch milliseconds, and is half the identity of a point.
	//
	// Milliseconds-since-epoch rather than RFC 3339 deliberately: `(person, timestamp)`
	// identifies a point, so the timestamp is a key, and an integer cannot be
	// re-serialised into a different-but-equal form the way a formatted date can. A
	// consumer comparing "2026-08-27T15:24:16.954Z" with "2026-08-27T15:24:16.954000Z"
	// would see two points where there is one.
	TS int64 `json:"ts"`

	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`

	// Accuracy is the radius in metres the browser reported.
	Accuracy float64 `json:"accuracy"`
}

// Reported is the published event body.
//
// The person and year are included even though the subject already carries them: a
// consumer should not have to parse a subject to read a message, and a subject is a
// routing address rather than a payload.
type Reported struct {
	PersonID string `json:"personId"`

	// UserType is the app role the person held when the batch was reported.
	//
	// Every message published to the telemetry stream carries one, and it is stamped at
	// publish time rather than looked up at read time on purpose: the stream is retained
	// indefinitely, while roles change — a spejder becomes a bandit, crew get reclassified
	// — so a consumer joining against today's directory would silently reinterpret last
	// year's history. It is also what lets a reader filter or scope by population without
	// a lookup back into this service, which for a stream read once per team after a race
	// is the difference between a query and a fan-out.
	//
	// It is not in the subject: the subject is keyed per person because that is the
	// erasure unit (see Subject), and a mutable attribute in a routing address would break
	// the per-person purge the moment it changed.
	UserType string `json:"userType"`

	Year   string  `json:"year"`
	Points []Point `json:"points"`
}

// Clean drops points that must not be persisted and returns the rest.
//
// DROPPING RATHER THAN REJECTING THE BATCH is the important decision here, and it is not
// leniency for its own sake. The client retries a batch until the server accepts it (task
// 083), so rejecting a whole batch over one bad point would put a member's entire track
// behind a poison pill: the same request would fail forever and every later point would
// queue up behind it. One junk point costs one junk point.
//
// The caller reports the dropped count back to the client, so this is visible rather than
// silent — a recorder producing junk shows up as a number instead of as missing data.
//
// now is injected so the plausibility window is testable.
func Clean(points []Point, now time.Time) (kept []Point, dropped int) {
	kept = make([]Point, 0, len(points))
	for _, p := range points {
		if plausible(p, now) {
			kept = append(kept, p)
			continue
		}
		dropped++
	}
	return kept, dropped
}

func plausible(p Point, now time.Time) bool {
	// NaN and ±Inf cannot survive JSON decoding into a float64, so this is belt and
	// braces — but it is one comparison, and a NaN coordinate would propagate into every
	// consumer that averages or bounds-checks a route.
	if math.IsNaN(p.Lat) || math.IsNaN(p.Lng) || math.IsNaN(p.Accuracy) ||
		math.IsInf(p.Lat, 0) || math.IsInf(p.Lng, 0) || math.IsInf(p.Accuracy, 0) {
		return false
	}
	if p.TS == 0 {
		return false
	}
	ts := time.UnixMilli(p.TS)
	if ts.Before(earliestPlausible) || ts.After(now.Add(maxClockSkew)) {
		return false
	}
	if p.Lat < -90 || p.Lat > 90 || p.Lng < -180 || p.Lng > 180 {
		return false
	}
	// Exactly (0, 0) is Null Island: the signature of a failed fix reported as a
	// successful one, and roughly 5,000 km from any Nathejk. Every other coordinate is
	// taken at face value — guessing at "plausible for Denmark" would eventually discard
	// real data for a boundary someone drew by hand.
	if p.Lat == 0 && p.Lng == 0 {
		return false
	}
	if p.Accuracy < 0 || p.Accuracy > maxAccuracyMetres {
		return false
	}
	return true
}

// Subject builds the per-person telemetry subject agreed in task 081:
//
//	TELEMETRY.<year>.track.<personId>.reported
//
// Per person because that is the erasure mechanism: `nats stream purge --subject` can
// remove one individual's track and nothing else (PRD 002 §11.1).
//
// The person id becomes a subject token, so it is validated rather than trusted. An id
// containing a dot would silently split into extra tokens — still matching `TELEMETRY.>`,
// so the publish would succeed — and the per-person purge pattern would then no longer
// match it, quietly making that person's track unerasable. Ids are UUIDs in practice; this
// is here so that remains true.
func Subject(year, personID string) (cqrs.Subject, error) {
	if err := validToken(year, "year"); err != nil {
		return nil, err
	}
	if err := validToken(personID, "person id"); err != nil {
		return nil, err
	}
	return cqrs.SubjectFromStr(fmt.Sprintf("TELEMETRY.%s.track.%s.reported", year, personID)), nil
}

// validToken rejects anything that would not survive as a single NATS subject token.
func validToken(s, what string) error {
	if s == "" {
		return fmt.Errorf("%s is empty", what)
	}
	// '.' splits tokens; '*' and '>' are wildcards; whitespace is not permitted in a
	// subject at all (stream/subject.FromStr returns an empty subject for it).
	if strings.ContainsAny(s, ". \t\r\n*>") {
		return fmt.Errorf("%s %q is not a valid subject token", what, s)
	}
	return nil
}
