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

// MethodNotAllowedResponse returns a 405 JSON error.
func (a *JsonApi) MethodNotAllowedResponse(w http.ResponseWriter, r *http.Request) {
	a.errorResponse(w, r, http.StatusMethodNotAllowed,
		fmt.Sprintf("the %s method is not supported for this resource", r.Method))
}

// BadRequestResponse returns a 400 JSON error from the given error.
func (a *JsonApi) BadRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
	a.errorResponse(w, r, http.StatusBadRequest, err.Error())
}

// RateLimitResponse returns a 429 JSON error.
func (a *JsonApi) RateLimitResponse(w http.ResponseWriter, r *http.Request) {
	a.errorResponse(w, r, http.StatusTooManyRequests, "rate limit exceeded; please try again later")
}

// InvalidCredentialsResponse returns a 401 JSON error for a failed auth attempt.
func (a *JsonApi) InvalidCredentialsResponse(w http.ResponseWriter, r *http.Request) {
	a.errorResponse(w, r, http.StatusUnauthorized, "invalid phone number or PIN")
}
