package person

import (
	"fmt"
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

// The message itself now lives in **shared-go** as `messages.NathejkMemberVerified` (task 147).
//
// It was declared here first, following the portrait precedent, because nothing outside `hej`
// consumed it; the maintainer has since lifted it, which is the right home for a member fact once
// a second party may read it. What stays here is what is genuinely ours: the subject this service
// publishes on, and the projection that folds the event into our read model.
//
// Field names to be careful with, because they changed in the lift and the old ones read fine:
//
//	PhoneParentAcknowledged — the number the member says can be reached (was AcknowledgedPhone)
//	PhoneParentRegistered   — what the register held at that moment (was RegisteredPhone)
//
// VerifiedSubject builds the subject a verification is published on:
//
//	NATHEJK.<year>.member.<personId>.verified
//
// Per person, like the portrait subject, so `nats stream purge --subject` can erase one
// individual's history and nothing else — this event carries a parent's phone number, so
// that matters here as much as it does for a photograph.
//
// On `NATHEJK` because it is a small, low-frequency domain fact about a member, and
// `NATHEJK.>` already claims the subject, so no broker topology change is needed
// (contrast task 081, where the position track's volume forced a sibling stream).
//
// `member.` rather than `person.`: "person" is this projection's local word for a row
// assembled from several populations, while the fact being recorded is about a *member* of
// the event — which is the vocabulary shared-go, hq and PRD 005 all use.
func VerifiedSubject(year, personID string) (cqrs.Subject, error) {
	if err := validSubjectToken(year, "year"); err != nil {
		return nil, err
	}
	if err := validSubjectToken(personID, "person id"); err != nil {
		return nil, err
	}
	return cqrs.SubjectFromStr(
		fmt.Sprintf("NATHEJK.%s.member.%s.verified", year, personID)), nil
}

// handleMemberVerified records the verification on the person's row.
//
// Both columns are written together, always. `verifiedAt` without `acknowledgedPhone`
// would be a verification whose subject is unknown, which `Person.IsVerified` correctly
// refuses to trust — so writing one without the other would produce a row that looks
// verified to a human reading the table and unverified to the code.
//
// Idempotent by construction: a replay writes the same two values from the same event.
// Re-confirmation (a member who verifies again after their guardian number changed)
// arrives as a later event with a later timestamp and simply overwrites.
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
	if body.PhoneParentAcknowledged == "" {
		// A verification that names no number cannot be checked for staleness later, so
		// it would be a permanent tick that no guardian-number change could ever clear.
		// Rejected rather than stored as a half-fact.
		return fmt.Errorf("member verified with no acknowledged phone")
	}

	verifiedAt := body.VerifiedAt
	if verifiedAt.IsZero() {
		// Should not happen — the publisher always sets it — but a zero TIMESTAMP is not
		// storable in MariaDB, and a message from a future publisher that forgets the
		// field must not dead-letter. The row is what matters; the exact minute is not.
		verifiedAt = time.Now().UTC()
	}

	return c.w.Consume(fmt.Sprintf(
		"UPDATE person SET verifiedAt=%s, acknowledgedPhone=%s WHERE personId=%s AND year=%s",
		quote(verifiedAt.UTC().Format("2006-01-02 15:04:05")),
		quote(string(body.PhoneParentAcknowledged)),
		quote(personID),
		quote(year),
	))
}
