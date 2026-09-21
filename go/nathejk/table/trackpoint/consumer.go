package trackpoint

import (
	"fmt"

	"github.com/jrgensen/cqrs"
)

// consumer folds telemetry batches into points.
//
// Idempotent by construction rather than by care: the PK is `(year, personId, ts)`, so an upsert of a
// point that is already there writes the same values to the same row. That is what makes a replay — and a
// client retrying a batch after a timeout — converge instead of accumulating (task 083's contract).
type consumer struct {
	w cqrs.Writer
}

// reported is the published event body.
//
// **Re-declared rather than imported from `internal/track`**, because a `nathejk/table/*` package may not
// import `internal/...` — the constraint every projection here works under, so they stay liftable to
// shared-go (PRD 006). The person projection's portrait event has the same duplication for the same
// reason.
//
// The duplication is real: if `track.Reported` gains a field this will not see it. That is tolerable
// because the fields consumed here are the ones the *drawn route* needs, and a new field is a decision
// somebody makes deliberately. A test asserts the JSON tags match the publisher's, which is the part that
// would break silently.
type reported struct {
	PersonID string `json:"personId"`

	// UserType is on the event and deliberately not projected — see table.sql.
	Points []point `json:"points"`
}

type point struct {
	TS       int64   `json:"ts"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	Accuracy float64 `json:"accuracy"`
}

// Consumes lists the subjects this projection subscribes to.
//
// `TELEMETRY`, not `NATHEJK`: the position track is on its own stream, with its own retention, precisely
// so that a decision about how long to keep a minor's positions is not entangled with how long to keep
// the event log (PRD 002 §11.1, task 081).
func (c consumer) Consumes() []cqrs.Subject {
	return []cqrs.Subject{
		cqrs.SubjectFromStr("TELEMETRY.*.track.*.reported"),
	}
}

// HandleMessage folds one batch.
func (c consumer) HandleMessage(msg cqrs.Message) error {
	subject := msg.Subject()
	if err := c.handleMessage(msg, subject); err != nil {
		return fmt.Errorf("trackpoint: %s: %w", subject.Subject(), err)
	}
	return nil
}

func (c consumer) handleMessage(msg cqrs.Message, subject cqrs.Subject) error {
	if !subject.Match("telemetry.*.track.*.reported") {
		return nil
	}
	year := subjectYear(subject)
	if year == "" {
		return fmt.Errorf("no year in subject")
	}

	var body reported
	if err := msg.Body(&body); err != nil {
		return err
	}

	personID := body.PersonID
	if personID == "" {
		personID = subjectEntityID(subject)
	}
	if personID == "" {
		// Refused rather than stored: a point belonging to nobody cannot be grouped into a patrol's
		// route, so it would be permanent rubbish on a stream retained indefinitely.
		return fmt.Errorf("track reported with no personId")
	}

	// A batch with no usable points is not an error. The publisher already drops junk points and counts
	// them (task 084's `Clean`), so an empty batch here means every point in it was junk — which is a
	// thing that happens to a phone with no fix, not a fault to log.
	for _, p := range body.Points {
		if !plausible(p) {
			continue
		}
		if err := c.w.Consume(fmt.Sprintf(
			"INSERT INTO track_point SET year=%s, personId=%s, ts=%d, lat=%f, lng=%f, accuracy=%f "+
				"ON DUPLICATE KEY UPDATE lat=VALUES(lat), lng=VALUES(lng), accuracy=VALUES(accuracy)",
			quote(year), quote(personID), p.TS, p.Lat, p.Lng, p.Accuracy,
		)); err != nil {
			return err
		}
	}
	return nil
}

// plausible drops a point that cannot be a position.
//
// # Why this is here when the publisher already validates
//
// Because the publisher validated *this* year's batches, and this projection replays **everything that
// was ever published** — including whatever went onto the stream before `track.Clean` existed or if it
// ever regresses. A stream retained indefinitely is a stream that outlives its validator.
//
// Deliberately minimal: a coordinate outside the possible range, or exactly 0,0. Accuracy is **not**
// filtered — a cell-tower fix of a few kilometres is poor but real, and discarding it here would throw
// away the only evidence of where somebody was (the same call `internal/track` makes).
func plausible(p point) bool {
	if p.TS <= 0 {
		return false
	}
	if p.Lat < -90 || p.Lat > 90 || p.Lng < -180 || p.Lng > 180 {
		return false
	}
	// 0,0 is the Atlantic off Ghana and is also what some devices emit with no fix. The same reading the
	// checkpoint and album projections apply to an unset position.
	return p.Lat != 0 || p.Lng != 0
}

func subjectYear(s cqrs.Subject) string {
	parts := s.Parts()
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// subjectEntityID extracts the person id, which is the part before the verb.
func subjectEntityID(s cqrs.Subject) string {
	parts := s.Parts()
	if len(parts) < 4 {
		return ""
	}
	return parts[3]
}

// quote renders a Go string as a SQL string literal.
func quote(s string) string { return fmt.Sprintf("%q", s) }

var _ cqrs.Consumer = consumer{}
