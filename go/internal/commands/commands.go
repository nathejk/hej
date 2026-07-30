// Package commands is the write-side facade handed to HTTP handlers. As the
// domain grows, imperative actions (publishing events, mutating state) are
// exposed here so handlers never touch the event log directly. It is
// intentionally empty in the skeleton.
package commands

// Commands is the write-side facade passed to handlers.
type Commands struct {
}

// New constructs the write-side facade. Command constructors will be wired in
// here as aggregates are added.
func New() Commands {
	return Commands{}
}
