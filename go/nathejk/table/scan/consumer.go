package scan

import (
	"fmt"
	"strings"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/messages"
)

// consumer folds scans and personnel shifts into the read model.
type consumer struct {
	w cqrs.Writer
}

// Consumes lists the subjects this projection subscribes to.
//
// **`qr.*.scanned`, not `qr.*.registered`.** The two look interchangeable and are not: `scanned` is a
// scan of a team's code at a post, while `registered` binds a code and its sheet to a team — that is a
// handover, and it belongs to the `maphandout` projection. Consuming the wrong one here would show a
// patrol its map handouts as if they were checkpoint visits.
//
// The `checkpersonnel` subjects have no `.updated`: a shift is added, its time may be specified
// separately, and it is removed. Three verbs, matching upstream.
func (c consumer) Consumes() []cqrs.Subject {
	return []cqrs.Subject{
		cqrs.SubjectFromStr("NATHEJK.*.qr.*.scanned"),
		cqrs.SubjectFromStr("NATHEJK.*.checkpersonnel.*.added"),
		cqrs.SubjectFromStr("NATHEJK.*.checkpersonnel.*.timespecified"),
		cqrs.SubjectFromStr("NATHEJK.*.checkpersonnel.*.removed"),
	}
}

// HandleMessage folds one event into the read model.
//
// Errors are annotated with the subject for the reason recorded in the person and checkpoint packages:
// the stream library logs a handler error and *drops* the message rather than dead-lettering it, so the
// log line is the only trace it existed, and a bare decode error is unattributable among tens of
// thousands of messages.
func (c consumer) HandleMessage(msg cqrs.Message) error {
	subject := msg.Subject()
	if err := c.handleMessage(msg, subject); err != nil {
		return fmt.Errorf("scan: %s: %w", subject.Subject(), err)
	}
	return nil
}

func (c consumer) handleMessage(msg cqrs.Message, subject cqrs.Subject) error {
	year := subjectYear(subject)
	if year == "" {
		return fmt.Errorf("no year in subject")
	}

	switch {
	case subject.Match("nathejk.*.qr.*.scanned"):
		return c.handleScanned(msg, year)
	case subject.Match("nathejk.*.checkpersonnel.*.added"):
		return c.handleShiftAdded(msg, year)
	case subject.Match("nathejk.*.checkpersonnel.*.timespecified"):
		return c.handleShiftTimeSpecified(msg)
	case subject.Match("nathejk.*.checkpersonnel.*.removed"):
		return c.handleShiftRemoved(msg)
	}
	return nil
}

// handleScanned records one scan.
//
// INSERT IGNORE, not an upsert: a scan is an immutable fact about a moment, so a replay must not
// rewrite it, and there is nothing to update. The (qrId, uts) key makes the second fold a no-op, which
// is what replay-safety means here.
//
// A scan with no team is still stored. It cannot appear in any patrol's list — the read filters on team
// — but discarding it would remove the only evidence a scan happened at all, and a scan nobody can
// attribute is precisely the diagnostic task 260 counts.
func (c consumer) handleScanned(msg cqrs.Message, year string) error {
	var body messages.NathejkQrScanned
	if err := msg.Body(&body); err != nil {
		return err
	}

	qrID := string(body.QrID)
	if qrID == "" {
		qrID = subjectEntityID(msg.Subject())
	}
	if qrID == "" {
		return fmt.Errorf("qr scanned with no qrId")
	}

	return c.w.Consume(fmt.Sprintf(
		"INSERT IGNORE INTO scan (qrId, uts, year, teamId, teamNumber, scannerId, latitude, longitude) "+
			"VALUES (%s, %d, %s, %s, %s, %s, %s, %s)",
		quote(qrID), msg.Time().Unix(), quote(year),
		quote(string(body.TeamID)), quote(body.TeamNumber), quote(body.ScannerID),
		quote(body.Location.Latitude), quote(body.Location.Longitude)))
}

// handleShiftAdded records a personnel shift.
//
// The time range is optional on the event, and an absent one is written as a zero window rather than
// skipped. A zero window matches no scan in the attribution join, so the shift attributes nothing —
// which is the safe direction. Guessing a window would attribute scans to a post on no evidence, and a
// wrong post name plus a wrong on-time verdict is worse for a patrol than a missing one.
func (c consumer) handleShiftAdded(msg cqrs.Message, year string) error {
	var body messages.NathejkCheckpersonnelAdded
	if err := msg.Body(&body); err != nil {
		return err
	}

	id := subjectEntityID(msg.Subject())
	if id == "" {
		return fmt.Errorf("checkpersonnel added with no id in subject")
	}

	var start, end int64
	if body.TimeRange != nil {
		start, end = body.TimeRange.Start.Unix(), body.TimeRange.End.Unix()
	}

	// An upsert rather than an INSERT: `timespecified` may arrive before `added` on a replay, and an
	// INSERT would then fail or lose the window. Only the window columns are conditional — see below.
	cols := map[string]string{
		"id":           quote(id),
		"year":         quote(year),
		"userId":       quote(string(body.UserID)),
		"checkpointId": quote(string(body.CheckpointID)),
	}
	if body.TimeRange != nil {
		cols["startUts"] = fmt.Sprintf("%d", start)
		cols["endUts"] = fmt.Sprintf("%d", end)
	}
	return c.w.Consume(upsert(cols))
}

// handleShiftTimeSpecified sets a shift's window.
//
// A separate event because upstream lets an organizer add the shift and set its hours in two steps. The
// UPDATE affects zero rows if the shift has not been seen yet, which is correct and harmless: the
// `added` event carries the window too, so the state converges either way.
func (c consumer) handleShiftTimeSpecified(msg cqrs.Message) error {
	var body messages.NathejkCheckpersonnelTimeSpecified
	if err := msg.Body(&body); err != nil {
		return err
	}

	id := subjectEntityID(msg.Subject())
	if id == "" {
		return fmt.Errorf("checkpersonnel timespecified with no id in subject")
	}

	return c.w.Consume(fmt.Sprintf(
		"UPDATE checkpersonnel SET startUts=%d, endUts=%d WHERE id=%s",
		body.Start.Unix(), body.End.Unix(), quote(id)))
}

// handleShiftRemoved deletes a shift.
//
// A hard DELETE, unlike the soft deletes elsewhere in this codebase, and deliberately: a shift is not
// an entity anything else references, nothing displays it, and a removed shift must stop attributing
// scans immediately. A `deleted` flag would mean every attribution query had to remember to filter on
// it, and the one that forgot would attribute scans to a post nobody was standing at.
//
// Note what this does *not* do: it does not un-attribute scans already made during the shift. Those
// happened. Whether a later reader still sees the attribution depends on the join, which is evaluated
// fresh each time — so removing a shift retroactively changes attribution. That is the upstream
// model's behaviour, not a choice available to us.
func (c consumer) handleShiftRemoved(msg cqrs.Message) error {
	id := subjectEntityID(msg.Subject())
	if id == "" {
		return fmt.Errorf("checkpersonnel removed with no id in subject")
	}
	return c.w.Consume(fmt.Sprintf("DELETE FROM checkpersonnel WHERE id=%s", quote(id)))
}

// subjectYear extracts the year from NATHEJK.<year>....
func subjectYear(s cqrs.Subject) string {
	parts := s.Parts()
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// subjectEntityID extracts the id, which is the part before the verb.
//
// The subject is authoritative where it disagrees with the body: that is what the stream routed on. For
// checkpersonnel it is the *only* source of the shift id — the bodies do not carry one.
func subjectEntityID(s cqrs.Subject) string {
	parts := s.Parts()
	if len(parts) < 4 {
		return ""
	}
	return parts[3]
}

// quote renders a Go string as a SQL string literal.
//
// Same reasoning as the checkpoint and person packages: cqrs.Writer takes a finished statement, not a
// statement plus arguments, so escaping is this file's responsibility rather than the driver's — and
// these values come from another service's event bodies, which is not the same as trusted input.
func quote(s string) string { return fmt.Sprintf("%q", s) }

// upsert builds an idempotent INSERT ... ON DUPLICATE KEY UPDATE for checkpersonnel.
//
// Idempotency is not optional: projections are rebuilt by replaying the stream from sequence zero on
// every boot, so every statement runs again on each start.
func upsert(cols map[string]string) string {
	if len(cols) == 0 {
		return ""
	}

	names := make([]string, 0, len(cols))
	for name := range cols {
		names = append(names, name)
	}
	sortStrings(names)

	values := make([]string, 0, len(names))
	updates := make([]string, 0, len(names))
	for _, name := range names {
		values = append(values, cols[name])
		if name == "id" {
			continue
		}
		updates = append(updates, fmt.Sprintf("%s=VALUES(%s)", name, name))
	}

	if len(updates) == 0 {
		return fmt.Sprintf("INSERT IGNORE INTO checkpersonnel (%s) VALUES (%s)",
			strings.Join(names, ", "), strings.Join(values, ", "))
	}
	return fmt.Sprintf("INSERT INTO checkpersonnel (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
		strings.Join(names, ", "), strings.Join(values, ", "), strings.Join(updates, ", "))
}

// sortStrings keeps statements deterministic, so a dead-lettered statement can be matched against the
// event that produced it.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

var _ cqrs.Consumer = consumer{}
