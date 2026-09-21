package publicpatrol

import (
	"fmt"
	"strings"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/messages"
)

// consumer folds the upstream team events into the narrow public read model.
//
// # What this consumer is for
//
// It exists to **throw fields away**. `NathejkTeamUpdated` carries the patrol's name, group and korps
// alongside a leader's name, phone, email, address and postcode. shared-go's own `patrulje` projection
// stores all of it, correctly, because the organizers need it. This one stores four fields and discards
// the rest — see table.sql for why that matters more than the duplication costs.
//
// Every fold is idempotent, because the table is rebuilt from sequence zero on every boot: the writes are
// upserts keyed by teamId, and each event updates only the columns it is authoritative for.
type consumer struct {
	w cqrs.Writer
}

// Consumes lists the subjects this projection subscribes to.
//
// # The colon is cosmetic, and that is worth knowing
//
// These are upstream events, and upstream spells them with a colon after NATHEJK
// (`NATHEJK:2026.patrulje.…`) while the entities this app owns use a dot (`NATHEJK.2026.glimt.…`).
//
// **The library normalises it**: `subject.FromStr` replaces the first colon with a dot, so the two
// spellings produce the same subject and either would work here. The colon is kept anyway, to match
// shared-go's own patrulje consumer character for character — so that grepping the codebase for how these
// events are spelled gives one answer, and so a reader comparing the two files finds no unexplained
// difference.
//
// A test asserts the normalisation rather than trusting this comment, because a comment claiming a
// library behaviour is a comment that goes stale on the next upgrade.
func (c consumer) Consumes() []cqrs.Subject {
	return []cqrs.Subject{
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.updated"),
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.numberassigned"),
	}
}

// HandleMessage folds one event.
//
// Errors are annotated with the subject, for the reason every projection in this repo records: the stream
// library logs a handler error and *drops* the message rather than dead-lettering it, so the log line is
// the only trace it existed.
func (c consumer) HandleMessage(msg cqrs.Message) error {
	subject := msg.Subject()
	if err := c.handleMessage(msg, subject); err != nil {
		return fmt.Errorf("publicpatrol: %s: %w", subject.Subject(), err)
	}
	return nil
}

func (c consumer) handleMessage(msg cqrs.Message, subject cqrs.Subject) error {
	year := subjectYear(subject)
	if year == "" {
		return fmt.Errorf("no year in subject")
	}

	// Matched with dots, which is what the subscription's colon normalises to — see Consumes. Kept
	// identical to shared-go's own consumer so the two cannot disagree about which events they fold.
	//
	// A subject that matches nothing is a **no-op, not an error**: this projection subscribes to two verbs
	// and the stream carries many, so erroring on the rest would fill the log with noise about events that
	// were never ours.
	switch {
	case subject.Match("NATHEJK.*.patrulje.*.updated"):
		return c.handleUpdated(msg, year)
	case subject.Match("NATHEJK.*.patrulje.*.numberassigned"):
		return c.handleNumberAssigned(msg, year)
	}
	return nil
}

// handleUpdated writes the patrol's own details.
//
// # The four fields, and the ones that are read and dropped
//
// Written: name, groupName, korps. Read from the event and **deliberately not written**: contactName,
// contactPhone, contactEmail, contactRole, contactAddress, contactPostalCode. They are in `body` because
// the event carries them; they reach no column, so no query can select them and no template can render
// them.
//
// `teamNumber` is not touched here even though a team event could plausibly carry one, because
// `numberassigned` is the event that is authoritative for it. Writing it from two places is how a number
// gets reverted by a replayed update.
func (c consumer) handleUpdated(msg cqrs.Message, year string) error {
	var body messages.NathejkTeamUpdated
	if err := msg.Body(&body); err != nil {
		return err
	}

	teamID := string(body.TeamID)
	if teamID == "" {
		teamID = subjectEntityID(msg.Subject())
	}
	if teamID == "" {
		// Refused rather than stored: a row with no id cannot be updated by a later event and cannot be
		// addressed by a page, so it would be invisible rubbish that a replay recreates every boot.
		return fmt.Errorf("patrulje updated with no teamId")
	}

	// Only patruljer. A klan has its own events and is not a patrol; PRD 011 §11 Q10 asks whether klans
	// get a page at all, and until that is answered this table must not quietly acquire them — a klan in
	// `public_patrol` would be a page nobody decided to publish.
	if body.Type != "" && !isPatrulje(string(body.Type)) {
		return nil
	}

	return c.w.Consume(fmt.Sprintf(
		"INSERT INTO public_patrol SET teamId=%s, year=%s, name=%s, groupName=%s, korps=%s "+
			"ON DUPLICATE KEY UPDATE "+
			"year=VALUES(year), name=VALUES(name), groupName=VALUES(groupName), korps=VALUES(korps)",
		quote(teamID), quote(year), quote(body.Name), quote(body.GroupName), quote(body.Korps),
	))
}

// handleNumberAssigned records the number the patrol is known by.
//
// A separate event and the only writer of this column. The row may not exist yet — a number can be
// assigned before any update has been seen on a fresh replay — so this is an upsert rather than an
// UPDATE, which would silently do nothing and leave a patrol unreachable by its own number.
func (c consumer) handleNumberAssigned(msg cqrs.Message, year string) error {
	var body messages.NathejkPatrolNumberAssigned
	if err := msg.Body(&body); err != nil {
		return err
	}

	teamID := string(body.TeamID)
	if teamID == "" {
		teamID = subjectEntityID(msg.Subject())
	}
	if teamID == "" {
		return fmt.Errorf("patrol number assigned with no teamId")
	}

	return c.w.Consume(fmt.Sprintf(
		"INSERT INTO public_patrol SET teamId=%s, year=%s, teamNumber=%s "+
			"ON DUPLICATE KEY UPDATE year=VALUES(year), teamNumber=VALUES(teamNumber)",
		quote(teamID), quote(year), quote(body.TeamNumber),
	))
}

// isPatrulje reports whether a team type is a patrol.
//
// Compared as a lowercase string rather than against `types.TeamType` constants, because the value
// arrives over the wire and an unexpected spelling should fall through to "not a patrol" — the safe
// direction, since the cost is a missing page rather than a published one.
func isPatrulje(teamType string) bool {
	return strings.EqualFold(teamType, "patrulje")
}

// subjectYear extracts the year, which is the second token.
func subjectYear(s cqrs.Subject) string {
	parts := s.Parts()
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// subjectEntityID extracts the team id, which is the part before the verb.
func subjectEntityID(s cqrs.Subject) string {
	parts := s.Parts()
	if len(parts) < 4 {
		return ""
	}
	return parts[3]
}

// quote renders a Go string as a SQL string literal.
//
// Same reasoning as the person, checkpoint, glimt and album packages: cqrs.Writer takes a finished
// statement rather than a statement plus arguments, so escaping is this file's responsibility. A patrol
// name is organizer-entered free text, which is not the least trusted input in the service but is
// certainly not the most trusted either.
func quote(s string) string { return fmt.Sprintf("%q", s) }

var _ cqrs.Consumer = consumer{}
