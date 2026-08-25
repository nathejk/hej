package main

import (
	"nathejk.dk/internal/phone"
	"nathejk.dk/nathejk/table/person"
)

// phoneNormalizer adapts internal/phone to the person.PhoneNormalizer port.
//
// The adapter exists because the projection package cannot import
// nathejk.dk/internal/... — it is bound for shared-go, and Go forbids importing
// another module's internal tree. Declaring the port there and satisfying it here is
// the pattern go-bff-layout prescribes for exactly this situation.
//
// What it buys, beyond module hygiene: there is now provably one normalization
// implementation behind both the projector that writes a phone number and the
// handler that looks one up. The alternative — the same rules written twice — fails
// in the worst possible way, because a mismatch does not error. The lookup simply
// finds nothing and the user is told their number is not recognised, with nothing in
// the logs to explain why.
type phoneNormalizer struct{}

func (phoneNormalizer) Normalize(input string) (string, error) {
	return phone.Normalize(input)
}

var _ person.PhoneNormalizer = phoneNormalizer{}
