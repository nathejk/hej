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
