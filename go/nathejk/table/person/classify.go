package person

import (
	"strings"
)

// AppRole values, duplicated from users.Role by necessity.
//
// This package cannot import nathejk.dk/internal/users — it is bound for shared-go,
// and Go forbids importing another module's internal tree (see the package doc). So
// the strings are repeated here rather than referenced.
//
// They are a wire format on both sides: the same values travel in GET /api/me and
// are compared in the frontend router guard. users.RoleValuesAreStable pins them
// there; changing one is a data migration, not a rename.
const (
	RoleSpejder      = "spejder"
	RoleBandit       = "bandit"
	RolePostmandskab = "postmandskab"
	RoleGuide        = "guide"
	RoleSamarit      = "samarit"
	RoleGoegler      = "gøgler"
	RoleCrew         = "crew"
)

// crewFunctionBySlug maps an organizer-authored section slug to a crew app role.
//
// This is the weakest link in the whole directory, and it is worth being honest
// about why rather than making it look authoritative:
//
// Crew function is not modelled anywhere upstream. A crew member carries a
// `sectionSlug` pointing at a row in the organizer-editable `section` tree, and
// tilmelding's crew signup UI treats first-level sections as "crew functions" —
// which is the only place that intent is written down. There is no enum, nothing
// validates the values, and an organizer can rename a section at any time.
//
// So this map is a **convention**, and the fallback matters more than the map:
// an unrecognised slug yields RoleCrew (least-privileged) rather than an error,
// because locking someone out of a safety app over a spelling is not acceptable.
//
// The better long-term answer is to project `section.Type`, which the
// NathejkSectionAdded message already carries and the shared-go projector currently
// drops (PRD 006 §11 Q2). Until that happens, extend this map — and task 078 exists
// to check it against the organizers' real section names before an event.
var crewFunctionBySlug = map[string]string{
	// Checkpoint staff.
	"postmandskab": RolePostmandskab,
	"post":         RolePostmandskab,
	"poster":       RolePostmandskab,
	"postmandskb":  RolePostmandskab, // observed misspelling; harmless to accept

	// Guides.
	"guide":  RoleGuide,
	"guider": RoleGuide,

	// Medics.
	"samarit":     RoleSamarit,
	"samaritter":  RoleSamarit,
	"førstehjælp": RoleSamarit,
}

// normalizeSlug folds a slug to its comparison form.
//
// Case and surrounding whitespace are noise from a hand-typed admin field. Danish
// characters are deliberately NOT folded to ASCII: "førstehjælp" and a hypothetical
// "forstehjaelp" are different strings an organizer might type, and silently
// treating them as equal would hide a data problem task 078 should surface. Instead,
// both spellings can be added to the map explicitly if they occur.
func normalizeSlug(slug string) string {
	return strings.ToLower(strings.TrimSpace(slug))
}

// ClassifyCrew maps a section slug to a crew app role.
//
// ok=false means the slug was not recognised: the caller gets RoleCrew and is
// expected to log it, so an unmapped slug is visible rather than silently absorbed.
// It is not an error — see the note on crewFunctionBySlug.
func ClassifyCrew(sectionSlug string) (role string, ok bool) {
	slug := normalizeSlug(sectionSlug)
	if slug == "" {
		// No section assigned yet. A real state in the upstream data (crewmember has
		// an `Unassigned` filter), not a data problem, so it is not reported as an
		// unmapped slug.
		return RoleCrew, true
	}
	if r, found := crewFunctionBySlug[slug]; found {
		return r, true
	}
	return RoleCrew, false
}

// Population is where a person came from in the upstream data, as distinct from the
// app role they end up with. Keeping the two apart is the point of this file: the
// upstream vocabulary is team types and entity kinds, the app's is roles, and only
// this package translates.
type Population int

const (
	PopulationUnknown Population = iota
	// PopulationSpejder is a `spejder` row; its team is a patrulje.
	PopulationSpejder
	// PopulationSenior is a `senior` row; its team is a klan. These are the people
	// the event calls banditter — "bandit" is not a field anywhere, it is the event
	// role a senior plays. The giveaway is the subject vocabulary: shared-go's
	// senior projector consumes NATHEJK.*.bandit.*.armNumber.assigned and writes
	// senior.armNumber.
	PopulationSenior
	// PopulationCrew is a `crewmember` row; function comes from its section slug.
	PopulationCrew
	// PopulationGoegler comes from NATHEJK.*.gøgler.* events. Note these people do
	// not exist in shared-go at all — hq keeps them in its local personnel table
	// (PRD 006 §11 Q4).
	PopulationGoegler
)

// Classify maps an upstream population (plus a section slug for crew) to an app
// role.
//
// ok=false only ever means "crew slug not recognised"; every other population maps
// unconditionally. An unknown population also yields RoleCrew with ok=false, on the
// same least-privilege reasoning: if we cannot tell what someone is, they get the
// fallback rather than a guess.
func Classify(p Population, sectionSlug string) (role string, ok bool) {
	switch p {
	case PopulationSpejder:
		return RoleSpejder, true
	case PopulationSenior:
		return RoleBandit, true
	case PopulationGoegler:
		return RoleGoegler, true
	case PopulationCrew:
		return ClassifyCrew(sectionSlug)
	default:
		return RoleCrew, false
	}
}

// HasGuardianPhone reports whether a population carries a guardian/emergency
// contact number.
//
// Only spejder do — confirmed against shared-go, where PhoneParent exists on the
// `spejder` table and on no other population. PRD 005's confirmation step is
// spejder-only for exactly this reason, and PRD 003 must render "not applicable"
// rather than a blank field for everyone else.
func HasGuardianPhone(p Population) bool {
	return p == PopulationSpejder
}
