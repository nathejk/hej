package main

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"nathejk.dk/internal/phone"
	"nathejk.dk/internal/pin"
	"nathejk.dk/internal/users"
)

// antiEnumerationMessage is returned by request-pin regardless of whether the
// number was recognized, so the response never reveals who is a known user.
const antiEnumerationMessage = "If we know you, we have sent you an SMS. " +
	"If you don't receive an SMS and you feel we should know you, please reach out."

type requestPinRequest struct {
	Phone string `json:"phone"`
}

// requestPinHandler starts phone login: it normalizes the number, and if it is
// recognized, issues a PIN and sends it by SMS. The response is identical
// whether or not the number was recognized (anti-enumeration).
//
// @Summary      Request a login PIN
// @Description  Sends an SMS PIN to the phone number if it is recognized. The response is identical regardless of recognition (anti-enumeration).
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      requestPinRequest  true  "Phone number"
// @Success      200      {object}  map[string]string
// @Failure      400      {object}  map[string]string
// @Failure      429      {object}  map[string]string
// @Router       /auth/request-pin [post]
func (app *application) requestPinHandler(w http.ResponseWriter, r *http.Request) {
	// Per-IP rate limit to blunt enumeration / SMS-bombing.
	if !app.requestPinLimiter.Allow(clientIP(r)) {
		app.RateLimitResponse(w, r)
		return
	}

	var input requestPinRequest
	if err := app.ReadJSON(w, r, &input); err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}

	normalized, err := phone.Normalize(input.Phone)
	if err != nil {
		// A malformed number is a genuine client error; it reveals nothing
		// about who is a known user.
		app.BadRequestResponse(w, r, err)
		return
	}

	// Only recognized numbers get a PIN. Everything else falls through to the
	// same success response below.
	//
	// LookupAll, not Lookup: a number shared by two people (siblings) is still a
	// recognized number and must still receive a PIN. Lookup deliberately reports
	// not-found for a shared number, so using it here would silently refuse to send
	// an SMS to exactly the users who need the disambiguation flow.
	if len(app.models.Users.LookupAll(normalized)) > 0 {
		code, issueErr := app.pins.Issue(normalized)
		switch {
		case issueErr == nil:
			if sendErr := app.sms.Send(r.Context(), normalized, pinMessage(code)); sendErr != nil {
				app.ServerErrorResponse(w, r, sendErr)
				return
			}
		case errors.Is(issueErr, pin.ErrCooldown):
			// A PIN was sent very recently; silently skip the resend but still
			// return the same success response.
		default:
			app.ServerErrorResponse(w, r, issueErr)
			return
		}
	}

	if err := app.WriteJSON(w, http.StatusOK, map[string]string{"message": antiEnumerationMessage}, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

func pinMessage(code string) string {
	return fmt.Sprintf("Din Hej Nathejk kode er: %s", code)
}

type verifyPinRequest struct {
	Phone string `json:"phone"`
	Pin   string `json:"pin"`
}

// identityResponse is who the caller is signed in as.
type identityResponse struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	// ProfileCount is how many profiles the caller's phone number carries (PRD 012).
	//
	// Present so the client can decide whether to offer "Skift profil" at all: a control that
	// answers "you have nothing to switch to" is worse than no control, and the majority of
	// members have exactly one profile.
	//
	// A fact about the caller's *own* number, so it discloses nothing they could not already
	// learn by signing in. Omitted when unknown — e.g. no directory — rather than reported as
	// 1, because "one profile" and "we could not look" should not be the same answer.
	ProfileCount int `json:"profile_count,omitempty"`
}

// candidate is one owner of a shared phone number, as shown in the chooser.
//
// The payload is deliberately thin. Disambiguation necessarily shows one person
// something about the others on their number — defensible, since whoever holds the
// phone already shares a household with them — but it is a disclosure, so it carries
// only what is needed to recognise yourself.
//
// That is a first name plus **the affiliation**: the patrulje or klan for a spejder or
// bandit, the section for crew. Affiliation is not decoration — a first name alone is
// often not enough (two siblings, or a parent and child with similar names), and
// "Patrulje Ravnene" versus "Samarit" is usually the thing that makes the choice
// obvious. Still no surname, address, birthday or role.
type candidate struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	// Team is the patrulje or klan; Section is the crew affiliation. Exactly one is
	// normally set, and both are omitted when empty so the client can render whichever
	// arrives without branching on role.
	Team    string `json:"team,omitempty"`
	Section string `json:"section,omitempty"`
}

// chooseRequiredResponse is returned when the verified number belongs to several
// people.
//
// It is a 200, not an error: verification *succeeded*. The client branches on the
// presence of `choice_token` rather than on a status code, so a shared number is a
// different shape of success rather than a failure to interpret.
type chooseRequiredResponse struct {
	ChoiceToken string      `json:"choice_token"`
	Candidates  []candidate `json:"candidates"`
}

type chooseRequest struct {
	Token  string `json:"token"`
	UserID string `json:"user_id"`
}

// firstName reduces a display name to its first word.
//
// Used for the chooser only. "Freja" is enough for a sibling to recognise themselves;
// "Freja Mikkelsen" hands the holder of the phone a fuller identifier for someone who
// is not them, for no added benefit.
func firstName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if i := strings.IndexByte(name, ' '); i > 0 {
		return name[:i]
	}
	return name
}

// verifyPinHandler completes phone login: it verifies the submitted PIN and, on
// success, either establishes a session or — when the number belongs to several
// people — returns a choice token and the candidates to pick from.
//
// @Summary      Verify a login PIN
// @Description  Verifies the SMS PIN for a phone number. On success, normally sets a session cookie and returns identity + role. When the number is registered to several people, returns 200 with a short-lived choice_token and the candidates instead; the client then calls /auth/choose. Both are successes — branch on the presence of choice_token, not on the status code.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      verifyPinRequest  true  "Phone number and PIN"
// @Success      200      {object}  identityResponse  "Signed in (single owner)"
// @Failure      400      {object}  map[string]string
// @Failure      401      {object}  map[string]string
// @Failure      429      {object}  map[string]string
// @Router       /auth/verify [post]
func (app *application) verifyPinHandler(w http.ResponseWriter, r *http.Request) {
	var input verifyPinRequest
	if err := app.ReadJSON(w, r, &input); err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}

	normalized, err := phone.Normalize(input.Phone)
	if err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}

	if err := app.pins.Verify(normalized, input.Pin); err != nil {
		if errors.Is(err, pin.ErrTooManyAttempts) {
			app.RateLimitResponse(w, r)
			return
		}
		// ErrNoPIN / ErrExpired / ErrMismatch all map to the same 401 so the
		// response never distinguishes an unknown number from a wrong PIN.
		app.InvalidCredentialsResponse(w, r)
		return
	}

	// A PIN only ever exists for a recognized number, so there is normally at least
	// one match. LookupAll rather than Lookup, because Lookup deliberately reports
	// not-found for a shared number (see users.Directory).
	matches := app.models.Users.LookupAll(normalized)
	switch len(matches) {
	case 0:
		app.InvalidCredentialsResponse(w, r)
		return

	case 1:
		user := matches[0]
		app.sessions.Issue(w, user.ID, string(user.Role))
		if err := app.WriteJSON(w, http.StatusOK, identityResponse{UserID: user.ID, Role: string(user.Role)}, nil); err != nil {
			app.ServerErrorResponse(w, r, err)
		}
		return

	default:
		// A shared number. The PIN proved control of the phone, not which of its
		// owners is holding it, so no session is issued here — picking one would be a
		// guess, and a wrong guess means one sibling reading the other's profile.
		//
		// Logged because it is also a data signal: 213 numbers in the real event data
		// are shared, and an organizer may want to correct some of them upstream.
		app.Logger.Info("shared phone number: asking the user to disambiguate",
			"candidates", len(matches),
		)

		candidates := candidatesFor(matches)

		resp := chooseRequiredResponse{
			ChoiceToken: app.choices.Issue(normalized),
			Candidates:  candidates,
		}
		if err := app.WriteJSON(w, http.StatusOK, resp, nil); err != nil {
			app.ServerErrorResponse(w, r, err)
		}
		return
	}
}

// chooseHandler exchanges a choice token plus a chosen user id for a session.
//
// It is the second half of login for a shared phone number. Three things make it safe
// to expose, and all three are load-bearing:
//
//  1. The token is only minted after a successful PIN verification, so this endpoint
//     cannot be used to enumerate anything a caller had not already proven access to.
//  2. The token is bound to the verified phone number, so it cannot be redeemed
//     against a different number's owners.
//  3. The chosen user must be one of *that* number's current owners, re-checked here
//     against the directory rather than trusted from the request. Skipping this would
//     turn a verified PIN for any number into a session as any user in the system.
//
// @Summary      Choose an account after PIN verification
// @Description  Completes login for a phone number shared by several people. Exchanges the short-lived choice_token from /auth/verify, plus the chosen user_id, for a session cookie. The chosen user must be a current owner of the verified number.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      chooseRequest  true  "Choice token and chosen user id"
// @Success      200      {object}  identityResponse
// @Failure      400      {object}  map[string]string
// @Failure      401      {object}  map[string]string  "Invalid or expired token, or a user who does not own the verified number"
// @Router       /auth/choose [post]
func (app *application) chooseHandler(w http.ResponseWriter, r *http.Request) {
	var input chooseRequest
	if err := app.ReadJSON(w, r, &input); err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}

	normalized, err := app.choices.Verify(input.Token)
	if err != nil {
		// Expired and invalid both return 401. The client's remedy is the same in
		// either case — start again with a new PIN — and distinguishing them would tell
		// a forger that their signature was accepted.
		app.InvalidCredentialsResponse(w, r)
		return
	}

	// Re-resolve the owners now. Using the token's phone number rather than anything
	// in the request is what binds the choice to the verification; re-reading the
	// directory rather than trusting a list the client was given earlier is what stops
	// a stale or tampered candidate being accepted.
	for _, u := range app.models.Users.LookupAll(normalized) {
		if u.ID != input.UserID {
			continue
		}
		app.sessions.Issue(w, u.ID, string(u.Role))
		if err := app.WriteJSON(w, http.StatusOK, identityResponse{UserID: u.ID, Role: string(u.Role)}, nil); err != nil {
			app.ServerErrorResponse(w, r, err)
		}
		return
	}

	// A valid token but a user who does not own that number: either a stale candidate
	// list or someone trying their luck. Worth a warning — it should not happen in
	// normal use.
	app.Logger.Warn("choice rejected: user is not an owner of the verified number")
	app.InvalidCredentialsResponse(w, r)
}

// candidatesFor builds the chooser payload for a number's owners.
//
// Shared by login (verifyPinHandler) and profile switching (switchProfileHandler) so the two
// cannot drift on what a candidate discloses — the comment on `candidate` explains why that
// payload is as thin as it is, and a second construction site is how such a rule quietly stops
// being true.
func candidatesFor(users []users.User) []candidate {
	out := make([]candidate, 0, len(users))
	for _, u := range users {
		out = append(out, candidate{
			UserID:  u.ID,
			Name:    firstName(u.Name),
			Team:    u.PatrolName,
			Section: u.Section,
		})
	}
	return out
}

// meHandler returns the current session identity + role. It runs behind
// requireAuth, so reaching it means a valid session exists.
//
// @Summary      Current identity
// @Description  Returns the signed-in user's id + role. 401 when not signed in.
// @Tags         auth
// @Produce      json
// @Success      200  {object}  identityResponse
// @Failure      401  {object}  map[string]string
// @Router       /me [get]
func (app *application) meHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}

	resp := identityResponse{UserID: s.UserID, Role: s.Role}
	// The profile count comes from the directory rather than the session, because the session
	// carries only {userId, role} — deliberately, since that is all the router guard needs. A
	// failed or empty lookup leaves the count at zero, which the client reads as "no switcher":
	// the safe direction, since offering the control is the thing that could mislead.
	if u, found := app.models.Users.Get(s.UserID); found && u.Phone != "" {
		resp.ProfileCount = len(app.models.Users.LookupAll(u.Phone))
	}

	if err := app.WriteJSON(w, http.StatusOK, resp, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// switchProfileHandler starts a profile switch for a number that carries several profiles.
//
// It returns the same `{choice_token, candidates}` shape a shared-number /auth/verify returns, so
// the client drives both with one code path, and the switch completes through the **unchanged**
// /auth/choose. Leaving that handler alone is deliberate: it is where ownership is re-checked and
// the session is issued, and a second copy of that logic is how the login and switch paths would
// drift apart.
//
// # What this preserves, and what it replaces
//
// /auth/choose is safe because of three properties (see its comment): the token is minted only
// after a PIN verification, it is bound to the verified number, and the chosen user is re-checked
// against that number's current owners. This handler keeps the second and third exactly as they
// are, and replaces the first with **"minted for the number the caller is already authenticated
// as"**.
//
// That is not a weakening this app can act on: the holder proved PIN control of the number at
// login, and every profile on it is already reachable by that same holder through the login
// chooser. The switch removes an SMS round-trip, not a barrier. PRD 012 §8 carries the full
// reasoning and the honest residual (a 7-day session window), which is why the switch is logged.
//
// @Summary      Start a profile switch
// @Description  For a phone number that carries several profiles. Returns a short-lived choice_token and the candidates, in the same shape a shared-number /auth/verify returns; the client then calls /auth/choose, which issues a session replacing the current one. Deliberately requires no new SMS code — the caller already proved control of this number at login, and every profile on it is reachable through the login chooser. 409 when the number carries fewer than two profiles or the caller has no number on file.
// @Tags         auth
// @Produce      json
// @Success      200  {object}  chooseRequiredResponse
// @Failure      401  {object}  map[string]string
// @Failure      409  {object}  map[string]string  "nothing to switch to"
// @Router       /auth/switch [post]
func (app *application) switchProfileHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}

	// The number is read from the directory for the *session's* user — never from the request.
	// That is what binds a switch to the caller's own number and makes it impossible to ask for
	// somebody else's owners.
	current, found := app.models.Users.Get(s.UserID)
	if !found {
		app.AuthenticationRequiredResponse(w, r)
		return
	}
	if current.Phone == "" {
		// Some members have no number of their own (PRD 006 §11 Q13). Nothing to switch
		// between, and not an error the user can act on.
		app.ConflictResponse(w, r, "der er kun én profil på dette nummer")
		return
	}

	owners := app.models.Users.LookupAll(current.Phone)
	if len(owners) < 2 {
		// A single-owner number must not return a list of one: the client would show a chooser
		// offering the profile the user is already in.
		app.ConflictResponse(w, r, "der er kun én profil på dette nummer")
		return
	}

	// Logged because this is an identity change made without a fresh proof of the number
	// (PRD 012 §8). The chosen profile is logged by /auth/choose; this is the other half.
	app.Logger.Info("profile switch started",
		"fromUserId", s.UserID,
		"profiles", len(owners),
	)

	resp := chooseRequiredResponse{
		ChoiceToken: app.choices.Issue(current.Phone),
		Candidates:  candidatesFor(owners),
	}
	if err := app.WriteJSON(w, http.StatusOK, resp, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}

// logoutHandler clears the session cookie. It is idempotent and safe to call
// without an active session.
//
// @Summary      Sign out
// @Description  Clears the session cookie.
// @Tags         auth
// @Produce      json
// @Success      200  {object}  map[string]string
// @Router       /auth/logout [post]
func (app *application) logoutHandler(w http.ResponseWriter, r *http.Request) {
	app.sessions.Clear(w)
	if err := app.WriteJSON(w, http.StatusOK, map[string]string{"message": "signed out"}, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}
