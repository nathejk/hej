package main

import (
	"testing"
	"time"

	"nathejk.dk/internal/data"
	"nathejk.dk/internal/scans"
	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/person"
)

// The derivation in one place, tested through the same helper the profile handler calls.
// It is a two-input predicate over data the client only partially has, which is exactly why
// PRD 005 §8 forbids the client reimplementing it — and why it is worth pinning here.
func TestConfirmationRequiredDerivation(t *testing.T) {
	guardian := "4512345678"
	verified := time.Date(2026, 8, 30, 19, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		p    person.Person
		want bool
	}{
		{
			name: "never verified and not started: ask",
			p: person.Person{
				PersonID:    "member-1",
				PhoneParent: &guardian,
			},
			want: true,
		},
		{
			name: "already verified for the number on file: do not ask",
			p: person.Person{
				PersonID:             "member-1",
				PhoneParent:          &guardian,
				AcknowledgedPhone:    &guardian,
				VerifiedAgainstPhone: &guardian,
				VerifiedAt:           &verified,
			},
			want: false,
		},
		{
			// The correction case (task 148): the member supplied a different number and
			// acknowledged that one. The register has not moved since, so this is settled — and
			// re-asking them would be the bug the old single-comparison rule produced.
			name: "verified against a number they corrected: do not ask",
			p: func() person.Person {
				corrected := "4522334455"
				return person.Person{
					PersonID:             "member-1",
					PhoneParent:          &guardian,
					AcknowledgedPhone:    &corrected,
					VerifiedAgainstPhone: &guardian,
					VerifiedAt:           &verified,
				}
			}(),
			want: false,
		},
		{
			// Starting implies the data was checked at the counter (PRD 005 §11), and
			// re-asking a member who is already on the trail is worse than useless.
			name: "not verified but racing: do not ask",
			p: person.Person{
				PersonID:     "member-1",
				PhoneParent:  &guardian,
				MemberStatus: person.MemberStatusRacing,
			},
			want: false,
		},
		{
			// No guardian number exists for this population. Not "verified" — there is
			// nothing to confirm, and rendering an empty field as missing data would
			// generate support calls and teach organizers to ignore the real flag.
			name: "no guardian number at all: do not ask",
			p: person.Person{
				PersonID: "member-2",
			},
			want: false,
		},
		{
			// A spejder with an empty guardian number IS asked. That record is precisely
			// the one an organizer wants to hear about, and task 128's "jeg kender ikke
			// nummeret" path is how it gets reported. Note "" vs nil is the whole reason
			// the field is a pointer.
			name: "guardian number expected but missing: ask",
			p: func() person.Person {
				empty := ""
				return person.Person{PersonID: "member-3", PhoneParent: &empty}
			}(),
			want: true,
		},
		{
			// Verified, but the guardian number has since changed. IsVerified() refuses
			// this, so the member is asked again — the earlier acknowledgement was about a
			// number nobody would be phoning now.
			name: "verified for a number that has since changed: ask again",
			p: func() person.Person {
				old := "4599999999"
				return person.Person{
					PersonID:             "member-4",
					PhoneParent:          &guardian,
					AcknowledgedPhone:    &old,
					VerifiedAgainstPhone: &old,
					VerifiedAt:           &verified,
				}
			}(),
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestApp(t)
			app.models = data.NewModels(
				users.NewMockDirectory(),
				scans.NewMockSource(),
				nil,
				&stubPeople{p: tc.p, found: true},
			)
			if got := app.confirmationRequired(tc.p.PersonID); got != tc.want {
				t.Errorf("confirmationRequired = %v, want %v", got, tc.want)
			}
		})
	}
}

// A database outage must not turn into a confirmation step whose endpoint cannot record
// anything. Same degradation rule as hasPortrait: the profile is still worth showing.
func TestConfirmationRequiredWithoutProjection(t *testing.T) {
	app := newTestApp(t)
	if app.confirmationRequired("member-1") {
		t.Error("want false with no person projection")
	}
	if app.verifiedAt("member-1") != nil {
		t.Error("want nil verifiedAt with no person projection")
	}
}

// verifiedAt reads through IsVerified rather than the raw column, so a superseded
// acknowledgement reports as "never verified" instead of showing a date that implies the
// current number was confirmed.
func TestVerifiedAtIgnoresSupersededAcknowledgement(t *testing.T) {
	guardian := "4512345678"
	old := "4599999999"
	at := time.Date(2026, 8, 30, 19, 0, 0, 0, time.UTC)

	app := newTestApp(t)
	app.models = data.NewModels(users.NewMockDirectory(), scans.NewMockSource(), nil, &stubPeople{
		found: true,
		p: person.Person{
			PersonID:             "member-1",
			PhoneParent:          &guardian,
			AcknowledgedPhone:    &old,
			VerifiedAgainstPhone: &old,
			VerifiedAt:           &at,
		},
	})
	if got := app.verifiedAt("member-1"); got != nil {
		t.Errorf("verifiedAt = %v, want nil for a stale acknowledgement", got)
	}
}
