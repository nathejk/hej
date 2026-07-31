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
