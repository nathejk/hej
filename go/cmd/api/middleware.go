package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

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

// The admin credential (PRD 022 §8.2, tasks 369 and 371).
//
// # Why basic auth exists here at all, when nothing else in this service uses it
//
// It is an exception, and it is recorded as one in `config.adminPassword`'s doc. In short: the session
// mechanism requires the curator to be a person in this year's `person` projection with a phone number we
// hold, reachable by SMS, in the right Team section. Our photographers are frequently none of those things,
// and the tool has to work on the Tuesday after the event.
//
// # What this wrapper deliberately does NOT do
//
// **It does not create a session, and it does not touch the request context.**
//
// That is the single most important line in this file. `requireAuth` is the only place a session enters a
// request (routes.go records this as a security property, and several tests enforce it), so if this wrapper
// called `contextSetSession` the admin credential would become a **second way to authenticate as somebody**
// — and since it has no person behind it, the only way to do that would be to invent one.
//
// So the credential grants exactly this tool and nothing else. A handler behind `requireAdmin` calling
// `contextGetSession` gets `false`, which is correct: there is no caller, only a credential.
//
// # And it must not be combined with requireAuth
//
// Not `requireAuth(requireAdmin(h))` and not the reverse. Either would mean an admin route also accepts a
// participant's session, or that a participant route can be reached with the shared password. Guarded by a
// test that walks the route table.

// adminRealm is what the browser shows in its credential dialog.
//
// Danish, because the people typing into it are Danish, and specific enough to tell somebody who reached it
// by accident that they are not meant to be here.
const adminRealm = "Nathejk billedarkiv"

// requireAdmin wraps a handler so it only runs for a request carrying the admin credential.
//
// The order of the checks is load-bearing and each one is cheaper than the next:
//
//  1. **Transport.** Refuse plain HTTP outside development before looking at a credential, because the
//     credential is *in the request we are refusing* — checking it first would mean accepting a password
//     over cleartext in order to tell somebody off for sending it over cleartext.
//  2. **Headers.** `no-store` and `noindex` are set on the way in, so they are present on the 401s too.
//  3. **Rate limit.** In front of the comparison, not behind it: a limiter that only counts *failures*
//     after doing the work still lets an attacker make the server do the work.
//  4. **Comparison.** Constant-time, over both fields.
func (app *application) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !app.adminTransportOK(w, r) {
			return
		}

		// Set before any outcome, so every response from this surface carries them — including the 401, the
		// 429 and the 421. A cached admin response on a shared laptop is a leak whatever its status code.
		setAdminHeaders(w)

		if !app.allowAdminAttempt(w, r) {
			return
		}

		user, pass, ok := r.BasicAuth()
		if !ok || !app.adminCredentialOK(user, pass) {
			// One response for a missing credential, a wrong username and a wrong password. Nothing
			// distinguishes them, so this cannot be used to confirm the username — which for a shared
			// credential is half the secret.
			w.Header().Set("WWW-Authenticate", `Basic realm="`+adminRealm+`", charset="UTF-8"`)
			app.Logger.Warn("admin credential rejected", "ip", clientIP(r), "path", r.URL.Path)
			http.Error(w, "Adgang kræver login.", http.StatusUnauthorized)
			return
		}

		// No session, no context mutation. See the file's note above.
		next(w, r)
	}
}

// adminCredentialOK compares a presented credential in constant time.
//
// # Why both fields are hashed before comparing
//
// `subtle.ConstantTimeCompare` returns 0 immediately for inputs of different lengths, so comparing raw
// strings leaks the **length** of the configured password through timing. Hashing both sides to a fixed 32
// bytes first removes that: every comparison is over the same number of bytes regardless of what was sent.
//
// This is the standard idiom for exactly this reason, and it matters more here than usual because the
// password's length is the main thing standing between a shared secret and a brute force.
//
// # Why an unset credential is refused rather than allowed
//
// It should be unreachable — `adminRoutesEnabled` means these handlers are not registered without a password
// (task 370) — and it is still checked, because "unreachable" is a property of today's `routes()` and this
// function outlives it. An empty configured password matching an empty presented one would turn a
// misconfiguration into an open door, which is the one failure mode this whole design is arranged against.
func (app *application) adminCredentialOK(user, pass string) bool {
	if app.config.adminPassword == "" {
		return false
	}

	wantUser := sha256.Sum256([]byte(app.config.adminUser))
	gotUser := sha256.Sum256([]byte(user))
	wantPass := sha256.Sum256([]byte(app.config.adminPassword))
	gotPass := sha256.Sum256([]byte(pass))

	// Both compared, and **not** short-circuited with `&&`: a short-circuit would skip the password
	// comparison when the username is wrong, which is a timing signal for "that username exists".
	userOK := subtle.ConstantTimeCompare(wantUser[:], gotUser[:])
	passOK := subtle.ConstantTimeCompare(wantPass[:], gotPass[:])
	return userOK&passOK == 1
}

// adminTransportOK refuses to serve the admin surface over plain HTTP.
//
// A basic-auth credential is sent on **every** request, so plain HTTP means the password in cleartext
// repeatedly — including on requests nobody thought of as sensitive, like a thumbnail.
//
// # How "is this HTTPS" is decided behind a proxy
//
// `r.TLS` is nil in production because Traefik terminates TLS and speaks plain HTTP to this service, so
// `r.TLS` alone would refuse every real request. The proxy's `X-Forwarded-Proto` is therefore consulted —
// and it is worth being explicit that this header is only trustworthy **because** we know the only route to
// this service is through our own proxy, which sets it. It is not evidence in general.
//
// # Why development is exempt
//
// The dev stack runs over plain HTTP at `hej.local.nathejk.dk`. Refusing there would make the tool
// undevelopable, and a dev-only password on a laptop is not the secret this protects.
func (app *application) adminTransportOK(w http.ResponseWriter, r *http.Request) bool {
	if devRoutesEnabled(app.config) {
		return true
	}
	if r.TLS != nil {
		return true
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); strings.EqualFold(proto, "https") {
		return true
	}

	setAdminHeaders(w)
	app.Logger.Error("refused an admin request over plain HTTP", "ip", clientIP(r), "path", r.URL.Path)
	// 421 Misdirected Request rather than a redirect to https. A redirect would be friendlier and is wrong:
	// the credential is already in the request that arrived over cleartext, so the damage is done and the
	// honest response is to refuse rather than to invite a retry that makes the same mistake look successful.
	http.Error(w, "Dette værktøj kræver en sikker forbindelse.", http.StatusMisdirectedRequest)
	return false
}

// allowAdminAttempt rate limits credential checks by client IP.
//
// # Why by IP, and why a new limiter
//
// Every other limiter in this service is keyed on a person, and there is no person here (PRD 022 §8.9). The
// IP is the only thing an unauthenticated attempt carries.
//
// # The tradeoff, stated because it is real
//
// A curator behind the same NAT as an attacker can be locked out by that attacker. The limit is set high
// enough that this needs deliberate abuse rather than a colleague reloading, and the alternative — no limit
// — leaves a shared password exposed to an unbounded guess rate, which is the weaker position. If a curator
// is ever locked out, the diagnosis is in the log line below rather than in a mystery.
//
// Counting **every** attempt rather than only failures is deliberate: a limiter that exempts successes lets
// an attacker who has guessed correctly continue unthrottled, and one that only counts failures still has to
// do the comparison to find out.
func (app *application) allowAdminAttempt(w http.ResponseWriter, r *http.Request) bool {
	if app.adminAuthLimiter == nil {
		return true
	}
	if app.adminAuthLimiter.Allow(clientIP(r)) {
		return true
	}
	app.Logger.Warn("admin credential attempts rate limited", "ip", clientIP(r), "path", r.URL.Path)
	app.RateLimitMessageResponse(w, r, "For mange forsøg. Prøv igen om et øjeblik.")
	return false
}

// setAdminHeaders applies the two headers every admin response carries.
//
// One function so the set cannot drift between the HTML pages, the JSON endpoints, the media bytes and the
// error paths — and so a new admin handler gets them by being behind `requireAdmin` rather than by
// remembering.
//
//   - `Cache-Control: no-store` — a contact sheet of the event's photographs, cached on a shared laptop, is
//     a leak that outlives the session. `no-store` rather than `private, max-age=0` because the latter still
//     permits storage.
//   - `X-Robots-Tag: noindex, nofollow` — the public site already sends this (it is not an index we want),
//     and an *admin* page in a search index is an invitation. Belt and braces behind a password, because the
//     cost is one header.
func setAdminHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
}
