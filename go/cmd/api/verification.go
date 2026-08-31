package main

import (
	"context"
	"fmt"
	"time"

	"github.com/nathejk/shared-go/messages"
	"github.com/nathejk/shared-go/types"

	"nathejk.dk/nathejk/table/person"
)

// The verification write path (PRD 005, task 133).
//
// # Why this is its own file
//
// Task 133 asked for the publish to live in `profile.go`. It is here instead, mirroring the
// split PRD 003 already shipped: `photo.go` holds the HTTP handlers, `portrait.go` holds the
// write path they call. The endpoint handlers for confirmation (tasks 135/136) go in
// `profile.go` alongside the profile read, so the endpoint group stays in one file — what
// lives here is the publish and the derivations, which is the same seam as portrait.go's.
// One file per HTTP surface, one file per write path.
//
// # No SQL here, deliberately
//
// The `verifiedAt` / `acknowledgedPhone` columns are written by the person projection
// consuming this event, never by this function (PRD 008 §8). That is what keeps the
// projection rebuildable, and it is why a handler holds a `commands.Commands` and has no
// way to reach a `*sql.DB`.
//
// # A failed publish is a failed confirmation
//
// The error is returned rather than swallowed, and the endpoint turns it into a 5xx. The
// alternative — tell the member "bekræftet" and hope — is the worse failure by a wide
// margin: they would stop being asked, no organizer would ever see the flag, and the whole
// point of the step is that somebody looked at that number. `commands.ErrNoPublisher`
// already carries this reasoning for the broker-down case.
//
// Contrast the portrait path, where bytes are stored *before* the event so a failure leaves
// only an unreferenced object. There is nothing to store here, so the ordering question
// does not arise: the event is the whole write.
func (app *application) storeVerification(
	ctx context.Context,
	personID string,
	acknowledgedPhone string,
	registeredPhone string,
) error {
	// ctx is accepted for symmetry with storePortrait and so this can carry a deadline
	// when the publisher grows one; the publish itself is currently synchronous.
	_ = ctx

	if personID == "" {
		return fmt.Errorf("store verification: no person")
	}
	if acknowledgedPhone == "" {
		// Refused here as well as in the projection handler. A verification that names no
		// number cannot be invalidated when the guardian number later changes, so it
		// would become a permanent tick for a phone nobody agreed to — see
		// person.MemberVerified.
		return fmt.Errorf("store verification: no acknowledged phone")
	}

	subject, err := person.VerifiedSubject(app.config.eventYear, personID)
	if err != nil {
		// A year or person id that cannot be a subject token is our problem, not the
		// member's — and publishing it anyway would put this person's verification on a
		// subject the per-person purge cannot reach, which matters because the event
		// carries a parent's phone number.
		return fmt.Errorf("store verification: %w", err)
	}

	body := messages.NathejkMemberVerified{
		MemberID: types.MemberID(personID),
		Year:     types.YearSlug(app.config.eventYear),
		// What the member says can be reached. Today always the registered number; once the
		// correction flow lands (task 148) it may be one they typed instead, which is the whole
		// reason the event carries both.
		PhoneParentAcknowledged: types.PhoneNumber(acknowledgedPhone),
		// What the register held at this moment. Populated now, even though nothing reads it yet:
		// this is an append-only log, so a field left empty today can never be filled in for these
		// events afterwards. It is what keeps "the register changed since" distinguishable from
		// "the member corrected us" — see task 148.
		PhoneParentRegistered: types.PhoneNumber(registeredPhone),
		VerifiedAt:            time.Now().UTC(),
	}
	if err := app.commands.Publish(subject, body); err != nil {
		return fmt.Errorf("publish verification: %w", err)
	}
	return nil
}

// confirmationRequired reports whether this member still has to confirm their guardian
// number.
//
// **Derived here, not in the client** (PRD 005 §8). The rule is "not verified AND has not
// started the event", and it is computed server-side so there is exactly one definition of
// it. A client-side copy would drift the moment either half changed, and the half that
// drifts silently is the one that stops asking.
//
// Three cases where the answer is false and none of them mean "verified":
//
//   - **No guardian number on file at all** (bandit, crew, gøgler — `PhoneParent == nil`).
//     There is nothing to confirm, so asking would render an empty field as though data
//     were missing. Note `nil` and `""` differ: a spejder with an empty number is still
//     asked, because that record is one an organizer wants to hear about.
//   - **Already verified**, with the acknowledged number still matching what is on file
//     (`Person.IsVerified`, which also guards against a stale acknowledgement).
//   - **Already started the event.** Starting implies the data was checked at the counter
//     (PRD 005 §11), and re-asking a member who is already on the trail is worse than
//     useless. Note this is `Person.HasStarted()` — the same single definition the rest of
//     the app uses.
//
// With no database it answers **false**: during an outage the profile page is still worth
// showing, and inviting a confirmation whose endpoint cannot record anything would produce
// a step the member cannot complete.
func (app *application) confirmationRequired(personID string) bool {
	p, ok := app.person(personID)
	if !ok {
		return false
	}
	if p.PhoneParent == nil {
		// nil means "this population has no guardian number" (bandit, crew, gøgler), as
		// distinct from "" which means one is expected and missing. Only the first is a
		// reason not to ask — a spejder with an empty number is exactly the record an
		// organizer needs to hear about, and the "jeg kender ikke nummeret" path (task
		// 128) is how that gets reported.
		return false
	}
	if p.IsVerified() {
		return false
	}
	return !p.HasStarted()
}

// verifiedAt returns when this member confirmed their guardian number, or nil.
//
// Reads through `Person.IsVerified` rather than the raw column, so a verification whose
// acknowledged number no longer matches the number on file is reported as absent — which is
// what it is. Returning the timestamp of a superseded acknowledgement would show a member a
// date that implies their current number was confirmed.
func (app *application) verifiedAt(personID string) *time.Time {
	p, ok := app.person(personID)
	if !ok || !p.IsVerified() {
		return nil
	}
	return p.VerifiedAt
}

// person loads a row from the projection, or reports that it is unavailable.
//
// Never an error to the caller: every consumer of this treats "no answer" as a reason to
// degrade rather than to fail the request, for the same reason `hasPortrait` does.
func (app *application) person(personID string) (person.Person, bool) {
	if app.models.People == nil || personID == "" {
		return person.Person{}, false
	}
	p, found, err := app.models.People.Get(app.config.eventYear, personID)
	if err != nil || !found {
		return person.Person{}, false
	}
	return p, true
}
