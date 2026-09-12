package main

// Development-only surface (PRD 014 §8).
//
// Everything in this file is registered conditionally, in routes(), on the app running
// with ENV=development. A staging or production binary serves no route defined here at
// all — the check is "is this route registered", not "is this caller allowed", so there
// is no handler left behind to be reached by a misconfiguration.

import (
	"net/http"

	"nathejk.dk/internal/phone"
	"nathejk.dk/internal/pin"
)

// envDevelopment is the one value of ENV in which dev routes exist.
const envDevelopment = "development"

// devRoutesEnabled reports whether the development-only routes should be registered.
func devRoutesEnabled(cfg config) bool {
	return cfg.env == envDevelopment
}

// pinStoreFor builds the PIN store for an environment.
//
// Only a development app gets the plaintext-recall store, because only a development app
// serves the endpoint that reads it. Outside development the issued PIN exists solely as a
// bcrypt hash, exactly as before this file was added.
func pinStoreFor(cfg config) *pin.Store {
	if devRoutesEnabled(cfg) {
		return pin.NewDevStoreWithPlaintextRecall()
	}
	return pin.NewStore()
}

// devPinResponse is the entire response body: the PIN and nothing else.
//
// A dedicated single-field type rather than reusing anything member-shaped, because the
// dev database holds real personal data about minors replayed from the broker. Adding a
// name "to make it easier to tell which account" would put that data on an unauthenticated
// endpoint, and a guardian number here would breach `.rules` outright.
type devPinResponse struct {
	Pin string `json:"pin"`
}

// devPinHandler returns the currently-issued login PIN for a phone number, so a developer
// can complete SMS login on a laptop without reading `docker compose logs api`.
//
// Unauthenticated by necessity — it is used *before* a session exists — which is why it
// only ever exists in development, and why it answers 404 with no detail when no PIN is
// outstanding. An unrecognised number and a recognised-but-no-PIN number are
// indistinguishable, so this cannot be turned into a phone-number oracle even in a dev
// environment holding real numbers. It logs nothing.
//
// @Summary      Read the issued login PIN (development only)
// @Description  Returns the currently-issued SMS login PIN for a phone number so a developer can log in without reading the API logs. This route does not exist outside ENV=development: it is registered conditionally, so a staging or production binary answers 404 for it like any unknown path. Returns only the PIN — never any member data — and 404 with no detail when no PIN is outstanding for the number.
// @Tags         dev
// @Produce      json
// @Param        phone  query     string  true  "Phone number, in any format accepted by /auth/request-pin"
// @Success      200    {object}  devPinResponse
// @Failure      400    {object}  map[string]string
// @Failure      404    {object}  map[string]string
// @Router       /dev/pin [get]
func (app *application) devPinHandler(w http.ResponseWriter, r *http.Request) {
	// Same normalisation as /auth/request-pin, so "30000001" here means the same key it
	// means at login. Anything else would hand the developer a PIN that does not verify.
	normalized, err := phone.Normalize(r.URL.Query().Get("phone"))
	if err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}

	code, ok := app.pins.IssuedPlaintextForDev(normalized)
	if !ok {
		app.NotFoundResponse(w, r)
		return
	}

	if err := app.WriteJSON(w, http.StatusOK, devPinResponse{Pin: code}, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}
