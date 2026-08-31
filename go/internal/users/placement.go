package users

import "strings"

// Directory placement (PRD 007 §6, task 152).
//
// Placement answers "where is this person listed", which is a different question from
// "what may this person see" (contacts.go). A crew member whose section slug is `bandit`
// is out on the race as a bandit and is listed among banditter — but their Role is still
// crew, so they view as crew and keep the patrol lookup.
//
// # Do not "fix" the role classifier to match
//
// Both special slugs already exist in `nathejk/table/person/classify.go`, where they map
// to RoleCrew, and that must stay true. Making slug `bandit` map to RoleBandit would look
// like a tidy-up and would in fact:
//
//   - hand a crew member the bandit's view of the pane, and
//   - take away their patrol lookup, which is a safety capability.
//
// `classify.go` already carries the same warning for `goeglerledelse`, which is
// deliberately not RoleGoegler because that role is for the performers. So there are two
// maps over the same slugs, answering two different questions. That is intentional
// duplication, not an oversight.
//
// # Magic slugs, for now
//
// This is string matching against organizer-authored slugs, and it is expected to migrate
// to a **section flag** upstream. That puts it in the same position as the role map after
// task 078: a mechanism rather than a stopgap. When the flag arrives, this map's *source*
// changes and its structure does not — so keep the mapping in this one place.

// crewPopulationBySlug places a crew member into a participant population.
//
// Only slugs that move someone *out* of the crew listing belong here. Every other crew
// section — hq, koekken, rover, hoensegaard, pr, team, noedtelefon — is crew and is
// listed as crew, which is why there is no "unmapped slug" condition in this map and
// nothing to log: the default is *correct*, not a fallback.
//
// That is the opposite of the role map in classify.go, where an unrecognised slug means
// classification failed and the caller logs it. Worth stating, because the two maps look
// similar enough that someone will reasonably wonder why only one of them warns.
var crewPopulationBySlug = map[string]Population{
	// Crew who are out on the race as banditter. Listed with the banditter, in their
	// klan, because operationally that is who they are that night.
	"bandit": PopulationBandit,
	// Gøglerledelse — the crew who run the gøgler operation. Listed with the gøglere,
	// which is what PRD 007 means by "including section gøglere".
	"goeglerledelse": PopulationGoegler,
}

// normalizeSlug folds a slug to its comparison form, matching classify.go's treatment so
// the two maps cannot disagree about whitespace or case.
func normalizeSlug(slug string) string {
	return strings.ToLower(strings.TrimSpace(slug))
}

// PopulationsOf returns every population the user is listed under, in a stable order.
//
// It returns a slice rather than a single value because a crew bandit genuinely appears
// **twice**: among the banditter (for a bandit viewer) and among the crew (for a crew
// viewer). Hiding them from one of the two would make a real colleague unfindable in the
// list where someone is looking for them.
//
// Which of those groups a given viewer sees them in is not decided here. The manifest is
// built per viewer, so it intersects these populations with what that viewer may list —
// a bandit sees them grouped by klan, a crew member sees them among crew, and neither
// needs to know the other view exists.
func PopulationsOf(u User) []Population {
	switch {
	case u.Role == RoleSpejder:
		return []Population{PopulationSpejder}
	case u.Role == RoleBandit:
		return []Population{PopulationBandit}
	case u.Role == RoleGoegler:
		return []Population{PopulationGoegler}
	case u.Role.IsCrew():
		if p, found := crewPopulationBySlug[normalizeSlug(u.SectionSlug)]; found {
			// Crew first or participant first does not matter to callers, which
			// intersect rather than take the head — but the order is fixed so tests
			// and payloads are stable.
			return []Population{p, PopulationCrew}
		}
		return []Population{PopulationCrew}
	}
	// An unknown role reaches no population. It cannot happen through the directory,
	// which refuses to build a User with an unrecognised role, but returning nothing is
	// the safe answer if it ever does.
	return nil
}

// IsListedIn reports whether the user appears in the given population's listing.
func IsListedIn(u User, p Population) bool {
	for _, own := range PopulationsOf(u) {
		if own == p {
			return true
		}
	}
	return false
}
