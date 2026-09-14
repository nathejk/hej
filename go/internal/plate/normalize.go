// Package plate normalizes free-form licence-plate input into the single
// canonical form the vehicle inventory compares and stores: a two-letter ISO
// country code, a plus, and the registration, e.g. "DK+AB12345".
//
// Shaped deliberately like internal/phone, and for the same reason. A value that
// has several spellings and no single normalizer cannot be compared, and
// comparison is the whole job here: PRD 010's duplicate detection exists because
// a crew member and their passenger will both register the same car, and two
// spellings of one plate are two rows a coordinator has to reconcile by hand.
// PRD 006 §2 learned this with phone numbers.
package plate

import (
	"errors"
	"regexp"
	"strings"
)

// DefaultCountry prefixes a plate typed without one. Nathejk is a Danish event
// and the overwhelming majority of plates are Danish, so assuming DK is what
// makes the field one-handed to fill in on a phone.
const DefaultCountry = "DK"

// ErrInvalid indicates the input cannot be a licence plate.
var ErrInvalid = errors.New("invalid licence plate")

// minLen/maxLen bound the registration part.
//
// A band rather than a format. Danish plates are two letters and five digits,
// but this must accept a foreign one — Nathejk draws Danish participants and not
// only Danish ones (PRD 010 §5) — and a rejected plate on a car that is
// physically parked in the race area is worse for the inventory than an oddly
// formatted one. So the only thing refused is what cannot be a plate at all.
const (
	minLen = 2
	maxLen = 10
)

var (
	// Separators people type inside a plate: "AB 12 345", "AB-12-345",
	// "AB.12.345".
	separators = regexp.MustCompile(`[\s.\-_]`)

	// An explicit country prefix, which is the only way to register a non-Danish
	// plate.
	prefixed = regexp.MustCompile(`^([A-Z]{2})\+(.*)$`)

	// What a registration may consist of once separators are gone.
	body = regexp.MustCompile(`^[A-Z0-9]+$`)
)

// Normalize converts free-form plate input into "CC+REGISTRATION".
//
//	"ab 12 345"     → "DK+AB12345"
//	"AB-12-345"     → "DK+AB12345"
//	"dk+ab12345"    → "DK+AB12345"
//	"se+abc123"     → "SE+ABC123"
//
// Normalizing an already-normalized plate returns it unchanged, so a value read
// back out of the inventory and re-submitted cannot drift.
//
// A country prefix is recognised **only** when written with the plus, never from
// a bare leading pair of letters. "DK 12345" is a plate, not a Danish "12345":
// two letters followed by digits is precisely the shape of a Danish
// registration, so guessing here would silently turn one member's plate into
// another's — and the point of this package is that two spellings never become
// two rows.
func Normalize(input string) (string, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(input))
	if trimmed == "" {
		return "", ErrInvalid
	}

	country := DefaultCountry
	registration := trimmed
	if m := prefixed.FindStringSubmatch(trimmed); m != nil {
		country, registration = m[1], m[2]
	}

	// After the prefix has been taken off, so a separator cannot hide the plus.
	registration = separators.ReplaceAllString(registration, "")

	if len(registration) < minLen || len(registration) > maxLen {
		return "", ErrInvalid
	}
	if !body.MatchString(registration) {
		return "", ErrInvalid
	}

	return country + "+" + registration, nil
}
