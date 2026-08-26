package person

// PhoneNormalizer folds free-form phone input into the single canonical form used
// for lookups.
//
// This is declared here, as an interface the application satisfies, rather than
// imported. Two reasons, and the second is the one that matters:
//
//  1. This package is bound for shared-go and cannot import
//     nathejk.dk/internal/... (Go forbids importing another module's internal tree).
//
//  2. Normalization must be *identical* on both sides of the directory. The
//     projector writes a phone number into the row; the login handler looks one up.
//     If those two use different implementations — or the same rules re-typed twice —
//     the lookup silently misses and the user is simply told their number is not
//     recognised, with nothing in the logs to suggest why. A single injected
//     implementation makes that class of bug unrepresentable rather than merely
//     unlikely.
//
// PRD 006 §2 lists exactly this as a correctness risk. `internal/phone.Normalize`
// is the implementation; `cmd/api` adapts it to this interface.
type PhoneNormalizer interface {
	// Normalize returns the canonical form, or an error if the input cannot be
	// interpreted as a phone number.
	Normalize(input string) (string, error)
}

// normalizeOrEmpty applies the normalizer, returning "" when the input cannot be
// normalized.
//
// Empty is the right outcome for the projection rather than an error: upstream data
// contains blanks and typos, and a single unparseable number must not stop the
// projector and stall every other person behind it. The row is still written, just
// without a usable login key — which is visible (that person cannot log in) and
// fixable upstream, whereas a dead-lettered event is neither.
//
// The querier refuses to look up an empty phone (see Lookup), so a blank cannot
// accidentally match every row that also has a blank.
//
// "Visible" was doing a lot of unearned work in that reasoning, though: nothing actually
// said so. See consumer.normalizePhone, which reports the drop.
func normalizeOrEmpty(n PhoneNormalizer, raw string) string {
	if n == nil || raw == "" {
		return ""
	}
	normalized, err := n.Normalize(raw)
	if err != nil {
		return ""
	}
	return normalized
}

// countDigits returns how many digits a string contains.
//
// Used to describe an unusable phone number without repeating it: see
// consumer.normalizePhone for why the raw value is deliberately not passed on.
func countDigits(s string) int {
	n := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			n++
		}
	}
	return n
}
