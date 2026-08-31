package main

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"nathejk.dk/internal/commands"
	"nathejk.dk/internal/phone"
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
		Name:                 user.Name,
		Role:                 string(user.Role),
		Team:                 user.PatrolName,
		Section:              user.Section,
		Address:              user.Address,
		PostalCode:           user.PostalCode,
		City:                 user.City,
		Phone:                user.Phone,
		PhoneParent:          user.PhoneParent,
		HasPhoto:             app.hasPortrait(s.UserID),
		ConfirmationRequired: app.confirmationRequired(s.UserID),
		VerifiedAt:           app.verifiedAt(s.UserID),
	}

	if err := app.WriteJSON(w, http.StatusOK, out, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
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
// @Description  Records that the member has looked at the parent/guardian emergency number on file and acknowledged that it can be reached during the event. The two digits are the ones the client masked; they are verified server-side so the acknowledgement is recorded against a real answer. This is a RECOGNITION check, not an authentication factor — /me/profile returns the full number to its owner by design, so the digits are not a secret. Returns 409 when nothing is required (already confirmed, already started the event, or no guardian number on file). Publishes a domain event; no SQL is written.
// @Tags         me
// @Accept       json
// @Produce      json
// @Param        request  body  confirmProfileRequest  true  "The two masked digits and the acknowledgement"
// @Success      204
// @Failure      400  {object}  map[string]string
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

	guardian := ""
	if p.PhoneParent != nil {
		guardian = *p.PhoneParent
	}
	if !lastTwoDigitsMatch(guardian, input.Digits) {
		// A wrong answer is not an accusation — it most likely means the number on file is
		// not one this member knows, which is what the "nummeret er forkert" / "jeg kender
		// ikke nummeret" paths exist for (task 128). The message says what happened and
		// nothing more.
		app.BadRequestMessageResponse(w, r, "de to cifre passer ikke")
		return
	}

	// Both numbers are the same on this path: the member confirmed the number we hold. They differ
	// only on the correction path (task 148), which is what the second field is for.
	if err := app.storeVerification(r.Context(), s.UserID, guardian, guardian); err != nil {
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
// reached, plus what the register held at that moment.
//
// The pair is what keeps two states apart that call for opposite responses — the register moving
// afterwards (stale → ask again) versus the member correcting us (→ fix the register). See
// person.IsVerified and person.GuardianCorrected.
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
	// read as a different number to `IsVerified`, to `GuardianCorrected` and to the projector.
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

	if err := app.storeVerification(r.Context(), s.UserID, normalized, registered); err != nil {
		if errors.Is(err, commands.ErrNoPublisher) {
			app.ServiceUnavailableResponse(w, r, "kan ikke gemmes lige nu")
			return
		}
		app.ServerErrorResponse(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
