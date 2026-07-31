package main

import (
	"errors"
	"fmt"
	"net/http"

	"nathejk.dk/internal/phone"
	"nathejk.dk/internal/pin"
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
	if _, ok := app.models.Users.Lookup(normalized); ok {
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

type identityResponse struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// verifyPinHandler completes phone login: it verifies the submitted PIN and, on
// success, establishes a session cookie and returns the user's identity + role.
//
// @Summary      Verify a login PIN
// @Description  Verifies the SMS PIN for a phone number; on success sets a session cookie and returns identity + role.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      verifyPinRequest  true  "Phone number and PIN"
// @Success      200      {object}  identityResponse
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

	// A PIN only ever exists for a recognized number, so lookup succeeds here.
	user, ok := app.models.Users.Lookup(normalized)
	if !ok {
		app.InvalidCredentialsResponse(w, r)
		return
	}

	app.sessions.Issue(w, user.ID, string(user.Role))

	if err := app.WriteJSON(w, http.StatusOK, identityResponse{UserID: user.ID, Role: string(user.Role)}, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
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
	if err := app.WriteJSON(w, http.StatusOK, identityResponse{UserID: s.UserID, Role: s.Role}, nil); err != nil {
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
