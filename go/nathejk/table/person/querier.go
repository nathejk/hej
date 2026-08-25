package person

import (
	"database/sql"
	"time"

	"github.com/jrgensen/cqrs"
)

// Person is one row of the directory.
//
// Nullable columns are pointers so the caller can tell "not applicable" from
// "empty". That matters most for PhoneParent: only spejder have a guardian number,
// so nil means "this population does not have one" while a pointer to "" would mean
// "should have one and it is missing" — PRD 005's confirmation step and PRD 003's
// profile page render those two differently.
type Person struct {
	PersonID string
	Year     string
	AppRole  string

	Name        string
	Phone       string
	PhoneParent *string

	Address    string
	PostalCode string
	City       string
	Email      string
	Birthday   *time.Time

	TeamID   string
	TeamName string

	MemberStatus string
	ArmNumber    string

	VerifiedAt        *time.Time
	AcknowledgedPhone *string
	PortraitRef       string
}

// Queries is the read API handed to the application through data.Models.
//
// Lookup returns a slice rather than a single Person on purpose: two people can
// share a phone number, and an interface returning one value would bake a
// collision policy into the type signature where nobody can see it. Task 071 owns
// the policy; this shape lets it be a decision rather than an accident.
type Queries interface {
	// Lookup returns every person in the given year whose normalized phone number
	// matches. Empty slice, not an error, when nothing matches.
	Lookup(year, normalizedPhone string) ([]Person, error)

	// Get resolves a person by id, scoped to a year.
	Get(year, personID string) (Person, bool, error)
}

type querier struct {
	db cqrs.Reader
}

const personColumns = `
	personId, year, appRole,
	name, phone, phoneParent,
	address, postalCode, city, email, birthday,
	teamId, teamName,
	memberStatus, armNumber,
	verifiedAt, acknowledgedPhone, portraitRef`

// Lookup finds people by normalized phone number.
//
// Soft-deleted rows are excluded here rather than at the call site: a deleted member
// must lose their login, and leaving that filter to every caller is how one of them
// eventually forgets (task 076).
func (q querier) Lookup(year, normalizedPhone string) ([]Person, error) {
	// Guard against an empty phone matching the column default. Without this, any
	// row that has no phone recorded would answer a lookup for "" — which is what a
	// caller passes when normalization fails.
	if normalizedPhone == "" {
		return nil, nil
	}

	rows, err := q.db.Query(`
		SELECT `+personColumns+`
		FROM person
		WHERE year = ? AND phone = ? AND deleted = 0
		ORDER BY personId`, year, normalizedPhone)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Person
	for rows.Next() {
		p, err := scanPerson(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (q querier) Get(year, personID string) (Person, bool, error) {
	row := q.db.QueryRow(`
		SELECT `+personColumns+`
		FROM person
		WHERE year = ? AND personId = ? AND deleted = 0`, year, personID)

	p, err := scanPerson(row)
	if err == sql.ErrNoRows {
		return Person{}, false, nil
	}
	if err != nil {
		return Person{}, false, err
	}
	return p, true, nil
}

// scanner covers both *sql.Row and *sql.Rows so one scan body serves Get and Lookup.
type scanner interface {
	Scan(dest ...any) error
}

func scanPerson(s scanner) (Person, error) {
	var p Person
	err := s.Scan(
		&p.PersonID, &p.Year, &p.AppRole,
		&p.Name, &p.Phone, &p.PhoneParent,
		&p.Address, &p.PostalCode, &p.City, &p.Email, &p.Birthday,
		&p.TeamID, &p.TeamName,
		&p.MemberStatus, &p.ArmNumber,
		&p.VerifiedAt, &p.AcknowledgedPhone, &p.PortraitRef,
	)
	return p, err
}

var _ Queries = querier{}
