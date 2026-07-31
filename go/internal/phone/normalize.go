// Package phone normalizes free-form phone-number input into a single canonical
// form used everywhere the app compares numbers (auth lookup, subscription
// storage, …). Producing the same output for equivalent inputs is what makes
// the recognition step correct.
package phone

import (
	"errors"
	"regexp"
	"strings"
)

// DefaultCountryCode is prepended to bare local numbers with no explicit
// country code. Danish numbers are 8 digits and the +45 prefix is assumed when
// no other prefix is present.
const DefaultCountryCode = "+45"

// ErrInvalid indicates the input cannot be normalized to a phone number.
var ErrInvalid = errors.New("invalid phone number")

var nonDigit = regexp.MustCompile(`\D`)

// Normalize converts free-form phone input into a canonical `+<country><digits>`
// form. Accepted inputs:
//
//   - `+<code><digits>` — kept as-is after stripping spaces, dashes, and
//     parentheses (leading `+` preserved).
//   - `00<code><digits>` — converted to `+<code><digits>`.
//   - Bare 8-digit local number — prefixed with DefaultCountryCode (+45).
//
// Any other input returns ErrInvalid.
func Normalize(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", ErrInvalid
	}

	// Note whether the caller supplied an explicit "+" before stripping
	// non-digit characters below.
	hasPlus := strings.HasPrefix(trimmed, "+")
	digits := nonDigit.ReplaceAllString(trimmed, "")
	if digits == "" {
		return "", ErrInvalid
	}

	switch {
	case hasPlus:
		return "+" + digits, nil
	case strings.HasPrefix(digits, "00"):
		return "+" + strings.TrimPrefix(digits, "00"), nil
	case len(digits) == 8:
		return DefaultCountryCode + digits, nil
	default:
		return "", ErrInvalid
	}
}
