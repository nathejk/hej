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
	// unmapped reports a section slug Classify does not recognise. It is a callback
	// and not a logger because this package must stay free of the application's
	// logging (see the package doc: it is bound for shared-go). nil is a valid value
	// and means "do not report".
	unmapped func(slug string)
	// unusablePhone reports a phone number that arrived but could not be normalized.
	// Same nil-safe callback reasoning as unmapped.
	unusablePhone func(personID, field string, digits int)
}

// reportUnmappedSlug surfaces a section slug the classifier does not know.
//
// Nil-safe on purpose: tests and any future caller that has nothing to log with must
// not have to supply a sink to use the projection.
func (c consumer) reportUnmappedSlug(slug string) {
	if c.unmapped == nil {
		return
	}
	c.unmapped(slug)
}

// normalizePhone folds a phone number, reporting it if it arrived but was unusable.
//
// The projection cannot fail on a bad number — one typo must not stall every person
// queued behind it — so the value becomes empty and the row is still written. What was
// missing is that this was called "visible" while being completely silent. It is not
// visible at all: nobody notices that one member of 557 cannot log in, and for the
// guardian field nobody notices until an emergency, when staff are told there is no
// number on file for a member whose parents did supply one.
//
// On the real 2026 data this fires for 14 of 51 spejder whose guardian number looked
// absent: `45`-prefixed numbers (now accepted — see internal/phone), 7- and 9-digit
// typos, and free text naming two numbers ("Mor: ... eller Far: ...").
//
// The raw value is deliberately NOT passed to the sink, only a digit count. These are
// third parties' phone numbers — usually a child's parent — and a log line is the wrong
// place for them; the person id is enough to find the record upstream and fix it. Making
// that structural rather than a rule someone has to remember is the point.
func (c consumer) normalizePhone(personID, field, raw string) string {
	if raw == "" {
		// Absent, not broken. Nothing to report.
		return ""
	}
	normalized := normalizeOrEmpty(c.normalizer, raw)
	if normalized == "" && c.unusablePhone != nil {
		c.unusablePhone(personID, field, countDigits(raw))
	}
	return normalized
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
		// Senior: the same for the people the event calls banditter. "bandit" is not a
		// field anywhere — it is the role a klan senior plays — and the subject
		// vocabulary is the giveaway: the arm number, which is how a bandit is
		// identified in the field, is published on a `bandit.*` subject and projected
		// onto the senior.
		cqrs.SubjectFromStr("NATHEJK.*.senior.*.updated"),
		cqrs.SubjectFromStr("NATHEJK.*.senior.*.deleted"),
		cqrs.SubjectFromStr("NATHEJK.*.bandit.*.armNumber.assigned"),
		// Klan: the team name for a bandit, the counterpart of patrulje below.
		cqrs.SubjectFromStr("NATHEJK:*.klan.*.signedup"),
		cqrs.SubjectFromStr("NATHEJK:*.klan.*.updated"),
		// Crew: the person, plus the section they are assigned to and the section's
		// label. All three are needed because the label arrives on a different event
		// from the assignment, in either order.
		cqrs.SubjectFromStr("NATHEJK.*.crewmember.*.registered"),
		cqrs.SubjectFromStr("NATHEJK.*.crewmember.*.updated"),
		cqrs.SubjectFromStr("NATHEJK.*.crewmember.*.deleted"),
		cqrs.SubjectFromStr("NATHEJK.*.crewmember.*.section.assigned"),
		cqrs.SubjectFromStr("NATHEJK.*.section.*.added"),
		cqrs.SubjectFromStr("NATHEJK.*.section.*.moved"),
		// Deliberately NOT subscribed: NATHEJK.*.crew.*.signedup. It was in PRD 006's
		// first list, and checking it against the stream showed it is a strict subset of
		// crewmember.*.updated — same person, same name/phone/email, one published
		// moments after the other, and its body keys the person as `teamId` rather than
		// `userId`. Consuming it would add a second spelling of the primary key for no
		// extra field.
		//
		// Gøglere: both events, and here `signedup` is NOT redundant — a third of the
		// population has no `updated` at all (see goegler.go). `.status.changed` and
		// `.deleted` were in PRD 006's list and do not exist on the stream; they are left
		// out rather than written blind against a shape nobody has seen.
		cqrs.SubjectFromStr("NATHEJK.*.gøgler.*.signedup"),
		cqrs.SubjectFromStr("NATHEJK.*.gøgler.*.updated"),
		// Patrulje: the team name shown alongside them, and the team they belong to.
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.signedup"),
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.updated"),
		// The one lifecycle transition this app needs: did the member actually start?
		// PRD 005's confirmation step is skipped for members who have (task 080).
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.started"),
		// The one subject this app publishes itself (task 103): a member's portrait.
		// See portrait.go for why it lives on NATHEJK rather than a sibling stream.
		cqrs.SubjectFromStr("NATHEJK.*.portrait.*.captured"),
	}
}

// HandleMessage folds one event into the read model.
//
// An unrecognised subject returns nil rather than an error: a projection that errors
// on messages it does not care about would dead-letter half the stream.
//
// Every returned error is annotated with the subject. That is not cosmetic: the stream
// library logs a handler error and *drops the message* rather than dead-lettering it
// (jetstream.Stream.Consume), so the log line is the only trace it ever existed. A bare
// "unexpected end of JSON input" from a decode is then unattributable to any of the
// ~30k messages in a replay — which is precisely what happened while implementing task
// 074.
func (c consumer) HandleMessage(msg cqrs.Message) error {
	subject := msg.Subject()
	if err := c.handleMessage(msg, subject); err != nil {
		return fmt.Errorf("person: %s: %w", subject.Subject(), err)
	}
	return nil
}

func (c consumer) handleMessage(msg cqrs.Message, subject cqrs.Subject) error {
	year := subjectYear(subject)
	if year == "" {
		// Without a year there is no primary key to write. Dropping it silently
		// would be wrong, but so would failing the replay: report it as a
		// dead-letter-worthy statement instead of guessing a year.
		return fmt.Errorf("no year in subject")
	}

	switch {
	// Checked before the shorter patrulje patterns. Both orderings happen to work
	// here, but hq's projectors carry a comment that the reverse has bitten this
	// codebase before, so specific-first is the house style.
	case subject.Match("nathejk.*.patrulje.*.started"):
		return c.handleTeamStarted(msg, year)
	// Five-part subject, so it must be matched before the four-part senior patterns.
	case subject.Match("nathejk.*.bandit.*.armNumber.assigned"):
		return c.handleArmNumberAssigned(msg, year)
	// Also five parts, and likewise before the four-part crewmember patterns.
	case subject.Match("nathejk.*.crewmember.*.section.assigned"):
		return c.handleSectionAssigned(msg, year)
	case subject.Match("nathejk.*.section.*.added"),
		subject.Match("nathejk.*.section.*.moved"):
		return c.handleSectionAdded(msg, year)
	case subject.Match("nathejk.*.crewmember.*.registered"),
		subject.Match("nathejk.*.crewmember.*.updated"):
		return c.handleCrewMemberUpdated(msg, year)
	case subject.Match("nathejk.*.crewmember.*.deleted"):
		return c.handleCrewMemberDeleted(msg, year)
	// Gøgler. Both are four-part subjects, so they cannot be confused with the
	// five-part `gøgler.*.mail.signup.sent` / `.sms.validate.sent` traffic that shares
	// the same prefix and outnumbers them.
	case subject.Match("nathejk.*.gøgler.*.signedup"):
		return c.handleGoeglerSignedUp(msg, year)
	case subject.Match("nathejk.*.gøgler.*.updated"):
		return c.handleGoeglerUpdated(msg, year)
	case subject.Match("nathejk.*.portrait.*.captured"):
		return c.handlePortraitCaptured(msg, year)
	case subject.Match("nathejk.*.spejder.*.updated"):
		return c.handleSpejderUpdated(msg, year)
	case subject.Match("nathejk.*.senior.*.updated"):
		return c.handleSeniorUpdated(msg, year)
	case subject.Match("nathejk.*.spejder.*.deleted"),
		subject.Match("nathejk.*.senior.*.deleted"):
		return c.handleMemberDeleted(msg, year)
	case subject.Match("nathejk.*.patrulje.*.signedup"),
		subject.Match("nathejk.*.patrulje.*.updated"),
		subject.Match("nathejk.*.klan.*.signedup"),
		subject.Match("nathejk.*.klan.*.updated"):
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
	memberID := string(body.MemberID)

	// Computed once and used twice: in the row, and in the verification check below.
	// Normalizing it twice would also report an unusable number twice for one event.
	guardian := c.normalizePhone(memberID, "phoneParent", string(body.PhoneContact))

	cols := map[string]string{
		"personId": quote(memberID),
		"year":     quote(year),
		"appRole":  quote(role),
		"name":     quote(body.Name),
		// Normalized with the same implementation the login handler uses, so a
		// lookup cannot miss on a formatting difference (see interfaces.go).
		"phone": quote(c.normalizePhone(memberID, "phone", string(body.Phone))),
		// The guardian/emergency contact. Only spejder have one, and it arrives as
		// PhoneContact on this message. NULL rather than "" when absent, so PRD 005
		// can tell "no number on file" from "this population has none".
		"phoneParent": nullableQuote(guardian),
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

	if err := c.w.Consume(upsert(cols)); err != nil {
		return err
	}

	// A changed guardian number invalidates the member's verification.
	return c.w.Consume(invalidateVerification(memberID, year, guardian))
}

// invalidateVerification clears a verification whose acknowledged number no longer
// matches the guardian number on file.
//
// # Why this is necessary
//
// PRD 005 asks the member to confirm the number of a parent or guardian who can be
// reached during the event — for an injury, or to arrange a pickup for someone who has
// resigned. The confirmation is therefore a claim about a *specific* number, not a
// checkbox: "this number can be contacted during Nathejk". Once that number changes,
// the claim is about a number nobody agreed to, and the app would be showing staff a
// green tick for a phone that may not answer. In an emergency-contact flow that is the
// expensive kind of wrong, so `acknowledgedPhone` exists precisely to make the
// staleness decidable.
//
// # Why it is a second statement, and SQL rather than Go
//
// A projector cannot read (cqrs.Writer takes statements, not queries), so the
// comparison has to happen inside the statement. It cannot be folded into the upsert
// either: the upsert must write `phoneParent` unconditionally, while this must fire only
// when the number actually *changed*. Spejder details are re-published on any edit and
// re-delivered on every replay, so clearing on every write would destroy a valid
// verification the first time an organizer corrected an address.
//
// `NOT (acknowledgedPhone <=> ?)` is MariaDB's null-safe inequality, which matters at
// both ends: a guardian number being *removed* must invalidate just as much as one being
// changed, and a plain `<>` against NULL yields NULL and would quietly skip it.
//
// `acknowledgedPhone` is deliberately left in place. The pair then reads as "they did
// verify, and it was for a number that is no longer current", which is more useful than
// erasing the evidence — and it means changing the number back does not silently
// resurrect the old consent, because `verifiedAt` stays NULL until they confirm again.
func invalidateVerification(personID, year, guardianPhone string) string {
	return fmt.Sprintf(
		"UPDATE person SET verifiedAt=NULL "+
			"WHERE personId=%s AND year=%s "+
			"AND verifiedAt IS NOT NULL AND NOT (acknowledgedPhone <=> %s)",
		quote(personID), quote(year), nullableQuote(guardianPhone),
	)
}

// handleSeniorUpdated writes a senior's own details, classified as a bandit.
//
// Seniors carry **no guardian number**: `NathejkSeniorUpdated` has no PhoneContact
// field, and shared-go's `senior` table has no `phoneParent` column. So phoneParent is
// written as NULL rather than left at a default — "this population does not have one"
// has to stay distinguishable from "should have one and it is missing", because PRD
// 005 skips its confirmation step for exactly this reason and PRD 003 renders the two
// differently.
func (c consumer) handleSeniorUpdated(msg cqrs.Message, year string) error {
	var body messages.NathejkSeniorUpdated
	if err := msg.Body(&body); err != nil {
		return err
	}
	if body.MemberID == "" {
		return fmt.Errorf("person: senior updated with no memberId")
	}

	role, _ := Classify(PopulationSenior, "")

	cols := map[string]string{
		"personId":    quote(string(body.MemberID)),
		"year":        quote(year),
		"appRole":     quote(role),
		"name":        quote(body.Name),
		"phone":       quote(c.normalizePhone(string(body.MemberID), "phone", string(body.Phone))),
		"phoneParent": "NULL",
		"address":     quote(body.Address),
		"postalCode":  quote(body.PostalCode),
		"city":        quote(body.City),
		"email":       quote(string(body.Email)),
		"deleted":     boolInt(false),
	}

	if bd, ok := parseBirthday(string(body.BirthDate)); ok {
		cols["birthday"] = quote(bd)
	}

	// The team link arrives on the legacy shape carried by the same event, exactly as
	// for spejder — shared-go's senior projector decodes both for this reason.
	var legacy messages.NathejkMemberAdded
	if err := msg.Body(&legacy); err == nil && legacy.TeamID != "" {
		cols["teamId"] = quote(string(legacy.TeamID))
	}

	return c.w.Consume(upsert(cols))
}

// handleArmNumberAssigned records the arm number a bandit is identified by.
//
// Worth carrying even though this app has no bandit-facing feature yet: it is the
// identification mechanism that needs **no photograph**, so it is the fallback when
// PRD 007's portrait is missing, declined, or unreadable in the dark.
//
// The subject is `bandit.*`, not `senior.*`, and the member id therefore comes from the
// subject rather than the body — `NathejkLokArmNumberAssigned` carries only the number
// and a team type.
func (c consumer) handleArmNumberAssigned(msg cqrs.Message, year string) error {
	var body messages.NathejkLokArmNumberAssigned
	if err := msg.Body(&body); err != nil {
		return err
	}
	// Five-part subject: NATHEJK.<year>.bandit.<memberId>.armNumber.assigned, so the id
	// is the fourth part rather than the second-to-last one subjectEntityID assumes.
	parts := msg.Subject().Parts()
	if len(parts) < 4 || parts[3] == "" {
		return fmt.Errorf("person: arm number assigned with no member id in subject %q", msg.Subject().Subject())
	}
	if body.ArmNumber == "" {
		// Nothing to record. Not an error — an empty assignment is a no-op, not a
		// reason to dead-letter the event.
		return nil
	}

	// UPDATE, not upsert: the arm number describes a senior whose details arrive on
	// their own event. Inserting here would create a row with an arm number and no
	// person attached.
	return c.w.Consume(fmt.Sprintf(
		"UPDATE person SET armNumber=%s WHERE personId=%s AND year=%s",
		quote(body.ArmNumber), quote(parts[3]), quote(year),
	))
}

// handleCrewMemberUpdated writes a crew member's own details.
//
// Crew are keyed by `userId`, not `memberId`, and carry **no guardian number** and no
// team — their affiliation is a section, which arrives separately.
//
// The app role is `crew` (the least-privileged fallback) until a section assignment
// says otherwise. That is the correct starting point rather than a placeholder: a crew
// member genuinely has no known function until they are assigned one, and
// handleSectionAssigned re-classifies them when that happens.
func (c consumer) handleCrewMemberUpdated(msg cqrs.Message, year string) error {
	var body messages.NathejkCrewMemberUpdated
	if err := msg.Body(&body); err != nil {
		return err
	}
	userID := string(body.UserID)
	if userID == "" {
		userID = subjectEntityID(msg.Subject())
	}
	if userID == "" {
		return fmt.Errorf("person: crewmember event with no userId")
	}

	role, _ := Classify(PopulationCrew, "")

	cols := map[string]string{
		"personId":    quote(userID),
		"year":        quote(year),
		"name":        quote(body.Name),
		"phone":       quote(c.normalizePhone(userID, "phone", string(body.Phone))),
		"phoneParent": "NULL",
		"email":       quote(string(body.Email)),
		"deleted":     boolInt(false),
	}

	// appRole is written only on insert, never on update: a later `registered` or
	// `updated` event must not demote a crew member whose section has already been
	// classified. VALUES() would overwrite `samarit` with `crew` on every replay of
	// their details, silently taking the SOS page away from them.
	cols["appRole"] = quote(role)
	return c.w.Consume(upsertKeepingRole(cols))
}

func (c consumer) handleCrewMemberDeleted(msg cqrs.Message, year string) error {
	var body messages.NathejkCrewMemberUpdated
	_ = msg.Body(&body)
	userID := string(body.UserID)
	if userID == "" {
		userID = subjectEntityID(msg.Subject())
	}
	if userID == "" {
		return fmt.Errorf("person: crewmember delete with no userId")
	}
	return c.w.Consume(fmt.Sprintf(
		"UPDATE person SET deleted=1 WHERE personId=%s AND year=%s",
		quote(userID), quote(year),
	))
}

// handleSectionAdded records a section's label and back-fills anyone already assigned
// to it.
//
// The back-fill is what makes the "assignment arrived first" order converge.
func (c consumer) handleSectionAdded(msg cqrs.Message, year string) error {
	var body messages.NathejkSectionAdded
	if err := msg.Body(&body); err != nil {
		return err
	}
	slug := string(body.Slug)
	if slug == "" {
		slug = subjectEntityID(msg.Subject())
	}
	if slug == "" || body.Label == "" {
		// A move with no label carries nothing this projection wants. Not an error:
		// section events carry parent/sort/type fields that are none of its business.
		return nil
	}

	if err := c.w.Consume(fmt.Sprintf(
		"INSERT INTO person_section (year, slug, label) VALUES (%s, %s, %s) "+
			"ON DUPLICATE KEY UPDATE label=VALUES(label)",
		quote(year), quote(slug), quote(body.Label),
	)); err != nil {
		return err
	}

	// Back-fill: anyone already assigned to this slug gets the label now.
	return c.w.Consume(fmt.Sprintf(
		"UPDATE person SET sectionName=%s WHERE year=%s AND sectionSlug=%s",
		quote(body.Label), quote(year), quote(slug),
	))
}

// handleSectionAssigned records which section a crew member belongs to, re-classifies
// their app role from it, and resolves the section's label.
//
// Three statements rather than one, and the order matters:
//
//  1. write the slug and the role derived from it
//  2. resolve the label by joining person_section — which reads the slug written in
//     step 1, so a JOIN in step 1 would have used the *old* slug
//
// Step 2 is what makes the "section arrived first" order converge;
// handleSectionAdded's back-fill covers the other order.
func (c consumer) handleSectionAssigned(msg cqrs.Message, year string) error {
	var body messages.NathejkCrewMemberSectionAssigned
	if err := msg.Body(&body); err != nil {
		return err
	}
	userID := string(body.UserID)
	if userID == "" {
		// Five-part subject: NATHEJK.<year>.crewmember.<userId>.section.assigned.
		if parts := msg.Subject().Parts(); len(parts) >= 4 {
			userID = parts[3]
		}
	}
	if userID == "" {
		return fmt.Errorf("person: section assigned with no userId")
	}

	slug := string(body.SectionSlug)
	role, ok := Classify(PopulationCrew, slug)
	if !ok && slug != "" {
		// An unrecognised slug is a routine data condition, not an error: organizers
		// rename sections and nothing validates the values. The member keeps the
		// least-privileged role and the slug is recorded, so task 078 can find it.
		// Reported through the writer's log rather than swallowed silently.
		c.reportUnmappedSlug(slug)
	}

	// An INSERT rather than an UPDATE: the assignment can arrive before the crew
	// member's own details do, and an UPDATE would then affect zero rows and lose the
	// role for good — nothing re-publishes an assignment. The stub row carries only the
	// key and the section; handleCrewMemberUpdated fills in the rest when it lands, and
	// leaves appRole alone when it does (upsertKeepingRole).
	if err := c.w.Consume(upsert(map[string]string{
		"personId":    quote(userID),
		"year":        quote(year),
		"sectionSlug": quote(slug),
		"appRole":     quote(role),
	})); err != nil {
		return err
	}

	// Resolve the label from whatever sections are known. LEFT JOIN with COALESCE so an
	// assignment to a section we have not seen yet clears the name rather than leaving a
	// stale one from a previous assignment.
	return c.w.Consume(fmt.Sprintf(
		"UPDATE person p LEFT JOIN person_section s ON s.year=p.year AND s.slug=p.sectionSlug "+
			"SET p.sectionName=COALESCE(s.label, '') WHERE p.personId=%s AND p.year=%s",
		quote(userID), quote(year),
	))
}

// handleGoeglerSignedUp writes a gøgler from the signup event.
//
// This handler is the *only* record of roughly a third of the gøglere — 31 of 99 in
// 2026 never get an `updated` (see goegler.go). Skipping it, by analogy with the crew
// signup event that genuinely is redundant, would have quietly excluded them from the
// directory and therefore from logging in at all.
//
// It writes with upsertKeepingRole for the same reason the crew handler does, plus one
// of its own: `signedup` is the *thinner* of the two events, and on replay it is
// re-delivered before the richer `updated`. Fields it does not carry are simply not in
// the column set, so a later re-delivery cannot blank what `updated` filled in.
func (c consumer) handleGoeglerSignedUp(msg cqrs.Message, year string) error {
	var body NathejkGoeglerSignedUp
	if err := msg.Body(&body); err != nil {
		return err
	}
	// TeamID despite the name: for this event it is the person's own id, and it equals
	// the subject's entity id. Falling back to the subject rather than trusting one
	// spelling of the key, because the two gøgler events disagree about it.
	personID := string(body.TeamID)
	if personID == "" {
		personID = subjectEntityID(msg.Subject())
	}
	if personID == "" {
		return fmt.Errorf("gøgler signedup with no id")
	}

	return c.w.Consume(upsertKeepingRole(c.goeglerColumns(personID, year,
		body.Name, string(body.Phone), string(body.Email), "")))
}

// handleGoeglerUpdated writes a gøgler's fuller profile.
func (c consumer) handleGoeglerUpdated(msg cqrs.Message, year string) error {
	var body NathejkGoeglerUpdated
	if err := msg.Body(&body); err != nil {
		return err
	}
	personID := string(body.UserID)
	if personID == "" {
		personID = subjectEntityID(msg.Subject())
	}
	if personID == "" {
		return fmt.Errorf("gøgler updated with no id")
	}

	return c.w.Consume(upsertKeepingRole(c.goeglerColumns(personID, year,
		body.Name, string(body.Phone), string(body.Email), body.Group)))
}

// goeglerColumns builds the column set shared by both gøgler events.
//
// Shared rather than duplicated because the two events describe the same person and any
// divergence between them would be a bug that only shows up for the third of the
// population that has just one of them.
func (c consumer) goeglerColumns(personID, year, name, phone, email, group string) map[string]string {
	role, _ := Classify(PopulationGoegler, "")

	cols := map[string]string{
		"personId": quote(personID),
		"year":     quote(year),
		"appRole":  quote(role),
		"name":     quote(name),
		"phone":    quote(c.normalizePhone(personID, "phone", phone)),
		"email":    quote(email),
		// Gøglere have no guardian number, so NULL rather than "": "this population does
		// not have one" must stay distinguishable from "should have one and it is
		// missing". PRD 005 skips its confirmation step on this, and PRD 003 renders the
		// two differently.
		"phoneParent": "NULL",
		"deleted":     boolInt(false),
	}

	// The scout group goes in teamName, which is a deliberate stretch of that column.
	//
	// A gøgler has no nathejk team and no section, so both of the columns the login
	// chooser reads to tell two people on one phone apart (task 079) are empty for them
	// — two gøglere sharing a number would be offered two identical first names and no
	// way to choose. Their scout group is the only affiliation the events carry.
	//
	// teamId is left empty, which is the part that matters: PRD 002's patrol-scoped
	// reads key on teamId, not teamName, so this cannot make a gøgler appear to be a
	// member of a patrulje. teamName is only ever a label.
	//
	// Written only when non-empty so the thin `signedup` event, which has no group,
	// does not blank a value `updated` supplied.
	if group != "" {
		cols["teamName"] = quote(group)
	}
	return cols
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
// Serves **both** patrulje (spejder) and klan (bandit). One decode covers both because
// `NathejkTeamUpdated` and `NathejkKlanUpdated` agree on the two fields used here —
// `teamId` and `name` — and the klan message simply has fewer of the rest. Worth
// stating, since it looks like the wrong type being used for a klan event.
//
// The name is stored per person rather than joined at read time because the login
// path reads one row and must not fan out. The cost is this fan-out on write, which
// happens rarely — a team is renamed far less often than a member logs in. The login
// chooser (task 079) is the other reader: for two people on one phone, "which klan" or
// "which patrulje" is often the only thing that tells them apart.
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

// handleTeamStarted records that named members actually started the event.
//
// # Scope: this is not a lifecycle projection
//
// `hq` already projects the full member lifecycle (`spejderstatus`): withdrawals,
// pickups, shelter placements, handovers, each with its own message types defined in
// that repo. This projection deliberately does **not** mirror it. It consumes one
// transition, because one is all this app needs — PRD 005 asks a single question,
// "has this member started?", to decide whether to skip the profile-confirmation step.
//
// Duplicating the rest would mean a second, lagging notion of member status in a
// second repo, disagreeing with `hq` in ways nobody would notice until an organizer
// compared two screens. If this app ever needs the operational states, the answer is
// to read `hq`'s projection or lift it to shared-go — not to grow a parallel one here.
//
// So `memberStatus` in this table is coarse by design: `racing` or empty. That is
// still correct for the question being asked, because every state *after* racing also
// implies the member started — a member who is now `waiting` or `sheltered` began the
// event, and we never need to unset the flag.
func (c consumer) handleTeamStarted(msg cqrs.Message, year string) error {
	var body messages.NathejkTeamStarted
	if err := msg.Body(&body); err != nil {
		return err
	}

	// The event names exactly who started, which is better than marking the whole
	// team: hq's projector notes that StartPatrulje publishes a separate `deleted` for
	// every member who did *not* start, so a team-wide update would wrongly mark
	// no-shows as racing.
	for _, m := range body.Members {
		if m.MemberID == "" {
			continue
		}
		// UPDATE, not upsert: a start event must not invent a person whose details
		// have not arrived. If the member is not here yet, the replay will apply their
		// details event and then this one again in order.
		stmt := fmt.Sprintf(
			"UPDATE person SET memberStatus=%s WHERE personId=%s AND year=%s",
			quote(MemberStatusRacing), quote(string(m.MemberID)), quote(year),
		)
		if err := c.w.Consume(stmt); err != nil {
			return err
		}
	}
	return nil
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
