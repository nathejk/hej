// Package data is the read-side facade handed to HTTP handlers. As the domain
// grows, SQL-backed read models (one per aggregate) are exposed here so
// handlers never touch SQL directly.
package data

import "nathejk.dk/internal/users"

// Models is the read-only facade passed to handlers.
type Models struct {
	// Users resolves a normalized phone number to a user + role. The concrete
	// implementation is injected in main.go; today that is a mock directory,
	// later the real Nathejk-records lookup — handlers do not care which.
	Users users.Directory
}

// NewModels constructs the read-side facade with the given user directory.
// Additional read models will be wired in here as aggregates are added.
func NewModels(usersDir users.Directory) Models {
	return Models{Users: usersDir}
}
