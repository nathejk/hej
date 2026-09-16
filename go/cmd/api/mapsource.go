package main

import (
	"log/slog"

	"nathejk.dk/internal/data"
	"nathejk.dk/internal/mapfixture"
	"nathejk.dk/internal/reveal"
)

// mapReadsFor picks the patrol-scoped map reads: the real reveal rule when there is one, the fixture world
// when the app is running without a database, and **nothing** when the projections exist but failed.
//
// # Three outcomes, not two, and the third is the important one
//
// `scanSourceFor` has a simple choice — real projection or mock — because a missing scan projection can only
// mean "no database". The map rule is composed from five projections, so it has a third state: a broker is
// configured and one of them did not build. That is a real misconfiguration, and the right answer there is
// the honest 503 the handlers give for a nil interface (see the comment where the rule is composed: an
// honest 503 is recoverable, a cached empty map is not). Serving fixtures in that state would be strictly
// worse than serving nothing, because a plausible-looking map is indistinguishable from a correct one.
//
// So the fixture world is chosen by the **total absence of eventing**, which is the supported
// no-database development mode (PRD 008 §5) — never by an environment flag, and never as a fallback for a
// failure. A deployment with a broker and a database always gets the real rule or an explicit outage.
//
// # Why the fixtures are worth wiring at all
//
// Without them, `/api/checkpoints` and `/api/patrol/handouts` answer 503 in development, so the markers,
// the edge arrows, the verdict badges and the whole "Kort udleveret" section are unreachable without a
// broker, a database and a replay of a real event. Most of this feature's states cannot be produced by hand
// at all (task 270). See the mapfixture package doc for the world it describes.
func mapReadsFor(rule *reveal.Rule, hasEventing bool, logger *slog.Logger) data.MapReads {
	if rule != nil {
		return rule
	}
	if hasEventing {
		// Projections were expected and are not here. Untyped nil, so `app.models.Maps == nil` holds and
		// the handlers answer 503 rather than dereferencing a nil pointer (the classic Go trap
		// `mapReadsOrNil` documents).
		return nil
	}
	logger.Warn("no map projections: serving the fixture reveal world (development fallback)",
		"patrol", mapfixture.PatrolID)
	return mapfixture.NewRule()
}
