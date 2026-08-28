package main

import "net/http"

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
// @Description  Returns the signed-in user's own details: name, role, team/section, postal address, own phone and guardian phone. phone_parent is null when the user's population has no guardian number, and an empty string when one is expected but not registered.
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
		Name:        user.Name,
		Role:        string(user.Role),
		Team:        user.PatrolName,
		Section:     user.Section,
		Address:     user.Address,
		PostalCode:  user.PostalCode,
		City:        user.City,
		Phone:       user.Phone,
		PhoneParent: user.PhoneParent,
	}

	if err := app.WriteJSON(w, http.StatusOK, out, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}
