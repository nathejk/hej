package main

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"nathejk.dk/internal/commands"
	"nathejk.dk/internal/phone"
	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/person"
)

// profileResponse is what the profile page (PRD 003) reads back to its owner.
//
// It is deliberately a *different* shape from the login chooser's candidate list:
// this is a person's own record, so it carries their address and guardian number,
// which the chooser must never grow (that list shows one holder of a shared
// number something about the others).
type profileResponse struct {
	Name string `json:"name"`
	Role string `json:"role"`
	// Team is a patrulje or klan; Section is the crew affiliation. Normally exactly
	// one is set, and both are empty for a gøgler. Empty means "not applicable",
	// not an error.
	Team    string `json:"team"`
	Section string `json:"section"`

	Address    string `json:"address"`
	PostalCode string `json:"postal_code"`
	City       string `json:"city"`

	Phone string `json:"phone"`
	// PhoneParent is null when this population has no guardian number at all
	// (bandit, crew, gøgler) and "" when one is expected but missing. The client
	// hides the row for the former and shows "Ikke registreret" for the latter, so
	// the pointer must survive serialization — do not "simplify" this to a string.
	PhoneParent *string `json:"phone_parent"`

	// HasPhoto says whether a portrait is on file, so the client knows whether to
	// request GET /api/me/photo and whether to nudge.
	//
	// A flag here rather than the client probing the photo endpoint: a probe costs a
	// second request whose only possible answers are 200 and 404, and HEAD is not
	// registered on that route. It also keeps the *bytes* on their own endpoint, which
	// is what lets them be cached independently of these details.
	HasPhoto bool `json:"has_photo"`

	// ConfirmationRequired says whether the member still has to confirm their guardian
	// number (PRD 005). **Derived server-side** from "not verified AND has not started the
	// event" — see confirmationRequired() in verification.go, and PRD 005 §8, which says
	// outright that the client must not reimplement that rule. Two definitions of when to
	// ask would drift, and the one that drifts silently is the one that stops asking.
	//
	// False for everyone with no guardian number on file at all (bandit, crew, gøgler), and
	// that is NOT "verified" — there is simply nothing for them to confirm. The client
	// distinguishes the two by `phone_parent` being null.
	ConfirmationRequired bool `json:"confirmation_required"`

	// VerifiedAt is when the member last confirmed the number **that is currently on
	// file**, or null.
	//
	// Null once the guardian number changes, even though the row keeps the old
	// acknowledgement: reporting the earlier timestamp would tell the member the current
	// number was confirmed, which is the one thing this field must never imply.
	VerifiedAt *time.Time `json:"verified_at"`
}

// showProfileHandler returns the signed-in user's own details. Runs behind
// requireAuth.
//
// The user is resolved from the session cookie, never from a client-supplied id:
// there is no path here that lets a caller name whose profile they want, which is
// what keeps an address and a guardian's phone number from being one URL edit away.
//
// A session whose user no longer resolves (a member deleted mid-session) gets 404
// rather than an empty-but-successful profile, so the client can tell "we have
// nothing on file for you" from "your record is gone".
//
// @Summary      Own profile
// @Description  Returns the signed-in user's own details: name, role, team/section, postal address, own phone and guardian phone, plus whether a portrait is on file. phone_parent is null when the user's population has no guardian number, and an empty string when one is expected but not registered. confirmation_required is derived server-side (PRD 005): true only while the member has a guardian number, has not verified it, and has not started the event. verified_at is null once the guardian number changes, even if an earlier confirmation exists.
// @Tags         me
// @Produce      json
// @Success      200  {object}  profileResponse
// @Failure      401  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /me/profile [get]
func (app *application) showProfileHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}

	user, found := app.models.Users.Get(s.UserID)
	if !found {
		app.NotFoundResponse(w, r)
		return
	}

	out := profileResponse{
		Name:       user.Name,
		Role:       string(user.Role),
		Team:       user.PatrolName,
		Section:    user.Section,
		Address:    user.Address,
		PostalCode: user.PostalCode,
		City:       user.City,
		Phone:      user.Phone,
		// Blanked when it is really the member's own number (task 229): such a record cannot serve
		// its purpose, and letting it verify would fast-track past check-in the one member whose
		// record needs fixing. Projected out here rather than left to the client — `.rules`.
		PhoneParent:          contactNumberForOwner(app.contactNumber(s.UserID, user), user.Phone),
		HasPhoto:             app.hasPortrait(s.UserID),
		ConfirmationRequired: app.confirmationRequired(s.UserID),
		VerifiedAt:           app.verifiedAt(s.UserID),
	}

	if err := app.WriteJSON(w, http.StatusOK, out, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// contactNumber resolves which contact number to show this member: the one check-in recorded once
// they have started, otherwise the register's (PRD 015, task 230).
//
// # Why it reads the projection and not just the directory
//
// The profile is assembled from `users.User`, which carries the register's `PhoneParent` and knows
// nothing about check-in. The number the counter wrote down arrives on the start event and lives on
// the person row, so the two have to be combined here. `person.Person.ContactNumber` owns the
// precedence, so the rule is stated once for every caller.
//
// Falls back to the directory's value whenever the projection cannot answer — an outage must degrade
// to "the register's number", not to "no contact number", which the client would render as though
// the member had none on file.
func (app *application) contactNumber(personID string, user users.User) *string {
	p, found := app.person(personID)
	if !found {
		return user.PhoneParent
	}
	number, has := p.ContactNumber()
	if !has {
		// The projection says this population has no contact number at all. Trust the directory
		// only if it disagrees by having one, since nil-vs-"" is the distinction both sides guard.
		return user.PhoneParent
	}
	return &number
}

// contactNumberForOwner returns the contact number to show the member it belongs to, blanking one
// that is really their own number (PRD 015, task 229).
//
// Takes the two values rather than a record, because the same rule has to hold on two shapes: the
// profile read has a `users.User`, the confirm and skip endpoints have a `person.Person`, and a
// version of this rule per struct is how one of them ends up not applying it.
//
// # Why a member's own number is worse than no number
//
// Some records were registered with the member's own phone as their emergency contact. Such a row
// passes every check we have: it is a well-formed Danish number, the member recognises the last two
// digits instantly, and it verifies perfectly — while being worthless in the situation it exists
// for. An injured or withdrawing 13-year-old's phone is the phone we are trying not to depend on.
//
// Left alone, it would be actively harmful under PRD 015: the member would breeze through the check
// and check-in would skip the one record that most needs fixing. So this returns "expected but not
// registered", which puts the member in front of the field asking for a number — and, if they do
// not supply one, tells the counter to ask.
//
// # "" and nil are different answers and both are load-bearing
//
// nil means "this population has no contact number at all" (bandit, crew, gøgler) and the client
// hides the row. "" means "one is expected and missing", which is what a collision now reads as.
// Returning nil here would tell a spejder they are not supposed to have a contact number — and
// would switch off `confirmation_required` for exactly the member who needs the question.
//
// # Compared after normalization
//
// "20 00 00 01", "+4520000001" and "004520000001" are one number, and the register contains all
// three styles. Both sides are normalized with the same function the login lookup uses; a value
// that will not normalize is compared raw, since an unparseable number is not something to guess
// about.
func contactNumberForOwner(contact *string, ownPhone string) *string {
	if contact == nil || *contact == "" {
		return contact
	}
	if !samePhoneNumber(*contact, ownPhone) {
		return contact
	}
	blank := ""
	return &blank
}

// samePhoneNumber reports whether two numbers are the same number, ignoring formatting.
func samePhoneNumber(a, b string) bool {
	if strings.TrimSpace(a) == "" || strings.TrimSpace(b) == "" {
		// Not "equal because both are blank": an empty own number must never blank a real contact
		// number, which is what a plain string compare on two empties would do for a member whose
		// own phone is missing from the register.
		return false
	}
	na, errA := phone.Normalize(a)
	nb, errB := phone.Normalize(b)
	if errA != nil || errB != nil {
		return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
	}
	return na == nb
}

// confirmProfileRequest is the body of POST /api/me/profile/confirm.
type confirmProfileRequest struct {
	// Digits are the last two digits of the guardian number, which the client masked and
	// the member typed from memory.
	Digits string `json:"digits"`
	// Acknowledged is the *"Dette nummer kan kontaktes i løbet af Nathejk"* tick.
	//
	// Carried explicitly rather than implied by the request existing: the acknowledgement
	// is the substance of the step — the digits only establish that the member looked —
	// and a client that forgot the checkbox must fail loudly rather than have consent
	// inferred from a POST.
	Acknowledged bool `json:"acknowledged"`
}

// confirmProfileHandler records that the member has looked at their guardian number and
// acknowledged that it can be reached during the event (PRD 005).
//
// # The digit check is a recognition check, not an auth factor
//
// The two digits are validated here so the acknowledgement is recorded against a real
// answer rather than against whatever the client felt like claiming. It is **not** a
// confidentiality control: GET /api/me/profile legitimately returns `phone_parent` in full
// to its owner (PRD 003, and PRD 005 §11 decided on 2026-08-30 to keep it that way), so a
// determined member can read the masked digits straight out of the network response. That is
// accepted. The purpose of the step is to make them look at the number and recognise it — a
// member who cannot complete it has discovered that the number on file is not one they know.
// Nobody is authenticated by this, and the number is their own guardian's, not a secret being
// kept from them.
//
// # State is per user and server-side
//
// This endpoint is why the PRD has BFF scope at all (PRD 005 §11): a `localStorage` flag
// would re-prompt a participant after a reinstall or on a new phone, possibly mid-event,
// which is exactly when nobody should be handed a blocking form. The durable record is the
// projection of the published event — no SQL is written here.
//
// @Summary      Confirm the guardian contact number
// @Description  Records that the member has looked at the parent/guardian emergency number on file and acknowledged that it can be reached during the event. The two digits are the ones the client masked; they are verified server-side so the acknowledgement is recorded against a real answer. This is a RECOGNITION check, not an authentication factor — /me/profile returns the full number to its owner by design, so the digits are not a secret. A wrong answer returns 400 with `attempts_remaining` and `check_closed`: after three failures in one login session the check ENDS, `check_closed` is true, the same outcome a skip records is published, and the client should let the member into the app rather than showing an error. Further attempts in that session return 409, as does every other reason there is nothing to confirm (already confirmed, already started the event, no guardian number on file). A new login resets the attempts. Publishes a domain event; no SQL is written.
// @Tags         me
// @Accept       json
// @Produce      json
// @Param        request  body  confirmProfileRequest  true  "The two masked digits and the acknowledgement"
// @Success      204
// @Failure      400  {object}  confirmAttemptFailedResponse  "Wrong digits; carries attempts_remaining and check_closed"
// @Failure      401  {object}  map[string]string
// @Failure      409  {object}  map[string]string
// @Failure      429  {object}  map[string]string
// @Failure      503  {object}  map[string]string
// @Router       /me/profile/confirm [post]
func (app *application) confirmProfileHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}

	// Per-IP, like the PIN endpoint, and for the same modest reason: not secrecy, just so
	// the endpoint cannot be hammered. Applied before the body is read so a flood costs
	// nothing to reject.
	if !app.confirmLimiter.Allow(clientIP(r)) {
		app.RateLimitResponse(w, r)
		return
	}

	var input confirmProfileRequest
	if err := app.ReadJSON(w, r, &input); err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}
	if !input.Acknowledged {
		app.BadRequestResponse(w, r, errors.New("acknowledgement is required"))
		return
	}

	// The member is resolved from the session cookie. There is no id in the path or the
	// body, so no caller can confirm on somebody else's behalf.
	p, found := app.person(s.UserID)
	if !found {
		// No projection, or no row. Not a 404: the client cannot act on this and retrying
		// is the correct behaviour, which is what 503 says.
		app.ServiceUnavailableResponse(w, r, "kan ikke bekræftes lige nu")
		return
	}

	// 409 covers every reason there is nothing to confirm — already verified, already
	// started the event, or no guardian number on file at all (the spejder-only rule,
	// PRD 005 §6). Deliberately one status for all three: the client treats 409 as
	// "carry on into the app", and splitting it would tempt a caller into inferring
	// which population somebody belongs to from an error code.
	if !app.confirmationRequired(s.UserID) {
		app.ConflictResponse(w, r, "ingen bekræftelse er nødvendig")
		return
	}

	// The check is over for this login session: three wrong answers already, or an explicit
	// "spring over". Same 409 as above, deliberately — the client's move is identical (carry on
	// into the app), and the member is not being told off for a request their client should not
	// have sent.
	checkKey := contactCheckKey(s.UserID, s.ExpiresAt)
	if app.contactChecks.Closed(checkKey) {
		app.ConflictResponse(w, r, "ingen bekræftelse er nødvendig")
		return
	}

	guardian := ""
	if contact := contactNumberForOwner(p.PhoneParent, p.Phone); contact != nil {
		// The blanked value, not the raw column: a member must not be able to "confirm" their own
		// number as their emergency contact (task 229). With "" no pair of digits can match, which
		// is the correct answer to a request the client should not have made — the member is shown
		// the supply-a-number field instead.
		guardian = *contact
	}
	if !lastTwoDigitsMatch(guardian, input.Digits) {
		app.rejectConfirmAttempt(w, r, checkKey, p)
		return
	}

	// Both numbers are the same on this path: the member confirmed the number we hold. They differ
	// only on the correction path (task 148), which is what the second field is for.
	if err := app.storeVerification(r.Context(), p, guardian, guardian); err != nil {
		if errors.Is(err, commands.ErrNoPublisher) {
			// The broker is down. Retryable, and reported as such rather than as a
			// success: a confirmation the log never saw did not happen, and telling the
			// member otherwise means they stop being asked and nobody ever sees the flag.
			app.ServiceUnavailableResponse(w, r, "kan ikke bekræftes lige nu")
			return
		}
		app.ServerErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// confirmAttemptFailedResponse is the body of a rejected recall attempt.
//
// A superset of the plain error shape every other endpoint here returns, so a client that only
// reads `error` keeps working. The two extra fields exist because the PWA has to say something
// different on the second miss than on the third, and deriving "that was the last one" from a
// counter the client keeps would put the rule in two places — with the client's copy being the one
// a reload resets.
type confirmAttemptFailedResponse struct {
	Error string `json:"error"`
	// AttemptsRemaining is how many tries are left in this login session. Zero when the check has
	// just been closed.
	AttemptsRemaining int `json:"attempts_remaining"`
	// CheckClosed says the step is over and the member should be let into the app. The outcome has
	// been recorded server-side; the client must not ask again in this session.
	CheckClosed bool `json:"check_closed"`
}

// rejectConfirmAttempt answers a wrong pair of digits, and ends the check on the third miss
// (PRD 015, task 227).
//
// # Not an accusation, and not an authentication failure
//
// A wrong answer most likely means the number on file is not one this member knows — which is the
// discovery the step exists to make. So the body says what happened, how many tries are left, and
// nothing else. In particular it never echoes any part of the registered number: the member is
// being asked to recall it, and a hint would turn the check into a copying exercise (PRD 015 §6,
// no enumeration).
//
// # Why exhaustion is reported as 400 rather than something friendlier
//
// The attempt did fail, and pretending otherwise would make "wrong digits" and "wrong digits, and
// that was your last go" the same response with different prose. `check_closed` is what the client
// branches on.
//
// # A failed publish does not reopen the check
//
// The member is out of tries whether or not the broker is reachable, and that decision cannot be
// un-made — so a 503 here would leave the client stuck on a screen the server considers finished.
// The outcome is logged and lost instead, which PRD 015 already accepts for a skip that cannot
// reach the BFF: check-in is the backstop, and it asks any member with no verified contact number.
func (app *application) rejectConfirmAttempt(
	w http.ResponseWriter,
	r *http.Request,
	checkKey string,
	p person.Person,
) {
	remaining, closed := app.contactChecks.Failed(checkKey)

	message := "de to cifre passer ikke"
	if closed {
		message = "vi spurgte tre gange — du kan komme videre uden at bekræfte nummeret"
		if err := app.recordContactCheckGivenUp(r.Context(), p); err != nil {
			app.Logger.Warn("contact check exhausted but the outcome could not be published",
				"personId", p.PersonID, "error", err)
		}
	}

	out := confirmAttemptFailedResponse{
		Error:             message,
		AttemptsRemaining: remaining,
		CheckClosed:       closed,
	}
	if err := app.WriteJSON(w, http.StatusBadRequest, out, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// skipProfileCheckHandler records that the member gave up on the contact-number check (PRD 015,
// task 228).
//
// # Why a give-up needs an endpoint at all
//
// Until now "spring over" happened only in the client, so the stream could not tell a member who
// looked at the number and could not place it from a member who never opened the app. Both walk up
// to the counter and get asked — but only one of them was worth knowing about in advance, and the
// counter had no way to see it.
//
// # 204 for every outcome the member can cause
//
// A double submit, a retry after a dropped connection, a member who already gave up in this
// session: all 204. The client's next move is the same in each case (let them into the app), and
// there is no failure here for a member to correct. Only a broken publish is reported, as 503.
//
// # It does not gate on confirmationRequired
//
// A member with nothing to confirm has nothing to give up on, but answering 409 for that would
// hand the client a distinction it cannot act on — and would tempt a caller into inferring which
// population somebody belongs to from a status code. The publish is skipped for them instead.
//
// @Summary      Give up on the contact-number check
// @Description  Records that the member could not confirm their emergency contact number and is carrying on without it — either by choosing to skip, or by running out of recall attempts. Publishes that the member's OWN number is verified (the SMS PIN proved it) with an empty contact number, which is what tells check-in to ask this member. No reason is recorded: a member who could not recall the number and one who tapped past the screen lead to the same action at the counter. Idempotent per login session, so a double submit records one outcome. Always 204 unless the event could not be published; the client lets the member into the app regardless, since login is the only mandatory step.
// @Tags         me
// @Produce      json
// @Success      204
// @Failure      401  {object}  map[string]string
// @Failure      429  {object}  map[string]string
// @Failure      503  {object}  map[string]string
// @Router       /me/profile/skip [post]
func (app *application) skipProfileCheckHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}

	// Shares the confirm limiter: this is the same screen and the same conversation.
	if !app.confirmLimiter.Allow(clientIP(r)) {
		app.RateLimitResponse(w, r)
		return
	}

	// Already over for this login session — a second tap, or the client retrying after a timeout
	// it could not distinguish from a failure. One outcome per check, so nothing is published.
	key := contactCheckKey(s.UserID, s.ExpiresAt)
	if app.contactChecks.Close(key) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	p, found := app.person(s.UserID)
	if !found {
		// Nothing to publish against. Retryable rather than a 404: the client cannot act on it,
		// and the member is getting into the app either way.
		app.ServiceUnavailableResponse(w, r, "kan ikke gemmes lige nu")
		return
	}
	// A population with no contact number (bandit, crew, gøgler) never sees this step, so a skip
	// from one is a client bug rather than a fact about the member. Accepted quietly — there is
	// nothing for them to give up on, and nothing worth telling the counter.
	if p.PhoneParent == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := app.recordContactCheckGivenUp(r.Context(), p); err != nil {
		if errors.Is(err, commands.ErrNoPublisher) {
			app.ServiceUnavailableResponse(w, r, "kan ikke gemmes lige nu")
			return
		}
		app.ServerErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// lastTwoDigitsMatch reports whether `typed` is the last two digits of `number`.
//
// Digits are compared, not strings: the stored number is normalized (`4512345678`) while
// what the member sees is grouped for reading (`45 12 34 56 78`), and the client is under no
// obligation to send exactly two bare characters. Anything non-numeric is stripped from both
// sides so a stray space cannot fail a correct answer — the check exists to catch a member
// who does not know the number, not one who typed it with a space.
//
// An empty or too-short number never matches. A spejder with no guardian number on file
// cannot confirm one, and answering "correct" for an empty number would record a
// verification of nothing.
func lastTwoDigitsMatch(number, typed string) bool {
	digits := func(s string) string {
		var b strings.Builder
		for _, r := range s {
			if r >= '0' && r <= '9' {
				b.WriteRune(r)
			}
		}
		return b.String()
	}

	num := digits(number)
	got := digits(typed)
	if len(num) < 2 || len(got) != 2 {
		return false
	}
	return num[len(num)-2:] == got
}

// setGuardianRequest is the body of POST /api/me/profile/guardian.
type setGuardianRequest struct {
	// Phone is the full guardian number the member typed, in whatever form they typed it.
	// Normalized server-side.
	Phone string `json:"phone"`
	// Acknowledged is the *"Dette nummer kan kontaktes i løbet af Nathejk"* tick, required for
	// the same reason as on /confirm: the acknowledgement is the substance of the step, and
	// consent must never be inferred from a POST having arrived.
	Acknowledged bool `json:"acknowledged"`
}

// setGuardianHandler records a guardian number the member supplied themselves, and their
// acknowledgement that it can be reached (PRD 005, task 148).
//
// # Why this is a better outcome than a flag
//
// A member who cannot recognise the number we hold used to be able only to *report* that, leaving
// the record broken and the work with an organizer. Now they can fix it — and the person standing
// there is the one most likely to know their own guardian's number. It turns the step from "verify
// our data" into "make sure we can reach an adult", which is what it was always for.
//
// # It does not overwrite the register
//
// `phoneParent` is projected from upstream and stays that way; pretending otherwise would mean the
// next upstream publish silently reimposes the old number with nobody able to tell which value was
// believed when. What this records is the *acknowledgement*: the number the member says can be
// reached.
//
// What the register held at that moment is deliberately not recorded (PRD 015 §4, task 225). It
// used to be, so that "the register moved since" stayed distinguishable from "the member corrected
// us" — two states that called for opposite responses. Neither question survives the reframing:
// what follows from a verification is only that check-in need not ask this member, and neither
// answer changes that.
//
// A separate endpoint from /confirm deliberately: agreeing with what we hold and replacing it are
// different acts, with different validation and different meaning in the log. One body carrying two
// mutually exclusive fields is the shape that produces "which did the client mean?" bugs.
//
// @Summary      Supply and confirm a guardian contact number
// @Description  Records a parent/guardian emergency number the member typed themselves, together with their acknowledgement that it can be reached during the event. For the member who cannot recognise the number on file — the person standing there is the one most likely to know the right one. The number is normalized server-side. This does NOT overwrite the registered number: the register keeps its own value, and the event records both, so "the register changed since" stays distinguishable from "the member corrected us". Publishes a domain event; no SQL is written.
// @Tags         me
// @Accept       json
// @Produce      json
// @Param        request  body  setGuardianRequest  true  "The full number and the acknowledgement"
// @Success      204
// @Failure      400  {object}  map[string]string
// @Failure      401  {object}  map[string]string
// @Failure      429  {object}  map[string]string
// @Failure      503  {object}  map[string]string
// @Router       /me/profile/guardian [post]
func (app *application) setGuardianHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}

	// Shares the confirm limiter: the two are the same conversation from the same screen, and a
	// member alternating between them should not get twice the budget.
	if !app.confirmLimiter.Allow(clientIP(r)) {
		app.RateLimitResponse(w, r)
		return
	}

	var input setGuardianRequest
	if err := app.ReadJSON(w, r, &input); err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}
	if !input.Acknowledged {
		app.BadRequestResponse(w, r, errors.New("acknowledgement is required"))
		return
	}

	// Normalized before publishing, because every comparison downstream is a string compare
	// against a normalized value — and the login lookup is too. An unnormalized number here would
	// read as a different number to `IsVerified` and to the projector.
	normalized, err := phone.Normalize(input.Phone)
	if err != nil {
		// Plain-language, and not an accusation: a member mistyping their parent's number is the
		// most ordinary thing in this flow.
		app.BadRequestMessageResponse(w, r, "det ser ikke ud som et telefonnummer")
		return
	}

	// Resolved from the session cookie. No id in the path or the body, so nobody can set a
	// guardian number on somebody else's record.
	p, found := app.person(s.UserID)
	if !found {
		app.ServiceUnavailableResponse(w, r, "kan ikke gemmes lige nu")
		return
	}

	// Deliberately NOT gated on confirmationRequired, unlike /confirm. A member whose number was
	// verified last week may discover today that it is wrong, and refusing them would leave the
	// only correction path closed to exactly the people who found the problem.
	registered := ""
	if p.PhoneParent != nil {
		registered = *p.PhoneParent
	}

	if err := app.storeVerification(r.Context(), p, normalized, registered); err != nil {
		if errors.Is(err, commands.ErrNoPublisher) {
			app.ServiceUnavailableResponse(w, r, "kan ikke gemmes lige nu")
			return
		}
		app.ServerErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
