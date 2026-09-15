package checkgroup

import (
	"fmt"
	"strings"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/messages"
)

// consumer folds checkgroup events into the read model.
type consumer struct {
	w cqrs.Writer
}

// Consumes lists the subjects this projection subscribes to.
//
// Note the **plural** on the sorted subject: `checkgroups.sorted`, not `checkgroup.sorted`. The
// checkpoint reorder is spelled differently again — `checkgroup.{id}.checkpoints_sorted` — so the two
// are not variations of one convention and neither can be guessed from the other. Both were confirmed
// against the live subjects rather than inferred.
//
// No `.created`: unlike a checkpoint's, a checkgroup create carries nothing this projection wants, and
// an update always follows. If that changes, the checkpoint package shows what adding it looks like.
func (c consumer) Consumes() []cqrs.Subject {
	return []cqrs.Subject{
		cqrs.SubjectFromStr("NATHEJK.*.checkgroup.*.updated"),
		cqrs.SubjectFromStr("NATHEJK.*.checkgroup.*.deleted"),
		cqrs.SubjectFromStr("NATHEJK.*.checkgroups.sorted"),
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
		return fmt.Errorf("checkgroup: %s: %w", subject.Subject(), err)
	}
	return nil
}

func (c consumer) handleMessage(msg cqrs.Message, subject cqrs.Subject) error {
	year := subjectYear(subject)
	if year == "" {
		return fmt.Errorf("no year in subject")
	}

	switch {
	case subject.Match("nathejk.*.checkgroups.sorted"):
		return c.handleSorted(msg, year)
	case subject.Match("nathejk.*.checkgroup.*.updated"):
		return c.handleUpdated(msg, year)
	case subject.Match("nathejk.*.checkgroup.*.deleted"):
		return c.handleDeleted(msg, year)
	}
	return nil
}

// handleUpdated writes whatever the event carries.
//
// `NathejkCheckgroupUpdated` is a **patch**: every field is a pointer and nil means "unchanged", not
// "cleared" — the same shape as the checkpoint update, and the same hazard. Writing all columns
// unconditionally would let an event that only renames a group erase its `scheme`, which would silently
// turn every on-time verdict at that group's posts into "no verdict". Nothing about the app would look
// broken; the badges would simply stop appearing.
func (c consumer) handleUpdated(msg cqrs.Message, year string) error {
	var body messages.NathejkCheckgroupUpdated
	if err := msg.Body(&body); err != nil {
		return err
	}

	id := string(body.CheckgroupID)
	if id == "" {
		id = subjectEntityID(msg.Subject())
	}
	if id == "" {
		return fmt.Errorf("checkgroup updated with no checkgroupId")
	}

	cols := map[string]string{
		"checkgroupId": quote(id),
		"year":         quote(year),
		// An update after a delete restores the group: the last event wins, as in `checkpoint`.
		"deleted": "0",
	}
	if body.Name != nil {
		cols["name"] = quote(*body.Name)
	}
	if body.Scheme != nil {
		// Stored as sent, without validating against the known values. A scheme we do not recognise
		// yields no verdict, which is the same outcome as `none` and the right one — inventing a
		// verdict from a scheme we do not understand is the failure worth avoiding.
		cols["scheme"] = quote(string(*body.Scheme))
	}
	if body.RelativeCheckgroupID != nil {
		// Written even when empty: clearing the anchor is a real edit, and a stale anchor would make
		// the verdict logic measure from a group the organizer no longer intends.
		cols["relativeCheckgroupId"] = quote(string(*body.RelativeCheckgroupID))
	}

	// `showOnMap` and `mandatory` are on the event and deliberately not stored. See table.sql.

	return c.w.Consume(upsert(cols))
}

// handleDeleted soft-deletes a checkgroup.
//
// No INSERT: a delete for a group never seen affects zero rows, which is correct and idempotent.
//
// **No cascade to its checkpoints**, and that is deliberate rather than an omission. Upstream emits no
// per-checkpoint event when a group is deleted, so the checkpoints keep pointing at a group that is
// now flagged deleted — and the reveal rule resolves that on read (task 256), dropping checkpoints
// whose group no longer resolves. Cascading here instead would couple two independent projections'
// replay order, and getting that wrong would delete checkpoints the race area is derived from.
func (c consumer) handleDeleted(msg cqrs.Message, year string) error {
	var body messages.NathejkCheckgroupDeleted
	// A delete body carrying nothing usable is still a delete: the id is in the subject. So a decode
	// failure here is not fatal, unlike an update whose payload *is* the information.
	_ = msg.Body(&body)

	id := string(body.CheckgroupID)
	if id == "" {
		id = subjectEntityID(msg.Subject())
	}
	if id == "" {
		return fmt.Errorf("checkgroup delete with no checkgroupId")
	}

	return c.w.Consume(fmt.Sprintf(
		"UPDATE checkgroup SET deleted=1 WHERE checkgroupId=%s AND year=%s", quote(id), quote(year)))
}

// handleSorted applies route order.
//
// One event for the whole collection rather than one update per group: a drag is one operator gesture,
// and N events would let a replay observe orders that never existed on screen. Position in the list is
// the order; ids not named keep theirs.
func (c consumer) handleSorted(msg cqrs.Message, year string) error {
	var body messages.NathejkCheckgroupsSorted
	if err := msg.Body(&body); err != nil {
		return err
	}

	for i, id := range body.SortedCheckgroupIDs {
		if id == "" {
			continue
		}
		if err := c.w.Consume(fmt.Sprintf(
			"UPDATE checkgroup SET sortOrder=%d WHERE checkgroupId=%s AND year=%s",
			i, quote(string(id)), quote(year))); err != nil {
			return err
		}
	}
	return nil
}

// subjectYear extracts the year from NATHEJK.<year>.checkgroup(s)....
func subjectYear(s cqrs.Subject) string {
	parts := s.Parts()
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// subjectEntityID extracts the id, which is the part before the verb.
//
// The subject is authoritative where it disagrees with the body: that is what the stream routed on.
// Returns "" for the collection-level `checkgroups.sorted` subject, which carries no id.
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

// upsert builds an idempotent INSERT ... ON DUPLICATE KEY UPDATE.
//
// Idempotency is not optional: projections are rebuilt by replaying the stream from sequence zero on
// every boot, so every statement runs again on each start.
//
// Only the columns a given event carries are written, which is what makes partial updates safe — see
// handleUpdated.
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
		if name == "checkgroupId" || name == "year" {
			continue
		}
		updates = append(updates, fmt.Sprintf("%s=VALUES(%s)", name, name))
	}

	if len(updates) == 0 {
		return fmt.Sprintf("INSERT IGNORE INTO checkgroup (%s) VALUES (%s)",
			strings.Join(names, ", "), strings.Join(values, ", "))
	}
	return fmt.Sprintf("INSERT INTO checkgroup (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
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
