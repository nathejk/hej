package person

import (
	"fmt"
	"time"

	"github.com/jrgensen/cqrs"
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

// MemberVerified says that a member has looked at the guardian number on file and
// acknowledged that it can be reached during the event.
//
// # Why the acknowledged number is on the event
//
// The confirmation is a claim about a *specific* number — "this number can be contacted
// during Nathejk" — not a checkbox. If the guardian number later changes, the claim is
// about a number nobody agreed to, and showing staff a tick for a phone that may not
// answer is the expensive kind of wrong during an emergency. Carrying it here means the
// staleness is decidable from the log alone, and it is what
// `invalidateVerification`/`Person.IsVerified` compare against.
//
// # What is NOT here
//
// The two digits the member typed. They are checked in the handler (task 135) and then
// thrown away: they are a fragment of a third party's phone number, the event already
// carries the whole number they refer to, and putting them on an append-only log would
// mean keeping a needless copy of a parent's phone number forever.
//
// Note this event is **not** an identity check, and must not be read as one later. The
// digit check is a recognition device: `GET /api/me/profile` returns the guardian number
// in full to its owner (PRD 003, and PRD 005 §11 decided to keep it that way), so a
// determined member can read the masked digits out of the response. What the event
// records is that they looked and agreed — nothing about who they are.
type MemberVerified struct {
	PersonID string `json:"personId"`
	Year     string `json:"year"`

	// AcknowledgedPhone is the guardian number as stored (normalized) at the moment of
	// confirmation. Empty is not a valid value: a verification of no number is not a
	// fact worth recording, and the handler rejects it.
	AcknowledgedPhone string `json:"acknowledgedPhone"`

	// VerifiedAt is when the member confirmed, in UTC.
	//
	// On the event rather than derived from delivery time, for the same reason as
	// PortraitCaptured.CapturedAt: delivery time changes on every replay, and this
	// timestamp is the answer to "how many members verified before arriving?" — the
	// measurement PRD 005 §9 is counted from.
	VerifiedAt time.Time `json:"verifiedAt"`
}

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

// GuardianReported says that a member could not confirm the guardian number on file.
//
// A separate event from a "not verified" absence, because absence is not a signal: most
// unverified members simply have not opened the app yet. This is a member actively saying
// something is wrong, which is what an organizer needs to act on before the event.
//
// The reason is a closed pair rather than free text, and the distinction is the point:
//
//	wrong   — the member can see the number is not the right one → a record to FIX
//	unknown — the member does not know it → a record to CHECK
//
// The signal is useful even when the number turns out to be correct: "this member could not
// confirm their guardian number" tells whoever is holding the phone at 02:00 not to rely on
// it without calling first, whatever the register says.
//
// # Why this is not projected onto `person` (yet)
//
// It is published and nothing consumes it. That is deliberate rather than unfinished: PRD 005
// §4 puts the organizer-facing surface out of scope, and §12 has not decided who reads the
// flag or where. Adding a column now would mean guessing at that shape, whereas the log keeps
// the reports — with their timestamps and reasons — until there is a consumer to project them
// for. The event is the durable record; the projection is a view, and views are cheap to add
// later from a log that already has the facts.
type GuardianReported struct {
	PersonID string `json:"personId"`
	Year     string `json:"year"`

	// Reason is "wrong" or "unknown". Validated at the edge (cmd/api), kept as a string
	// here so an older consumer meets an unfamiliar value rather than failing to decode.
	Reason string `json:"reason"`

	// ReportedAt is when the member said so, in UTC — on the event rather than derived from
	// delivery time, which changes on every replay.
	ReportedAt time.Time `json:"reportedAt"`
}

// GuardianReportSubject builds the subject a report is published on:
//
//	NATHEJK.<year>.member.<personId>.guardianReported
//
// Per person, like the verification, so one individual's history can be purged with
// `nats stream purge --subject`.
func GuardianReportSubject(year, personID string) (cqrs.Subject, error) {
	if err := validSubjectToken(year, "year"); err != nil {
		return nil, err
	}
	if err := validSubjectToken(personID, "person id"); err != nil {
		return nil, err
	}
	return cqrs.SubjectFromStr(
		fmt.Sprintf("NATHEJK.%s.member.%s.guardianReported", year, personID)), nil
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
	var body MemberVerified
	if err := msg.Body(&body); err != nil {
		return err
	}

	personID := body.PersonID
	if personID == "" {
		personID = subjectEntityID(msg.Subject())
	}
	if personID == "" {
		return fmt.Errorf("member verified with no personId")
	}
	if body.AcknowledgedPhone == "" {
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
		quote(body.AcknowledgedPhone),
		quote(personID),
		quote(year),
	))
}
