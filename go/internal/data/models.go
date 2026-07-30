// Package data is the read-side facade handed to HTTP handlers. As the domain
// grows, SQL-backed read models (one per aggregate) are exposed here so
// handlers never touch SQL directly. It is intentionally empty in the skeleton.
package data

// Models is the read-only facade passed to handlers.
type Models struct {
}

// NewModels constructs the read-side facade. Read models will be wired in here
// as aggregates are added.
func NewModels() Models {
	return Models{}
}
