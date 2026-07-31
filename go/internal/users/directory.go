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

// User is what the directory returns for a recognized phone number.
type User struct {
	ID   string
	Role Role
}

// Directory resolves a **normalized** phone number to a user. found=false means
// the number is not recognized. Handlers must treat found=false identically to
// found=true from the client's perspective (anti-enumeration) — verification
// simply never succeeds for an unknown number.
type Directory interface {
	Lookup(phone string) (user User, found bool)
}
