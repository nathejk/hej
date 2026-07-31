package main

import (
	"context"
	"net/http"

	"nathejk.dk/internal/session"
)

type contextKey string

const sessionContextKey = contextKey("session")

// contextSetSession returns a copy of ctx carrying the authenticated session.
func contextSetSession(ctx context.Context, s session.Session) context.Context {
	return context.WithValue(ctx, sessionContextKey, s)
}

// contextGetSession returns the session placed on the request by requireAuth.
func contextGetSession(r *http.Request) (session.Session, bool) {
	s, ok := r.Context().Value(sessionContextKey).(session.Session)
	return s, ok
}
