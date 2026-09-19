package main

import (
	"log/slog"

	"nathejk.dk/internal/publicgate"
)

// The public patrol page's gate, at the HTTP boundary (PRD 011 §0b.3, task 330).
//
// The rule itself is `internal/publicgate`, which has no opinion about HTTP. This file is the part that
// must not be got wrong per route: **every** handler serving section 2 calls `patrolPageOpen` first, and
// none of them reaches patrol data before it answers.
//
// # Why this is a function and not a middleware
//
// A middleware would be tidier and would hide the thing worth seeing. Three routes serve section 2 — the
// HTML page and two JSON endpoints — and the gate is not a property of a URL prefix: it is evaluated
// per patrol, against a number in the path. Wiring it as a middleware would mean parsing that number in
// two places, or routing everything through one prefix so the middleware could find it. Either is more
// machinery than one call at the top of three handlers.
//
// # Why it must not live in the template
//
// Because two of the three consumers are JSON. A gate applied at render time would leave
// `/api/public/patrol/:number/map` open, which is the entire course in machine-readable form — the
// worst possible place to leave it. That is the specific mistake this file exists to make impossible to
// make quietly.

// patrolGateFor resolves the gate for one patrol, or reports that the page must not be served.
//
// It answers the HTTP question ("may I serve this, and if not what do I send?") and leaves the domain
// question to publicgate. Three outcomes:
//
//   - open, with the reason — serve the page, and let the reason decide whether a diploma appears;
//   - not open — send the not-yet answer, which is deliberately indistinguishable from "no such patrol";
//   - unavailable — the gate could not be evaluated at all.
//
// **The second and third collapse to the same response**, and that is the point: a visitor must not be
// able to tell "not yet" from "does not exist" from "we cannot tell", because the differences between
// them leak exactly what §6 says must not leak — which patrol numbers are real, and which patrols have
// finished, live, during the race.
func (app *application) patrolGateFor(patrolID string) (publicgate.Reason, bool) {
	// Nil when the app runs without a database or the projections failed to build. Fails closed,
	// unlike most nil-projection paths in this service, which answer 503. The difference is what the two
	// answers publish: a 503 here would confirm that a patrol number exists and that we simply cannot
	// serve it yet, which is one bit more than a stranger should get from a closed gate.
	if app.publicGate == nil {
		return publicgate.Closed, false
	}

	reason, err := app.publicGate.For(app.config.eventYear, patrolID)
	if err != nil {
		// Logged, not surfaced. The gate failing is our problem; the visitor gets the same answer they
		// would get before the race, because any distinguishable error response is a probe.
		app.Logger.Error("evaluating the public patrol gate", "patrol", patrolID, "err", err)
		return publicgate.Closed, false
	}
	return reason, reason.Open()
}

// publicGateOverride builds the override predicate from configuration.
//
// A set built once at startup rather than a scan of the slice per request: the list is tiny, but this is
// consulted on an unauthenticated route and there is no reason to make a stranger's request do linear
// work.
//
// Empty configuration yields a predicate that is false for everything — including, importantly, for the
// empty patrol id. `splitCSV` drops empties precisely so an unset variable cannot produce a set
// containing "", which would otherwise make every personnel user's non-existent patrol "overridden".
func publicGateOverride(ids []string, logger *slog.Logger) func(string) bool {
	if len(ids) == 0 {
		return func(string) bool { return false }
	}
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	// Logged at startup because a manual override is a deliberate deviation from the rule, and the next
	// person to wonder why one patrol's page is open early should be able to find out from the log
	// rather than from the environment.
	logger.Info("public patrol pages force-opened by configuration", "patrols", ids)
	return func(patrolID string) bool { return patrolID != "" && set[patrolID] }
}

// A note on the closed-gate *response*, which is deliberately not here.
//
// Every closed path must answer identically to "no such patrol" — status and body — or the difference
// becomes a way to enumerate patrols and to watch the field finish in real time. The response itself
// belongs with the page it mimics, so it lives in publicsite.go as `renderPatrolNotYet` (task 332).
// The property to preserve as branches are added: one function producing that answer, called from
// every closed branch, so the two cannot drift apart.
