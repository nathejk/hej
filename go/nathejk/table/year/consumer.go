package year

import (
	"fmt"
	"strings"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/messages"
)

// consumer folds the upstream year events into the two columns this app reads.
//
// Every fold is idempotent, because the table is rebuilt from sequence zero on every boot: the write is an
// upsert keyed by slug, and it touches only the columns the event is authoritative for.
type consumer struct {
	w cqrs.Writer
}

// Consumes lists the subjects this projection subscribes to.
//
// # Three tokens, and why that is the whole filter
//
// A year's subject is `NATHEJK:<slug>.updated` — the entity *is* the year, so there is no entity id and no
// entity name in the subject. That makes these patterns exactly three tokens wide, and a `*` matches exactly
// one token, so `NATHEJK.*.updated` cannot match `NATHEJK.2026.patrulje.<id>.updated`. The breadth is only
// apparent; hq's copy relies on the same property.
//
// The colon after NATHEJK is how upstream spells these, and `SubjectFromStr` normalises the first colon to a
// dot — the same normalisation `publicpatrol` documents and tests. Kept as a colon to match hq character for
// character, so grepping either repo for how these events are spelled gives one answer.
func (c consumer) Consumes() []cqrs.Subject {
	return []cqrs.Subject{
		cqrs.SubjectFromStr("NATHEJK:*.created"),
		cqrs.SubjectFromStr("NATHEJK:*.updated"),
		cqrs.SubjectFromStr("NATHEJK:*.deleted"),
	}
}

// HandleMessage folds one event.
//
// Errors are annotated with the subject, for the reason every projection here records: the stream library logs
// a handler error and *drops* the message rather than dead-lettering it, so the log line is the only trace it
// existed.
func (c consumer) HandleMessage(msg cqrs.Message) error {
	subject := msg.Subject()
	if err := c.handleMessage(msg, subject); err != nil {
		return fmt.Errorf("year: %s: %w", subject.Subject(), err)
	}
	return nil
}

func (c consumer) handleMessage(msg cqrs.Message, subject cqrs.Subject) error {
	slug := subjectSlug(subject)
	if slug == "" {
		return fmt.Errorf("no year in subject")
	}

	switch {
	case subject.Match("NATHEJK.*.created"):
		// The row, so a later update has something to update. Nothing else is known yet.
		return c.w.Consume(fmt.Sprintf(
			"INSERT INTO event_year SET slug=%s ON DUPLICATE KEY UPDATE slug=VALUES(slug)", quote(slug)))

	case subject.Match("NATHEJK.*.updated"):
		return c.handleUpdated(msg, slug)

	case subject.Match("NATHEJK.*.deleted"):
		return c.w.Consume(fmt.Sprintf("DELETE FROM event_year WHERE slug=%s", quote(slug)))
	}
	return nil
}

// handleUpdated writes the two cities, and only when the event carries them.
//
// # Why the pointers matter
//
// `NathejkYearUpdated`'s fields are pointers precisely so an event can say "headline changed" without saying
// anything about the cities. A nil is **not** an empty string: writing "" for a nil would let an unrelated edit
// in hq — somebody fixing a typo in the description — silently erase the route from every diploma.
//
// So a nil field is skipped, and an event that names neither city writes nothing at all rather than an upsert
// that only touches `updatedAt`.
//
// Headline, description and both dates are read past deliberately. See table.sql.
func (c consumer) handleUpdated(msg cqrs.Message, slug string) error {
	var body messages.NathejkYearUpdated
	if err := msg.Body(&body); err != nil {
		return err
	}

	columns := []string{}
	values := []string{}
	if body.CityDeparture != nil {
		columns = append(columns, "cityDeparture")
		values = append(values, quote(*body.CityDeparture))
	}
	if body.CityDestination != nil {
		columns = append(columns, "cityDestination")
		values = append(values, quote(*body.CityDestination))
	}
	if len(columns) == 0 {
		return nil
	}

	assignments := make([]string, 0, len(columns))
	updates := make([]string, 0, len(columns))
	for i, column := range columns {
		assignments = append(assignments, column+"="+values[i])
		updates = append(updates, column+"=VALUES("+column+")")
	}

	// The row may not exist: `created` is not guaranteed to have been seen first on a replay, and an event
	// for a year this app has never heard of is still a year. An UPDATE would silently do nothing.
	return c.w.Consume(fmt.Sprintf(
		"INSERT INTO event_year SET slug=%s, %s ON DUPLICATE KEY UPDATE %s",
		quote(slug), strings.Join(assignments, ", "), strings.Join(updates, ", "),
	))
}

// subjectSlug extracts the year, which is the second token.
func subjectSlug(s cqrs.Subject) string {
	parts := s.Parts()
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// quote renders a Go string as a SQL string literal.
//
// Same reasoning as the person, checkpoint, glimt, album and publicpatrol packages: cqrs.Writer takes a
// finished statement rather than a statement plus arguments, so escaping is this file's responsibility. A city
// name is organizer-entered free text.
func quote(s string) string { return fmt.Sprintf("%q", s) }

var _ cqrs.Consumer = consumer{}
