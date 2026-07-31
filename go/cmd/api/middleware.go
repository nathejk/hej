package main

import "net/http"

// requireAuth wraps a handler so it only runs for requests with a valid
// session; otherwise it returns 401. The resolved session is put on the request
// context (see contextGetSession) for the wrapped handler and any future
// role-authorization checks on protected data endpoints.
func (app *application) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := app.sessions.Read(r)
		if err != nil {
			app.AuthenticationRequiredResponse(w, r)
			return
		}
		next(w, r.WithContext(contextSetSession(r.Context(), s)))
	}
}
