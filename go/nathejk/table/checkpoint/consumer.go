package checkpoint

import (
	"fmt"
	"strings"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/messages"
)

// consumer folds checkpoint events into the read model.
type consumer struct {
	w cqrs.Writer
}

// Consumes lists the subjects this projection subscribes to.
//
// Only `.updated`. The matching `.created` events were checked against the live stream and
// carry nothing this projection wants — no name, no position, just the id and the
// checkgroup it belongs to (`NathejkCheckpointCreated`). Subscribing to them would add a
// message family that can only ever write an empty row.
func (c consumer) Consumes() []cqrs.Subject {
	return []cqrs.Subject{
		cqrs.SubjectFromStr("NATHEJK.*.checkpoint.*.updated"),
		cqrs.SubjectFromStr("NATHEJK.*.checkpoint.*.deleted"),
	}
}

// HandleMessage folds one event into the read model.
//
// Errors are annotated with the subject for the reason recorded in the person package: the
// stream library logs a handler error and *drops* the message rather than dead-lettering it,
// so the log line is the only trace it existed, and a bare decode error is unattributable
// among tens of thousands of messages.
func (c consumer) HandleMessage(msg cqrs.Message) error {
	subject := msg.Subject()
	if err := c.handleMessage(msg, subject); err != nil {
		return fmt.Errorf("checkpoint: %s: %w", subject.Subject(), err)
	}
	return nil
}

func (c consumer) handleMessage(msg cqrs.Message, subject cqrs.Subject) error {
	year := subjectYear(subject)
	if year == "" {
		return fmt.Errorf("no year in subject")
	}

	switch {
	case subject.Match("nathejk.*.checkpoint.*.updated"):
		return c.handleUpdated(msg, year)
	case subject.Match("nathejk.*.checkpoint.*.deleted"):
		return c.handleDeleted(msg, year)
	}
	return nil
}

// handleUpdated writes whatever the event carries.
//
// Every field on NathejkCheckpointUpdated is a **pointer**, which is the whole shape of this
// handler: the message expresses a partial update, so a nil field means "unchanged" and not
// "cleared". Writing all columns unconditionally would let an event that only renames a
// checkpoint erase its position — and since the race area is derived from positions, that
// would silently shrink the cached region.
//
// So only the columns actually present are written, which the upsert already supports.
func (c consumer) handleUpdated(msg cqrs.Message, year string) error {
	var body messages.NathejkCheckpointUpdated
	if err := msg.Body(&body); err != nil {
		return err
	}

	id := string(body.CheckpointID)
	if id == "" {
		id = subjectEntityID(msg.Subject())
	}
	if id == "" {
		return fmt.Errorf("checkpoint updated with no checkpointId")
	}

	cols := map[string]string{
		"checkpointId": quote(id),
		"year":         quote(year),
		// An update after a delete restores the checkpoint: the last event wins, as in
		// `person`.
		"deleted": "0",
	}
	if body.Name != nil {
		cols["name"] = quote(*body.Name)
	}
	if body.Position != nil {
		// 0,0 is not a position on this event — it is the Atlantic off Ghana, and it is
		// what an unset coordinate serialises to. Treating it as absent keeps a
		// placeholder from dragging the convex hull across two continents, which is the
		// single worst thing that can happen to the derived area.
		if body.Position.Latitude == 0 && body.Position.Longitude == 0 {
			cols["latitude"] = "NULL"
			cols["longitude"] = "NULL"
		} else {
			cols["latitude"] = formatFloat(body.Position.Latitude)
			cols["longitude"] = formatFloat(body.Position.Longitude)
		}
	}

	return c.w.Consume(upsert(cols))
}

// handleDeleted soft-deletes a checkpoint.
//
// No INSERT: a delete for a checkpoint never seen is a no-op affecting zero rows, which is
// correct and idempotent. No `.deleted` events exist on the stream today — the subject is
// subscribed because a checkpoint being removed must not leave a phantom point stretching
// the race area, and finding that out during an event would be worse than a handler that
// never fires.
func (c consumer) handleDeleted(msg cqrs.Message, year string) error {
	var body messages.NathejkCheckpointCreated // carries checkpointId; shape is shared
	_ = msg.Body(&body)

	id := string(body.CheckpointID)
	if id == "" {
		id = subjectEntityID(msg.Subject())
	}
	if id == "" {
		return fmt.Errorf("checkpoint delete with no checkpointId")
	}

	return c.w.Consume(fmt.Sprintf(
		"UPDATE checkpoint SET deleted=1 WHERE checkpointId=%s AND year=%s",
		quote(id), quote(year),
	))
}

// subjectYear extracts the year from NATHEJK.<year>.checkpoint.<id>.<verb>.
func subjectYear(s cqrs.Subject) string {
	parts := s.Parts()
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// subjectEntityID extracts the id, which is the part before the verb.
func subjectEntityID(s cqrs.Subject) string {
	parts := s.Parts()
	if len(parts) < 4 {
		return ""
	}
	return parts[3]
}

var _ cqrs.Consumer = consumer{}

// quote renders a Go string as a SQL string literal.
//
// Same reasoning as the person package: cqrs.Writer takes a finished statement, not a
// statement plus arguments, so escaping is this file's responsibility rather than the
// driver's — and these values come from another service's event bodies, which is not the
// same as trusted input.
func quote(s string) string { return fmt.Sprintf("%q", s) }

// formatFloat renders a coordinate as a SQL literal.
//
// %v rather than a fixed precision: a coordinate truncated to a few decimals would still be
// fine for a 3 km buffer, but there is no reason to lose precision on the way into a DOUBLE
// column, and rounding here would make the stored value differ from the event for no gain.
func formatFloat(f float64) string { return fmt.Sprintf("%v", f) }

// upsert builds an idempotent INSERT ... ON DUPLICATE KEY UPDATE.
//
// Idempotency is not optional: projections are rebuilt by replaying the stream from sequence
// zero on every boot, so every statement runs again on each start.
//
// Only the columns a given event carries are written, which is what makes partial updates
// safe — see handleUpdated.
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
		if name == "checkpointId" || name == "year" {
			continue
		}
		updates = append(updates, fmt.Sprintf("%s=VALUES(%s)", name, name))
	}

	if len(updates) == 0 {
		return fmt.Sprintf("INSERT IGNORE INTO checkpoint (%s) VALUES (%s)",
			strings.Join(names, ", "), strings.Join(values, ", "))
	}
	return fmt.Sprintf("INSERT INTO checkpoint (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
		strings.Join(names, ", "), strings.Join(values, ", "), strings.Join(updates, ", "))
}

// sortStrings keeps statements deterministic, so a dead-lettered statement can be matched
// against the event that produced it.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
