package main

import (
	"log/slog"
	"sync/atomic"

	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/person"
)

// This file is the seam PRD 001 designed for: it turns the person projection into a
// users.Directory so login and role-gated navigation read real data, without any handler
// learning where that data came from.

// personDirectory adapts the person projection to users.Directory.
//
// Two things the interface does not carry and this adapter supplies:
//
//   - **The year.** users.Directory is phone-in, user-out; the projection is keyed per
//     event year. The year is fixed at construction rather than derived per call, so
//     every lookup in a request agrees about which event is running.
//   - **Role translation.** person stores app-role strings that are deliberate
//     duplicates of users.Role's values (see classify.go), so this is a conversion, not
//     an inference — the projection owns the classification and nothing here re-derives
//     a role from a team type or a section.
type personDirectory struct {
	queries person.Queries
	year    string
	logger  *slog.Logger
}

func newPersonDirectory(q person.Queries, year string, logger *slog.Logger) *personDirectory {
	return &personDirectory{queries: q, year: year, logger: logger}
}

// LookupAll returns every user registered with a normalized phone number.
//
// A query error yields an empty slice, not a panic and not a distinguishable failure.
// That is the anti-enumeration contract from users.Directory: the caller must not be able
// to tell "unknown number" from "database hiccup" from "known number", because any of
// those becoming visible turns the login form into a membership oracle. The error is
// logged so the operator sees what the client cannot.
func (d *personDirectory) LookupAll(phone string) []users.User {
	people, err := d.queries.Lookup(d.year, phone)
	if err != nil {
		d.logger.Error("directory lookup failed", "err", err)
		return nil
	}

	out := make([]users.User, 0, len(people))
	for _, p := range people {
		u, ok := d.toUser(p)
		if !ok {
			continue
		}
		out = append(out, u)
	}
	return out
}

// Lookup returns the single owner of a number, and NOT FOUND when several people share
// it.
//
// The ambiguity rule is users.Directory's, not this adapter's invention: a caller who has
// not thought about shared numbers gets a refused login rather than being silently signed
// in as somebody's sibling. The login flow that *has* thought about it uses LookupAll and
// the task 079 chooser.
func (d *personDirectory) Lookup(phone string) (users.User, bool) {
	matches := d.LookupAll(phone)
	if len(matches) != 1 {
		return users.User{}, false
	}
	return matches[0], true
}

// Get resolves the user behind an id carried by a session cookie.
func (d *personDirectory) Get(id string) (users.User, bool) {
	p, found, err := d.queries.Get(d.year, id)
	if err != nil {
		d.logger.Error("directory get failed", "err", err)
		return users.User{}, false
	}
	if !found {
		return users.User{}, false
	}
	return d.toUser(p)
}

// toUser converts a directory row into what handlers see.
//
// It refuses a person whose app role this build does not recognise, rather than passing
// the string through or substituting a default. Both alternatives are worse:
//
//   - Passing it through puts an unknown value in the session and in GET /api/me, where
//     the frontend router guard compares against a fixed enum and would silently show
//     a member the wrong navigation.
//   - Defaulting to RoleCrew would *grant* access off the back of a data problem. RoleCrew
//     is the least-privileged fallback for a crew member whose section is unrecognised,
//     which is a specific, understood condition; "we have no idea what this row is" is
//     not the same thing and must not borrow its privileges.
//
// So an unclassified row cannot log in, which is visible and fixable, and is logged here
// because from the client's side it is indistinguishable from an unknown number. On the
// current data this never fires: every one of the 3,278 projected rows carries one of
// spejder/bandit/crew/gøgler.
func (d *personDirectory) toUser(p person.Person) (users.User, bool) {
	role := users.Role(p.AppRole)
	if !role.Valid() {
		d.logger.Warn("person has an unrecognised app role, refusing login",
			"personId", p.PersonID, "appRole", p.AppRole)
		return users.User{}, false
	}

	return users.User{
		ID:   p.PersonID,
		Role: role,
		Name: p.Name,
		// A patrulje for a spejder, a klan for a bandit; empty for crew and gøglere,
		// who have a section or nothing. Consumers must read empty as "no patrol"
		// rather than as an error.
		PatrolID:   p.TeamID,
		PatrolName: p.TeamName,
		Section:    p.SectionName,
		// Own details for PRD 003's profile page. PhoneParent is passed through as a
		// pointer, not dereferenced with a default: nil ("no guardian number in this
		// population") and "" ("expected, missing") are rendered differently, and
		// flattening them here is exactly the information loss the projection avoids.
		Phone:       p.Phone,
		PhoneParent: p.PhoneParent,
		Address:     p.Address,
		PostalCode:  p.PostalCode,
		City:        p.City,
	}, true
}

var _ users.Directory = (*personDirectory)(nil)

// switchableDirectory lets the application start on the mock and move to the real
// projection once it exists.
//
// This indirection is not gold-plating; it is forced by the startup order. The projection
// is built inside the broker's connect callback — deliberately, so a database-only run
// does not create tables nothing will fill (see main.go) — and that callback runs in a
// goroutine, after the HTTP server is already assembled and possibly already serving.
// Handlers therefore hold a users.Directory that does not exist yet.
//
// The alternatives were to build the projection eagerly, which reintroduces the table
// problem and makes a slow broker delay the API, or to have handlers check for nil, which
// spreads a startup concern across every call site.
//
// An atomic pointer rather than a mutex because the read path is every authenticated
// request and the write path happens once.
type switchableDirectory struct {
	current atomic.Pointer[users.Directory]
}

func newSwitchableDirectory(initial users.Directory) *switchableDirectory {
	s := &switchableDirectory{}
	s.set(initial)
	return s
}

// set installs the directory all subsequent lookups will use.
func (s *switchableDirectory) set(d users.Directory) {
	s.current.Store(&d)
}

func (s *switchableDirectory) LookupAll(phone string) []users.User {
	return (*s.current.Load()).LookupAll(phone)
}

func (s *switchableDirectory) Lookup(phone string) (users.User, bool) {
	return (*s.current.Load()).Lookup(phone)
}

func (s *switchableDirectory) Get(id string) (users.User, bool) {
	return (*s.current.Load()).Get(id)
}

var _ users.Directory = (*switchableDirectory)(nil)
