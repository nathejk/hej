package person

import (
	"fmt"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/messages"
)

// consumer folds member events into the read model.
//
// One row per person per year, assembled from several event families that each own a
// slice of it. Handlers therefore write only the columns their event carries (see
// upsert) rather than whole rows, so the spejder handler and the patrulje handler can
// both touch the same row without resetting each other's work.
type consumer struct {
	w          cqrs.Writer
	normalizer PhoneNormalizer
}

// Consumes lists the subjects this projection subscribes to.
//
// Note the two separators: shared-go's own projectors use `NATHEJK.*.spejder.*` for
// member events and `NATHEJK:*.patrulje.*` for team events, and the subject parser
// treats `:` and `.` interchangeably between domain and type. Both forms are copied
// verbatim from the projectors that are known to receive these messages rather than
// normalised to one, because a subject that does not match is silent — the projection
// simply stays empty.
func (c consumer) Consumes() []cqrs.Subject {
	return []cqrs.Subject{
		// Spejder: the person's own details, including the guardian number.
		cqrs.SubjectFromStr("NATHEJK.*.spejder.*.updated"),
		cqrs.SubjectFromStr("NATHEJK.*.spejder.*.deleted"),
		// Patrulje: the team name shown alongside them, and the team they belong to.
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.signedup"),
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.updated"),
	}
}

// HandleMessage folds one event into the read model.
//
// An unrecognised subject returns nil rather than an error: a projection that errors
// on messages it does not care about would dead-letter half the stream.
func (c consumer) HandleMessage(msg cqrs.Message) error {
	subject := msg.Subject()
	year := subjectYear(subject)
	if year == "" {
		// Without a year there is no primary key to write. Dropping it silently
		// would be wrong, but so would failing the replay: report it as a
		// dead-letter-worthy statement instead of guessing a year.
		return fmt.Errorf("person: no year in subject %q", subject.Subject())
	}

	switch {
	case subject.Match("nathejk.*.spejder.*.updated"):
		return c.handleSpejderUpdated(msg, year)
	case subject.Match("nathejk.*.spejder.*.deleted"):
		return c.handleMemberDeleted(msg, year)
	case subject.Match("nathejk.*.patrulje.*.signedup"),
		subject.Match("nathejk.*.patrulje.*.updated"):
		return c.handleTeamUpdated(msg, year)
	}
	return nil
}

// handleSpejderUpdated writes a scout's own details.
//
// Two message types are read from the same event, which looks odd and is deliberate:
// shared-go's spejder projector does the same. NathejkScoutUpdated carries the
// person's details but not their team, while the older NathejkMemberAdded shape
// carries TeamID — historically the same event served both. Reading both and writing
// whichever fields are present is what keeps the team link intact for older events.
func (c consumer) handleSpejderUpdated(msg cqrs.Message, year string) error {
	var body messages.NathejkScoutUpdated
	if err := msg.Body(&body); err != nil {
		return err
	}
	if body.MemberID == "" {
		return fmt.Errorf("person: spejder updated with no memberId")
	}

	role, _ := Classify(PopulationSpejder, "")

	cols := map[string]string{
		"personId": quote(string(body.MemberID)),
		"year":     quote(year),
		"appRole":  quote(role),
		"name":     quote(body.Name),
		// Normalized with the same implementation the login handler uses, so a
		// lookup cannot miss on a formatting difference (see interfaces.go).
		"phone": quote(normalizeOrEmpty(c.normalizer, string(body.Phone))),
		// The guardian/emergency contact. Only spejder have one, and it arrives as
		// PhoneContact on this message. NULL rather than "" when absent, so PRD 005
		// can tell "no number on file" from "this population has none".
		"phoneParent": nullableQuote(normalizeOrEmpty(c.normalizer, string(body.PhoneContact))),
		"address":     quote(body.Address),
		"postalCode":  quote(body.PostalCode),
		"city":        quote(body.City),
		"email":       quote(string(body.Email)),
		// An update must clear a previous soft delete: a member who is deleted and
		// re-added upstream should get their login back.
		"deleted": boolInt(false),
	}

	if bd, ok := parseBirthday(string(body.BirthDate)); ok {
		cols["birthday"] = quote(bd)
	}
	// An unparseable birthday deliberately omits the column instead of failing the
	// statement. Before this, one bad date dead-lettered the whole row — costing that
	// member their login over a field nothing authenticates on.

	// The team link, from the legacy shape carried on the same event.
	var legacy messages.NathejkMemberAdded
	if err := msg.Body(&legacy); err == nil && legacy.TeamID != "" {
		cols["teamId"] = quote(string(legacy.TeamID))
	}

	return c.w.Consume(upsert(cols))
}

// handleMemberDeleted soft-deletes a person.
//
// It does not INSERT: a delete for someone we never saw is a no-op, not a reason to
// create a tombstone row. The UPDATE simply affects zero rows, which is correct and
// idempotent.
func (c consumer) handleMemberDeleted(msg cqrs.Message, year string) error {
	var body messages.NathejkMemberAdded
	if err := msg.Body(&body); err != nil {
		return err
	}
	memberID := string(body.MemberID)
	if memberID == "" {
		// Fall back to the id in the subject: a delete event may carry nothing else.
		memberID = subjectEntityID(msg.Subject())
	}
	if memberID == "" {
		return fmt.Errorf("person: delete with no memberId")
	}

	return c.w.Consume(fmt.Sprintf(
		"UPDATE person SET deleted=1 WHERE personId=%s AND year=%s",
		quote(memberID), quote(year),
	))
}

// handleTeamUpdated denormalizes the team name onto every member of that team.
//
// The name is stored per person rather than joined at read time because the login
// path reads one row and must not fan out. The cost is this fan-out on write, which
// happens rarely — a team is renamed far less often than a member logs in.
//
// It is an UPDATE with no INSERT for the same reason as the delete: members arrive on
// their own events, and a team event should not invent people.
func (c consumer) handleTeamUpdated(msg cqrs.Message, year string) error {
	var body messages.NathejkTeamUpdated
	if err := msg.Body(&body); err != nil {
		return err
	}
	teamID := string(body.TeamID)
	if teamID == "" {
		teamID = subjectEntityID(msg.Subject())
	}
	if teamID == "" || body.Name == "" {
		// Nothing to denormalize. Not an error: team events carry many fields and
		// most of them are none of this projection's business.
		return nil
	}

	return c.w.Consume(fmt.Sprintf(
		"UPDATE person SET teamName=%s WHERE teamId=%s AND year=%s",
		quote(body.Name), quote(teamID), quote(year),
	))
}

// subjectYear extracts the year from a subject like NATHEJK.2026.spejder.<id>.updated.
func subjectYear(s cqrs.Subject) string {
	parts := s.Parts()
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// subjectEntityID extracts the entity id, which is the part before the verb.
func subjectEntityID(s cqrs.Subject) string {
	parts := s.Parts()
	if len(parts) < 4 {
		return ""
	}
	return parts[len(parts)-2]
}

var _ cqrs.Consumer = consumer{}
