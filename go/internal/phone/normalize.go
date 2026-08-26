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
//   - Bare `45` + 8 digits — treated as a Danish number whose country code was
//     typed without the `+`. Unambiguous, because Danish subscriber numbers are
//     exactly 8 digits, so a 10-digit local number starting `45` cannot be one.
//     This case is not hypothetical: guardian numbers in the real event data are
//     entered this way, and rejecting them meant an emergency contact silently
//     did not exist (task 076).
//
// Any other input returns ErrInvalid. In particular a 7- or 9-digit number is
// rejected rather than guessed at: those are typos, and inventing a digit for a
// number the app may have to ring in an emergency would be worse than admitting
// it is unusable.
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
	case len(digits) == 10 && strings.HasPrefix(digits, "45"):
		return "+" + digits, nil
	default:
		return "", ErrInvalid
	}
}
