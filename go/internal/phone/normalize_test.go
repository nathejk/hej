package phone_test

import (
	"errors"
	"testing"

	"nathejk.dk/internal/phone"
)

func TestNormalize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"e164 with plus", "+4520304050", "+4520304050"},
		{"bare local 8 digits", "20304050", "+4520304050"},
		{"spaces", "20 30 40 50", "+4520304050"},
		{"plus with spaces", "+45 20 30 40 50", "+4520304050"},
		{"parens and dashes", "(20) 30-40-50", "+4520304050"},
		{"double zero prefix", "004520304050", "+4520304050"},
		// Observed in the real event data as a guardian number: the Danish country
		// code typed without a "+". Unambiguous, because a Danish subscriber number is
		// exactly 8 digits (task 076).
		{"bare 45 country code", "4530756173", "+4530756173"},
		{"bare 45 country code with spaces", "45 30 75 61 73", "+4530756173"},
		{"US number", "+1 555 123 4567", "+15551234567"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := phone.Normalize(tc.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNormalize_Invalid(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"whitespace only", "   "},
		{"letters only", "abc"},
		{"too few digits", "12345"},
		// All observed in the real event data. Each is rejected on purpose rather than
		// repaired by guessing: these are numbers the app may have to ring in an
		// emergency, and a wrong number that looks valid is worse than a missing one.
		{"seven digits, one short", "3068640"},
		{"nine digits, one long", "533899557"},
		{"ten digits, not a country code", "6542165156"},
		// Two numbers in one free-text field ("Mor: ... eller Far: ..."). Picking one
		// silently would be a guess about who to call.
		{"two numbers in free text", "Mor: 24281097 eller Far: 22239313"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := phone.Normalize(tc.in)
			if !errors.Is(err, phone.ErrInvalid) {
				t.Fatalf("got err %v, want ErrInvalid", err)
			}
		})
	}
}
