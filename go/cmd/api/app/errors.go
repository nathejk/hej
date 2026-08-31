package app

import (
	"fmt"
	"net/http"
)

// errorResponse writes a JSON error envelope, logging if the write itself fails.
func (a *JsonApi) errorResponse(w http.ResponseWriter, r *http.Request, status int, message any) {
	env := Envelope{"error": message}
	if err := a.WriteJSON(w, status, env, nil); err != nil {
		a.Logger.Error("failed to write error response",
			"err", err, "method", r.Method, "uri", r.URL.RequestURI())
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// ServerErrorResponse logs the underlying error and returns a 500 to the client.
func (a *JsonApi) ServerErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	a.Logger.Error("server error",
		"err", err, "method", r.Method, "uri", r.URL.RequestURI())
	a.errorResponse(w, r, http.StatusInternalServerError,
		"the server encountered a problem and could not process your request")
}

// NotFoundResponse returns a 404 JSON error.
func (a *JsonApi) NotFoundResponse(w http.ResponseWriter, r *http.Request) {
	a.errorResponse(w, r, http.StatusNotFound, "the requested resource could not be found")
}

// ForbiddenResponse returns a 403 JSON error: the caller is authenticated, and this is not
// for them.
//
// Use it only where the *existence* of the resource is not itself a secret. Where it is —
// PRD 007's patrol lookup, where a distinguishable 403 would tell a caller which patrol
// numbers exist — answer NotFoundResponse for both cases instead, so refusal and absence
// are indistinguishable.
func (a *JsonApi) ForbiddenResponse(w http.ResponseWriter, r *http.Request) {
	a.errorResponse(w, r, http.StatusForbidden, "this resource is not available to you")
}

// MethodNotAllowedResponse returns a 405 JSON error.
func (a *JsonApi) MethodNotAllowedResponse(w http.ResponseWriter, r *http.Request) {
	a.errorResponse(w, r, http.StatusMethodNotAllowed,
		fmt.Sprintf("the %s method is not supported for this resource", r.Method))
}

// BadRequestResponse returns a 400 JSON error from the given error.
func (a *JsonApi) BadRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
	a.errorResponse(w, r, http.StatusBadRequest, err.Error())
}

// ConflictResponse returns a 409 with a caller-supplied message.
//
// Takes a message for the same reason RateLimitMessageResponse does: PRD 005's confirmation
// step surfaces the server's text to a member who is often twelve and Danish, so a generic
// "conflict" is the wrong string to put in front of them.
func (a *JsonApi) ConflictResponse(w http.ResponseWriter, r *http.Request, message string) {
	a.errorResponse(w, r, http.StatusConflict, message)
}

// BadRequestMessageResponse returns a 400 with a caller-supplied message rather than an
// error's text.
//
// Exists so a 400 that a human reads can be phrased for them. The wrong-digits case in PRD
// 005 is the motivating example: it is not really a client error at all — the likely
// explanation is that the number on file is not one the member knows — so the text must not
// read as an accusation, and wrapping a Go error string would.
func (a *JsonApi) BadRequestMessageResponse(w http.ResponseWriter, r *http.Request, message string) {
	a.errorResponse(w, r, http.StatusBadRequest, message)
}

// RateLimitResponse returns a 429 JSON error.
func (a *JsonApi) RateLimitResponse(w http.ResponseWriter, r *http.Request) {
	a.errorResponse(w, r, http.StatusTooManyRequests, "rate limit exceeded; please try again later")
}

// RateLimitMessageResponse returns a 429 with a caller-supplied message.
//
// Exists because some 429s are read by a human and some by a client. The generic message
// above is fine for the auth endpoints, whose text the UI never surfaces — but the
// portrait upload shows the server's message directly to the member, and that member is
// often twelve years old and Danish, so "rate limit exceeded" is the wrong string to put
// in front of them.
//
// Mirrors ServiceUnavailableResponse, which takes a message for the same reason.
func (a *JsonApi) RateLimitMessageResponse(w http.ResponseWriter, r *http.Request, message string) {
	a.errorResponse(w, r, http.StatusTooManyRequests, message)
}

// PayloadTooLargeResponse returns a 413 JSON error.
//
// Distinct from a 400 because the client can act on it: "too big" tells an uploader to split
// its batch and try again, which is what the track uploader's chunking does (task 083),
// whereas a generic bad request only tells it something is wrong with a body it believes is
// correct.
func (a *JsonApi) PayloadTooLargeResponse(w http.ResponseWriter, r *http.Request, err error) {
	a.errorResponse(w, r, http.StatusRequestEntityTooLarge, err.Error())
}

// InvalidCredentialsResponse returns a 401 JSON error for a failed auth attempt.
func (a *JsonApi) InvalidCredentialsResponse(w http.ResponseWriter, r *http.Request) {
	a.errorResponse(w, r, http.StatusUnauthorized, "invalid phone number or PIN")
}

// AuthenticationRequiredResponse returns a 401 for a missing/invalid session.
func (a *JsonApi) AuthenticationRequiredResponse(w http.ResponseWriter, r *http.Request) {
	a.errorResponse(w, r, http.StatusUnauthorized, "you must be authenticated to access this resource")
}

// ServiceUnavailableResponse returns a 503 for a dependency that is not available.
//
// Distinct from a 500: this app is designed to run in degraded modes on purpose — without a
// database, without a broker, or with a projection that failed to build (PRD 008 §5) — and a
// handler that cannot answer for that reason has not encountered a *problem*, it is missing a
// dependency the operator may not have configured. A client should retry later rather than
// treat it as a bug, and the log should not read as an error when nothing is broken.
func (a *JsonApi) ServiceUnavailableResponse(w http.ResponseWriter, r *http.Request, message string) {
	a.Logger.Warn("service unavailable",
		"reason", message, "method", r.Method, "uri", r.URL.RequestURI())
	a.errorResponse(w, r, http.StatusServiceUnavailable, message)
}
