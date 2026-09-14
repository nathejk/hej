package person

import (
	"fmt"
	"strings"
	"time"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/messages"
)

// The verification write path (PRD 005, tasks 132–134).
//
// # Why the event type lives HERE and not in shared-go
//
// PRD 005 §8 and this task's brief said to declare a `member.verified` message in
// shared-go. It is defined here instead, following the precedent set by the portrait
// event (portrait.go, task 103), and the reasoning is worth writing down because the PRD
// says otherwise:
//
//   - Events *this* service publishes are owned by the projection that consumes them.
//     shared-go carries the messages other services publish, and the whole point of that
//     module is that both ends agree on a shape neither one alone controls.
//   - Nothing outside `hej` consumes this event today, and by PRD 005 §4 (revised
//     2026-08-30) nothing is going to as part of this PRD — the `hq` check-in work is
//     explicitly out of scope. A type added to shared-go with no second party would be an
//     unused export in a module three repos depend on, plus a version bump in each.
//   - When a consumer does appear, moving it is mechanical: this whole package is bound
//     for shared-go (see the package doc), and the message travels with it.
//
// So the cross-repo release loop is deliberately not paid here. If that turns out to be
// wrong, what changes is where the struct is declared — not the subject, the body or the
// projection.
//
// # Nothing writes SQL directly
//
// The `verifiedAt` / `acknowledgedPhone` columns are written only by the handler below,
// consuming this event (PRD 008 §8). That is what makes the projection rebuildable, and
// it is why the confirm endpoint publishes rather than updating a row.

// The message itself lives in **shared-go** as `messages.NathejkMemberVerified` (task 147),
// reshaped by task 222 for PRD 015.
//
// It was declared here first, following the portrait precedent, because nothing outside `hej`
// consumed it; the maintainer has since lifted it, which is the right home for a member fact once
// a second party may read it. That second party now exists — the check-in side reads these events
// to skip asking a member who already answered — so the lift has earned itself. What stays here is
// what is genuinely ours: the subject this service publishes on, and the projection that folds the
// event into our read model.
//
// Field names to be careful with, because the shape has changed twice and every old name reads
// fine:
//
//	Phone        — the member's own number, proven by the SMS PIN at login
//	PhoneContact — the emergency contact number the member acknowledged
//	               (was PhoneParentAcknowledged, and AcknowledgedPhone before that)
//
// `PhoneParentRegistered` is gone. It existed so "the register moved since" stayed distinguishable
// from "the member corrected us", and PRD 015 §4 dropped both questions: the only thing worth
// knowing is whether a verified contact number exists yet, because that is what decides whether
// the counter asks. `Year` is gone too — it is the second subject token.
//
// VerifiedSubject builds the subject a verification is published on:
//
//	NATHEJK.<year>.spejder.<memberId>.verified
//
// Per person, like the portrait subject, so `nats stream purge --subject` can erase one
// individual's history and nothing else — this event carries a contact number, so that
// matters here as much as it does for a photograph.
//
// On `NATHEJK` because it is a small, low-frequency domain fact about a member, and
// `NATHEJK.>` already claims the subject, so no broker topology change is needed
// (contrast task 081, where the position track's volume forced a sibling stream).
//
// # `spejder` is a token, not a population
//
// The subject used to read `member.` (task 133). It moved to `spejder.` in task 222/223 to sit
// alongside the member-lifecycle events, which are already published on
// `NATHEJK.{year}.spejder.{memberId}.{event}` for members who are not only spejder. **A
// consumer must not infer a role from this token.** Bandits publish their own-phone
// verification here too (PRD 015 §6), and the body carries no role field to correct the
// impression — so the temptation to read one out of the subject is real.
//
// Nothing was ever published on the old `.member.` subject, which is why there is no dual
// subscription and no migration.
func VerifiedSubject(year, personID string) (cqrs.Subject, error) {
	if err := validSubjectToken(year, "year"); err != nil {
		return nil, err
	}
	if err := validSubjectToken(personID, "person id"); err != nil {
		return nil, err
	}
	return cqrs.SubjectFromStr(
		fmt.Sprintf("NATHEJK.%s.spejder.%s.verified", year, personID)), nil
}

// handleMemberVerified records on the person's row whichever of the two numbers this event
// established.
//
// # An absent field says nothing; it never clears anything
//
// The event carries two optional numbers (PRD 015 §6). Logging in publishes `Phone` alone;
// confirming or supplying a contact number publishes `PhoneContact`. This handler therefore
// builds its assignment list from what is *present*, and an event with neither is refused.
//
// That asymmetry is the sharp edge, and it is worth being explicit about: a login event must
// never be able to un-verify a contact number. If this handler wrote both columns
// unconditionally, every login by a member who had already confirmed their contact number
// would silently wipe that confirmation — and the member would be asked again at check-in with
// nobody able to explain why. Only the guardian-check endpoints publish `PhoneContact`, and
// only they can change those two columns.
//
// # Two facts, two column pairs
//
// `verifiedAt`/`acknowledgedPhone` is the contact number; `phoneVerifiedAt`/`verifiedPhone` is
// the member's own, proven by the SMS PIN. They are kept apart because a member who skips the
// contact check still verifies their own number by logging in — so sharing one timestamp would
// mark everyone who gave up as verified, silence the question next login, and fast-track at
// check-in exactly the records that still need a number (task 224).
//
// `verifiedAgainstPhone` is gone (task 225), along with the staleness rule that read it: a
// verification now survives a later change to the register. See Person.IsVerified for why that
// follows from what the tick is for.
//
// Idempotent by construction: a replay writes the same values from the same event.
// Re-verification arrives as a later event with a later timestamp and simply overwrites.
func (c consumer) handleMemberVerified(msg cqrs.Message, year string) error {
	var body messages.NathejkMemberVerified
	if err := msg.Body(&body); err != nil {
		return err
	}

	personID := string(body.MemberID)
	if personID == "" {
		personID = subjectEntityID(msg.Subject())
	}
	if personID == "" {
		return fmt.Errorf("member verified with no memberId")
	}
	if body.Phone == "" && body.PhoneContact == "" {
		// A verification naming no number at all is a tick against nothing: there is no way to
		// say later which number it vouched for, so it could never be superseded. Refused rather
		// than stored as a timestamp with no subject.
		return fmt.Errorf("member verified with no phone number")
	}

	verifiedAt := body.VerifiedAt
	if verifiedAt.IsZero() {
		// Should not happen — the publisher always sets it — but a zero TIMESTAMP is not
		// storable in MariaDB, and a message from a future publisher that forgets the
		// field must not dead-letter. The row is what matters; the exact minute is not.
		verifiedAt = time.Now().UTC()
	}
	stamp := quote(verifiedAt.UTC().Format("2006-01-02 15:04:05"))

	var sets []string
	if body.PhoneContact != "" {
		sets = append(sets,
			"verifiedAt="+stamp,
			"acknowledgedPhone="+quote(string(body.PhoneContact)),
		)
	}
	if body.Phone != "" {
		sets = append(sets,
			"phoneVerifiedAt="+stamp,
			"verifiedPhone="+quote(string(body.Phone)),
		)
	}

	// UPDATE, not an upsert: a verification must not invent a person whose details have not
	// arrived. A replay applies their details event first and then this one again, in order.
	return c.w.Consume(fmt.Sprintf(
		"UPDATE person SET %s WHERE personId=%s AND year=%s",
		strings.Join(sets, ", "),
		quote(personID),
		quote(year),
	))
}
