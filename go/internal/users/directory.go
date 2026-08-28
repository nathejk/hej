// Package users defines the directory interface the BFF uses to recognize a
// phone number and look up the associated user + role. This skeleton ships
// only a mock implementation (seeded phone → role map); the real
// Nathejk-records lookup replaces it later without touching handlers.
package users

// Role is the app-level role a recognized user has. It drives which pages the
// bottom navigation shows.
//
// These are *app* roles, not signup categories. The distinction matters because the
// upstream data uses a different vocabulary: shared-go has `TeamType`
// (patrulje/klan/crew/gøgler) and `UserType` (gøgler/crew), and neither lines up
// one-to-one with this list — a klan senior is a `bandit` here, and `crew` splits
// into three functions. PRD 006's person projection owns that translation, so this
// enum stays the app's own vocabulary and nothing infers a role from a team type.
type Role string

const (
	RoleSpejder      Role = "spejder"
	RoleBandit       Role = "bandit"
	RolePostmandskab Role = "postmandskab"
	RoleGuide        Role = "guide"
	RoleSamarit      Role = "samarit"
	// RoleGoegler is the entertainers/jugglers ("gøglere") who staff posts. The
	// constant name is ASCII while the value keeps the Danish spelling, because the
	// value is what appears on the wire and in the upstream subjects
	// (NATHEJK.*.gøgler.*).
	RoleGoegler Role = "gøgler"
	// RoleCrew is the fallback for a crew member whose function could not be
	// determined.
	//
	// It exists because crew function comes from an organizer-authored section slug
	// with nothing validating it (PRD 006 §8), so an unrecognised slug is a routine
	// data condition rather than an error — and locking someone out of a safety app
	// over a spelling is not acceptable.
	//
	// It must be treated as **least-privileged**, not as "generic crew with crew
	// powers". An account lands here precisely because classification *failed*, so
	// granting it what an identified samarit gets would mean a typo in a slug
	// silently widens access — PRD 007's portrait access matrix depends on this
	// distinction, since identified crew may see every portrait in the event.
	RoleCrew Role = "crew"
)

// AllRoles is every role this app knows, in a stable order. Anything that has to
// enumerate roles (tests, an access matrix, a nav audit) should use this rather than
// its own list, which is how a newly added role gets silently missed.
var AllRoles = []Role{
	RoleSpejder,
	RoleBandit,
	RolePostmandskab,
	RoleGuide,
	RoleSamarit,
	RoleGoegler,
	RoleCrew,
}

// Valid reports whether r is a role this version of the code knows.
func (r Role) Valid() bool {
	for _, known := range AllRoles {
		if r == known {
			return true
		}
	}
	return false
}

// IsCrew reports whether r is one of the crew functions, including the
// unclassified fallback.
//
// Deliberately does **not** imply any privilege: callers that gate on "is crew"
// for access decisions must check the specific function instead, because RoleCrew
// means "we do not know what this person does".
func (r Role) IsCrew() bool {
	switch r {
	case RolePostmandskab, RoleGuide, RoleSamarit, RoleCrew:
		return true
	}
	return false
}

// User is what the directory returns for a recognized phone number. It is the
// single place where per-user attributes accumulate, so a new consumer (a
// patrol-scoped read, a profile page) needs a field here rather than its own
// lookup path.
type User struct {
	ID   string
	Role Role

	// Name is the person's display name. It exists for one specific job: when a phone
	// number is shared, the disambiguation prompt (PRD 006 §11, task 079) shows the
	// candidates' names so the holder can pick themselves. It is the *minimum* that
	// makes that possible — a first name and maybe a team — and deliberately not a
	// fuller profile, since one person is being shown another's details.
	Name string

	// PatrolID identifies the patrol (patrulje/banditgruppe) the user belongs
	// to, or "" for personnel roles that have none. Consumers must treat the
	// empty value as "no patrol" rather than an error: personnel are legitimate
	// users, they simply have nothing patrol-scoped to show.
	PatrolID string
	// PatrolName is the human-readable patrol name, for display only. A patrulje for
	// a spejder, a klan for a bandit.
	PatrolName string

	// Section is the crew affiliation, for display only — empty for everyone who
	// belongs to a team instead.
	//
	// It exists for the same reason PatrolName is exposed: when a phone number is
	// shared, "which patrulje" or "which section" is often the only thing that tells
	// two candidates apart, and two siblings in the same patrol or two crew in the
	// same section need something better than identical rows (task 079).
	Section string

	// The fields below are the user's *own* details, shown back to them on the
	// profile page (PRD 003). Unlike Name and Section they are never shown to
	// anybody else — in particular the login chooser must keep displaying only
	// Name + PatrolName/Section, because that list shows one person something
	// about the others on their number.

	// Phone is the user's own normalized number. Note it is NOT a login key for
	// PhoneParent's holder — see person.querier.Lookup on why the guardian number
	// never authenticates anyone.
	Phone string

	// PhoneParent is the guardian's number, and is a pointer because nil and ""
	// mean different things:
	//
	//	nil — this population has no guardian number (bandit, crew, gøgler)
	//	"" — one is expected and missing (a spejder with nothing on file)
	//
	// The profile page hides the row for the first and shows "Ikke registreret"
	// for the second, so collapsing them here would destroy the distinction the
	// person projection deliberately preserves.
	PhoneParent *string

	// Address, PostalCode and City are the postal address as held upstream. Kept as
	// three fields rather than one pre-joined string so the client owns formatting.
	Address    string
	PostalCode string
	City       string
}

// Directory resolves users, either by their **normalized** phone number (login)
// or by their user id (an established session). found=false means the user is
// not recognized. Handlers must treat found=false identically to found=true
// from the client's perspective (anti-enumeration) — verification simply never
// succeeds for an unknown number.
//
// Keeping both lookups behind this one interface is deliberate: the real
// Nathejk-records implementation can serve them from the same query, and
// handlers never learn where the data came from.
//
// # Phone numbers are not unique
//
// One number can belong to several people: siblings sharing a phone, or a
// guardian's number entered as the scout's own. That is ordinary data, not an
// error, so the interface exposes both shapes:
//
//	LookupAll — every match, for the login flow that has to disambiguate
//	Lookup    — the single match, and NOT FOUND when there is more than one
//
// Lookup's ambiguity rule is the important part. It is defined that way so a
// caller that has not thought about collisions cannot silently log someone in as
// the wrong person — the failure mode is a refused login (visible, fixable) rather
// than one sibling reading the other's profile.
type Directory interface {
	// LookupAll returns every user registered with the given normalized phone
	// number. An empty slice means the number is not recognized.
	//
	// Order is stable, so a disambiguation prompt does not reshuffle between
	// requests.
	LookupAll(phone string) []User

	// Lookup returns the user for a normalized phone number when exactly one is
	// registered with it. found=false covers both "unknown number" and
	// "ambiguous" — see the note above on why those collapse.
	Lookup(phone string) (user User, found bool)

	// Get resolves the user behind an id carried by a session cookie.
	Get(id string) (user User, found bool)
}
