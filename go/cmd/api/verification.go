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
//
// # The year comes from the member, not from config
//
// `p.Year` rather than `app.config.eventYear` (task 223). They hold the same value today — the
// row was loaded by that year in the first place — and reading config here is exactly how they
// would stop holding the same value without anyone noticing: the subject would then claim a
// verification for a year the row does not belong to, and the projection's UPDATE, which keys on
// both, would silently match nothing.
//
// `registeredPhone` is still a parameter and is deliberately unused since task 222: the event no
// longer carries what the register held, because PRD 015 §4 dropped both questions that field
// answered. It is kept on the signature until the callers are reworked, so that removing it is one
// reviewable change rather than noise inside this one.
func (app *application) storeVerification(
	ctx context.Context,
	p person.Person,
	acknowledgedPhone string,
	registeredPhone string,
) error {
	// ctx is accepted for symmetry with storePortrait and so this can carry a deadline
	// when the publisher grows one; the publish itself is currently synchronous.
	_ = ctx
	_ = registeredPhone

	if p.PersonID == "" {
		return fmt.Errorf("store verification: no person")
	}
	if acknowledgedPhone == "" {
		// Refused here as well as in the projection handler. A verification that names no
		// number is a tick against nothing — see person.handleMemberVerified.
		return fmt.Errorf("store verification: no acknowledged phone")
	}

	subject, err := person.VerifiedSubject(p.Year, p.PersonID)
	if err != nil {
		// A year or person id that cannot be a subject token is our problem, not the
		// member's — and publishing it anyway would put this person's verification on a
		// subject the per-person purge cannot reach, which matters because the event
		// carries a contact number.
		return fmt.Errorf("store verification: %w", err)
	}

	body := messages.NathejkMemberVerified{
		MemberID: types.MemberID(p.PersonID),
		// The contact number the member acknowledged — the registered one when they recognised it,
		// one they typed when they did not. Which of the two it was is not recorded, on purpose
		// (PRD 015 §4): what decides whether check-in asks is only whether a verified number exists.
		PhoneContact: types.PhoneNumber(acknowledgedPhone),
		VerifiedAt:   time.Now().UTC(),
	}
	if err := app.commands.Publish(subject, body); err != nil {
		return fmt.Errorf("publish verification: %w", err)
	}
	return nil
}

// recordOwnPhoneVerified publishes that the member's own number is verified, because they just
// proved it: a PIN was sent to that number by SMS and they typed it back (PRD 015, task 226).
//
// # Why this is free, and why it is worth having
//
// Nobody is asked anything. The fact falls out of logging in, and it is one more question the
// check-in counter does not have to put to a queue of tired teenagers.
//
// # It must never fail the login
//
// Every error here is swallowed after logging — the deliberate exception to the rule stated on
// storeVerification that a failed publish fails the request. There the publish *is* the act the
// member performed; here it is a by-product, and a broker outage must not stand between a member
// and the app they are trying to get into. The event is not lost forever either: the next login
// republishes, because nothing was recorded to suppress it.
//
// # Once per verified number, not once per login
//
// A member logging in every morning restates nothing: the publish is suppressed while the number
// already recorded is the one the PIN just proved (`Person.PhoneVerifiedIs`). If their number
// changes, the new one publishes and supersedes — last verification wins (PRD 015 §6).
//
// The read-then-publish is racy under two simultaneous logins, and that is accepted rather than
// locked: both events would say the same thing about the same member, and the projection takes the
// latest.
//
// # Spejder and bandit only
//
// Crew and gøglere have their own numbers verified through another route, so an event here would
// restate a known fact for the population that generates the most logins. Note the subject still
// reads `spejder` for a bandit — it is a prefix, not a population; see person.VerifiedSubject.
func (app *application) recordOwnPhoneVerified(personID, provenPhone string) {
	if personID == "" || provenPhone == "" {
		return
	}

	p, found := app.person(personID)
	if !found {
		// No projection, or no row yet. Not worth a log line at error level: the member is
		// logging in against the user directory, which can answer when the projection cannot.
		return
	}
	switch p.AppRole {
	case person.RoleSpejder, person.RoleBandit:
	default:
		return
	}
	if p.PhoneVerifiedIs(provenPhone) {
		return
	}

	subject, err := person.VerifiedSubject(p.Year, p.PersonID)
	if err != nil {
		app.Logger.Warn("own phone verified: cannot build subject",
			"personId", p.PersonID, "error", err)
		return
	}

	// `PhoneContact` is deliberately absent, not empty-and-meaningless: this event establishes
	// nothing about the contact number, and the projection must not touch those columns. A login
	// that could clear a confirmation the member made last week would be a silent data loss with
	// no plausible cause for whoever investigated it (see person.handleMemberVerified).
	body := messages.NathejkMemberVerified{
		MemberID:   types.MemberID(p.PersonID),
		Phone:      types.PhoneNumber(provenPhone),
		VerifiedAt: time.Now().UTC(),
	}
	if err := app.commands.Publish(subject, body); err != nil {
		app.Logger.Warn("own phone verified: publish failed, carrying on",
			"personId", p.PersonID, "error", err)
	}
}

// recordContactCheckGivenUp publishes the outcome of a member giving up on the contact-number
// check — by tapping "spring over", or by running out of attempts (PRD 015, tasks 227/228).
//
// # It is a verification, not a failure record
//
// The event is the same shape as the login publish: the member's own `Phone`, no `PhoneContact`.
// That is a true statement rather than a workaround for a missing message type — their own number
// *is* verified, because the PIN proved it, and the contact number is not. A consumer asks "do we
// have a verified contact number for this member?" and gets "not yet", which is exactly what
// check-in needs to know: ask this one.
//
// Nothing records *why* they gave up. Deliberate (PRD 015 §4): a member who could not recall the
// number and a member who tapped past the screen lead to the same action at the counter.
//
// # Unlike the login publish, this always fires
//
// No once-per-number suppression. That optimisation exists to keep daily logins off the stream;
// applying it here would mean a member whose own number was already recorded produces *nothing*
// when they give up — which is the silence this whole PRD exists to remove, for precisely the
// members it is about.
//
// Idempotency is handled by the caller through `contactCheck.Close`, which reports whether the
// check was already over. A double submit therefore publishes once.
//
// # A failed publish fails the request
//
// Opposite of recordOwnPhoneVerified, and for the reason given on storeVerification: here the
// publish *is* the act being recorded. The client is expected to let the member into the app
// anyway — login is the only mandatory step — so a 503 here costs the outcome, not the member's
// evening, and check-in remains the backstop.
func (app *application) recordContactCheckGivenUp(ctx context.Context, p person.Person) error {
	_ = ctx

	if p.PersonID == "" {
		return fmt.Errorf("record given up: no person")
	}

	subject, err := person.VerifiedSubject(p.Year, p.PersonID)
	if err != nil {
		return fmt.Errorf("record given up: %w", err)
	}

	body := messages.NathejkMemberVerified{
		MemberID: types.MemberID(p.PersonID),
		// The member's own number, from the register. The PIN they typed to get here proved it, so
		// this event is entitled to say so — and saying it here means the give-up outcome is not
		// silently *less* informative than the login that preceded it.
		Phone:      types.PhoneNumber(p.Phone),
		VerifiedAt: time.Now().UTC(),
	}
	if err := app.commands.Publish(subject, body); err != nil {
		return fmt.Errorf("publish given up: %w", err)
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
