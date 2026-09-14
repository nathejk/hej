package plate

import (
	"errors"
	"testing"
)

// The rule these tests exist to protect is not "the plate looks right" but "two
// people typing the same car produce the same string". Every equivalence case
// below is a duplicate registration that PRD 010's 409 would otherwise miss.
func TestNormalizeEquivalentSpellings(t *testing.T) {
	const want = "DK+AB12345"
	for _, in := range []string{
		"AB12345",
		"ab12345",
		"ab 12 345",
		"AB-12-345",
		"AB.12.345",
		"  ab12345  ",
		"dk+ab12345",
		"DK+AB 12 345",
	} {
		got, err := Normalize(in)
		if err != nil {
			t.Errorf("Normalize(%q) errored: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

// A value read back out of the inventory and submitted again must not drift.
func TestNormalizeIsIdempotent(t *testing.T) {
	for _, in := range []string{"ab12345", "se+abc123", "DK+AB12345"} {
		once, err := Normalize(in)
		if err != nil {
			t.Fatalf("Normalize(%q): %v", in, err)
		}
		twice, err := Normalize(once)
		if err != nil {
			t.Fatalf("Normalize(%q) (second pass): %v", once, err)
		}
		if once != twice {
			t.Errorf("not idempotent: %q → %q → %q", in, once, twice)
		}
	}
}

// A foreign plate must survive. Nathejk is Danish but not exclusively, and a car
// parked in the race area is in the inventory whatever country it is from.
func TestNormalizeKeepsExplicitCountry(t *testing.T) {
	cases := map[string]string{
		"se+abc123":  "SE+ABC123",
		"DE+M AB123": "DE+MAB123",
		"no+ep12345": "NO+EP12345",
	}
	for in, want := range cases {
		got, err := Normalize(in)
		if err != nil {
			t.Errorf("Normalize(%q) errored: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

// No national format is enforced: a plausible plate in an unfamiliar shape is
// accepted rather than refused. Refusing it would leave a car that is physically
// on site out of the inventory, which is the failure this feature exists to fix.
func TestNormalizeDoesNotEnforceDanishFormat(t *testing.T) {
	for _, in := range []string{
		"12345",     // digits only
		"ABCDE",     // letters only
		"1AB234",    // digits first
		"ab123456",  // eight characters
		"se+ab1234", // foreign, prefixed
	} {
		if _, err := Normalize(in); err != nil {
			t.Errorf("Normalize(%q) should be accepted, got %v", in, err)
		}
	}
}

func TestNormalizeRejectsWhatCannotBeAPlate(t *testing.T) {
	for _, in := range []string{
		"",             // empty
		"   ",          // whitespace only
		"-",            // separators only
		"A",            // too short
		"ABCDEFGHIJKL", // too long
		"AB12345!",     // punctuation no plate carries
		"ÆØ12345",      // non-ASCII
		"AB 123 45 67 890",
	} {
		if _, err := Normalize(in); !errors.Is(err, ErrInvalid) {
			t.Errorf("Normalize(%q) should be ErrInvalid, got %v", in, err)
		}
	}
}

// The country code is not checked against a list of real ISO codes, and that is a
// decision rather than an omission: the list would have to be maintained, and the
// cost of the two failure modes is wildly asymmetric. An unknown code stored
// against a car that is genuinely on site costs a coordinator nothing — the plate
// is still the identity. Refusing one leaves the car out of the inventory, which
// is the problem PRD 010 exists to solve.
func TestCountryCodeIsNotValidatedAgainstARealList(t *testing.T) {
	got, err := Normalize("zz+ab12345")
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if got != "ZZ+AB12345" {
		t.Errorf("Normalize = %q, want %q", got, "ZZ+AB12345")
	}
}

// A bare leading letter pair is part of the registration, never a country code.
//
// This is the one ambiguity in the format, and getting it wrong is not cosmetic:
// two letters then digits *is* the Danish plate shape, so treating "DK 12345" as
// country DK plus "12345" would file a real plate under a different string —
// which is exactly the two-spellings-one-car problem this package prevents.
func TestBareLetterPairIsNotACountryCode(t *testing.T) {
	got, err := Normalize("DK 12345")
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if got != "DK+DK12345" {
		t.Errorf("Normalize(%q) = %q, want %q", "DK 12345", got, "DK+DK12345")
	}
}
