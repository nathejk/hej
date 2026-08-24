// Package users defines the directory interface the BFF uses to recognize a
// phone number and look up the associated user + role. This skeleton ships
// only a mock implementation (seeded phone → role map); the real
// Nathejk-records lookup replaces it later without touching handlers.
package users

// Role is the app-level role a recognized user has. It drives which pages the
// bottom navigation shows.
type Role string

const (
	RoleSpejder      Role = "spejder"
	RoleBandit       Role = "bandit"
	RolePostmandskab Role = "postmandskab"
	RoleGuide        Role = "guide"
	RoleSamarit      Role = "samarit"
)

// User is what the directory returns for a recognized phone number. It is the
// single place where per-user attributes accumulate, so a new consumer (a
// patrol-scoped read, a profile page) needs a field here rather than its own
// lookup path.
type User struct {
	ID   string
	Role Role

	// PatrolID identifies the patrol (patrulje/banditgruppe) the user belongs
	// to, or "" for personnel roles that have none. Consumers must treat the
	// empty value as "no patrol" rather than an error: personnel are legitimate
	// users, they simply have nothing patrol-scoped to show.
	PatrolID string
	// PatrolName is the human-readable patrol name, for display only.
	PatrolName string
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
type Directory interface {
	Lookup(phone string) (user User, found bool)
	// Get resolves the user behind an id carried by a session cookie.
	Get(id string) (user User, found bool)
}
