package main

// The photographer admin surface (PRD 022 §8.2, tasks 369–371).
//
// Everything on it is registered conditionally, in routes(), on the app having an admin password. A binary
// with none serves no route defined here at all — the check is "is this route registered", not "is this
// caller allowed", which is the rule dev.go already states and which matters more here than it does there.
//
// # Absent, not open
//
// This is the distinction the whole design rests on. A default credential, or a guard that waves requests
// through when the config is empty, means a misconfigured deploy publishes an **anonymous bulk upload
// endpoint** on the public internet — a write path into the blob store, which is the one thing in this
// service that cannot be rebuilt from the event log (PRD 008 §8).
//
// So there is no default password, in any environment, including development. The cost is that a developer
// must set one to see the tool; the benefit is that there is no value of the configuration that produces an
// unprotected upload.
//
// # And it is unattended between events
//
// The tool is used hard for a week and then not at all for a year. That is exactly the situation in which a
// stale deployment goes unnoticed, so turning it off has to be something somebody will actually do: deleting
// the password is that, and it takes the routes with it.

import (
	"time"

	"nathejk.dk/internal/ratelimit"
)

// adminRoutesEnabled reports whether the admin routes should be registered.
//
// Keyed on the **password** alone, not on the username.
//
// That asymmetry is deliberate. A username is not a secret — it will be "nathejk" or "foto" — so requiring
// both would mean a deploy that set only `ADMIN_USER` looks half-configured and serves nothing, which is
// fine, while a deploy that set only `ADMIN_PASSWORD` would serve a tool whose username is the empty string.
// The second is the dangerous one to allow by accident, and it is the one this predicate refuses: with a
// password set and no username, the credential is `"" / <password>`, which is a real credential and is
// checked as one. An operator who wants a username sets one.
//
// `ENV` is deliberately **not** part of this. Unlike the dev routes, the admin tool is a production feature —
// it is where the event's photographs actually arrive — so gating it on the environment would be gating the
// wrong thing.
func adminRoutesEnabled(cfg config) bool {
	return cfg.adminPassword != ""
}

// adminAuthLimiterFor builds the credential-attempt limiter, or nil when the tool is not served.
//
// Nil rather than a limiter nobody consults, matching `limiterOrNil`'s shape for the glimt limits: there are
// no routes to protect, so allocating a map to throttle requests that answer 404 would be ceremony.
func adminAuthLimiterFor(cfg config) *ratelimit.Limiter {
	if !adminRoutesEnabled(cfg) {
		return nil
	}
	return ratelimit.New(adminAuthAttemptsPerHour, time.Hour)
}

// adminAuthAttemptsPerHour is the credential-attempt ceiling per client IP.
//
// A constant rather than configuration, unlike the glimt limits. Those are tuned against observed member
// behaviour and were changed once already (task 324); this one guards a secret, and making it configurable
// would mean the number protecting a shared password can be raised by whoever is debugging a lockout at the
// time. The reasoning for the value is at the assignment in main.go.
const adminAuthAttemptsPerHour = 30
