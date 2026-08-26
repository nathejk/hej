package person

import (
	"strings"
	"testing"
	"time"

	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/cqrs/cqrstest"
	"github.com/nathejk/shared-go/messages"
	"github.com/nathejk/shared-go/types"
)

// This file covers the two mutations of PRD 006 §5 that both carry security weight: a
// member losing their access, and a phone number changing hands.

func spejder(id, phone, guardian string) event {
	return event{"NATHEJK.2026.spejder." + id + ".updated",
		messages.NathejkScoutUpdated{
			MemberID:     types.MemberID(id),
			Name:         "Freja Hansen",
			Phone:        types.PhoneNumber(phone),
			PhoneContact: types.PhoneNumber(guardian),
		}}
}

// A deleted member must lose their login. The filter lives in the querier so no call
// site can forget it; this pins that the projector actually records the deletion.
func TestDeleteMarksThePersonDeleted(t *testing.T) {
	for _, e := range []event{
		{"NATHEJK.2026.spejder.member-1.deleted", messages.NathejkMemberAdded{MemberID: "member-1"}},
		{"NATHEJK.2026.senior.member-1.deleted", messages.NathejkMemberAdded{MemberID: "member-1"}},
	} {
		stmt := onlyStatement(t, fold(t, e))
		if !strings.Contains(stmt, "deleted=1") {
			t.Errorf("%s: want deleted=1\ngot: %s", e.subject, stmt)
		}
		if !strings.Contains(stmt, `personId="member-1"`) || !strings.Contains(stmt, `year="2026"`) {
			t.Errorf("%s: a delete must be scoped to one person-year, or it takes\n"+
				"other people's logins with it\ngot: %s", e.subject, stmt)
		}
	}
}

// A delete for someone never seen must not create a tombstone row: it is a no-op, and
// an UPDATE affecting zero rows is exactly right.
func TestDeleteDoesNotInsert(t *testing.T) {
	stmt := onlyStatement(t, fold(t,
		event{"NATHEJK.2026.spejder.ghost.deleted", messages.NathejkMemberAdded{MemberID: "ghost"}}))
	if strings.Contains(stmt, "INSERT") {
		t.Errorf("a delete must not invent a person\ngot: %s", stmt)
	}
}

// Deletion is not sticky, and that is the intended behaviour: upstream re-adding a
// member is a real thing, and they should get their login back. Stream order is the
// truth and the last event about a person wins.
func TestUpdateAfterDeleteRestoresTheLogin(t *testing.T) {
	stmts := fold(t,
		spejder("member-1", "30112233", ""),
		event{"NATHEJK.2026.spejder.member-1.deleted", messages.NathejkMemberAdded{MemberID: "member-1"}},
		spejder("member-1", "30112233", ""))

	last := upsertStatement(t, stmts[2:])
	if !strings.Contains(last, "deleted=VALUES(deleted)") {
		t.Errorf("a re-add must clear the soft delete\ngot: %s", last)
	}
	if !strings.Contains(last, ", 0,") && !strings.Contains(last, "(0,") {
		t.Errorf("want deleted written as 0\ngot: %s", last)
	}
}

// The other half of "a deleted member loses their login": order matters, and a delete
// arriving last must win. This is the ordering that a hard DELETE plus INSERT would get
// wrong.
func TestDeleteAfterUpdateWins(t *testing.T) {
	stmts := fold(t,
		spejder("member-1", "30112233", ""),
		event{"NATHEJK.2026.spejder.member-1.deleted", messages.NathejkMemberAdded{MemberID: "member-1"}})

	if !strings.Contains(stmts[len(stmts)-1], "deleted=1") {
		t.Errorf("the delete must be the last word\ngot: %v", stmts)
	}
}

// A changed number must stop resolving at the old value. The projector overwrites the
// column rather than accumulating numbers, which is what makes the old one stop
// matching — worth pinning, because "also keep the previous number" is a plausible
// change that would silently let two numbers log in as one person.
func TestPhoneChangeOverwritesTheOldNumber(t *testing.T) {
	stmt := upsertStatement(t, fold(t, spejder("member-1", "40556677", "")))

	if !strings.Contains(stmt, `"+4540556677"`) {
		t.Errorf("want the new number\ngot: %s", stmt)
	}
	if !strings.Contains(stmt, "phone=VALUES(phone)") {
		t.Errorf("the phone must be overwritten on update, or a reassigned number\n"+
			"logs in as its previous owner\ngot: %s", stmt)
	}
}

// A number being removed upstream must remove the login too, not leave the previous one
// working.
func TestPhoneRemovalClearsTheNumber(t *testing.T) {
	stmt := upsertStatement(t, fold(t, spejder("member-1", "", "")))
	if !strings.Contains(stmt, `phone=VALUES(phone)`) || !strings.Contains(stmt, `""`) {
		t.Errorf("an emptied phone must be written through\ngot: %s", stmt)
	}
}

// The verification invalidation. It must be conditional: spejder details are
// re-published on any edit and re-delivered on every replay, so an unconditional clear
// would destroy a valid verification the first time someone fixed an address.
func TestGuardianChangeInvalidatesVerificationConditionally(t *testing.T) {
	stmts := fold(t, spejder("member-1", "30112233", "40556677"))
	if len(stmts) != 2 {
		t.Fatalf("want an upsert plus an invalidation, got %d: %v", len(stmts), stmts)
	}
	inv := stmts[1]

	for _, want := range []string{
		"SET verifiedAt=NULL",
		// Only for this person, only this year.
		`personId="member-1"`,
		`year="2026"`,
		// A no-op unless there is something to invalidate.
		"verifiedAt IS NOT NULL",
		// Null-safe inequality: a plain <> against NULL yields NULL and would skip
		// the case where the guardian number was removed entirely.
		`NOT (acknowledgedPhone <=> "+4540556677")`,
	} {
		if !strings.Contains(inv, want) {
			t.Errorf("invalidation is missing %s\ngot: %s", want, inv)
		}
	}
}

// The number is compared in its normalized form, or every re-publication with different
// formatting would look like a change and revoke a good verification.
func TestInvalidationComparesNormalizedNumbers(t *testing.T) {
	stmts := fold(t, spejder("member-1", "30112233", "40 55 66 77"))
	if !strings.Contains(stmts[1], `"+4540556677"`) {
		t.Errorf("want the normalized guardian number in the comparison\ngot: %s", stmts[1])
	}
}

// A guardian number being removed must invalidate as surely as one being changed: there
// is now no number for the acknowledged consent to be about.
func TestGuardianRemovalInvalidates(t *testing.T) {
	stmts := fold(t, spejder("member-1", "30112233", ""))
	if !strings.Contains(stmts[1], "NOT (acknowledgedPhone <=> NULL)") {
		t.Errorf("want a null-safe comparison against NULL\ngot: %s", stmts[1])
	}
}

// Populations with no guardian number must not emit an invalidation at all — there is
// nothing to invalidate, and a statement that clears verifiedAt for a bandit would be a
// bug waiting for verification to ship.
func TestNonSpejderPopulationsDoNotInvalidate(t *testing.T) {
	for _, e := range []event{
		{"NATHEJK.2026.senior.member-9.updated",
			messages.NathejkSeniorUpdated{MemberID: "member-9", Name: "Bandit"}},
		crewRegistered,
		goeglerUpdated,
	} {
		for _, stmt := range fold(t, e) {
			if strings.Contains(stmt, "verifiedAt") {
				t.Errorf("%s must not touch verifiedAt\ngot: %s", e.subject, stmt)
			}
		}
	}
}

// Both statements are re-run on every boot.
func TestDeletionAndInvalidationAreIdempotent(t *testing.T) {
	stmts := fold(t,
		spejder("member-1", "30112233", "40556677"),
		event{"NATHEJK.2026.spejder.member-1.deleted", messages.NathejkMemberAdded{MemberID: "member-1"}})

	for _, stmt := range stmts {
		switch {
		case strings.HasPrefix(stmt, "UPDATE "):
			if !strings.Contains(stmt, " WHERE ") {
				t.Errorf("unscoped UPDATE: %s", stmt)
			}
		case strings.HasPrefix(stmt, "INSERT "):
			if !strings.Contains(stmt, "ON DUPLICATE KEY UPDATE") {
				t.Errorf("INSERT would fail on the second replay: %s", stmt)
			}
		default:
			t.Errorf("unexpected statement shape: %s", stmt)
		}
	}
}

// A number that arrived and could not be used must be reported, not silently dropped.
// For the guardian field, silence means staff being told "no number on file" for a
// member whose parents did supply one.
func TestUnusablePhoneIsReported(t *testing.T) {
	type report struct {
		personID, field string
		digits          int
	}

	for _, tc := range []struct {
		name, phone, guardian string
		want                  []report
	}{
		{"seven-digit guardian typo", "30112233", "3068640",
			[]report{{"member-1", "phoneParent", 7}}},
		{"free text naming two numbers", "30112233", "Mor: 24281097 eller Far: 22239313",
			[]report{{"member-1", "phoneParent", 16}}},
		{"unusable own number", "533899557", "40556677",
			[]report{{"member-1", "phone", 9}}},
		// Absent is not broken: nothing to report.
		{"both absent", "", "", nil},
		// Both usable.
		{"both fine", "30112233", "40556677", nil},
		// Now accepted by the normalizer, so no longer a drop (see internal/phone).
		{"bare 45 country code", "30112233", "4530756173", nil},
	} {
		var got []report
		w := &cqrstest.Writer{}
		c := consumer{w: w, normalizer: testNormalizer{},
			unusablePhone: func(personID, field string, digits int) {
				got = append(got, report{personID, field, digits})
			}}

		msg := cqrstest.NewMessage(cqrs.SubjectFromStr("NATHEJK.2026.spejder.member-1.updated"))
		if err := msg.SetBody(messages.NathejkScoutUpdated{
			MemberID:     "member-1",
			Phone:        types.PhoneNumber(tc.phone),
			PhoneContact: types.PhoneNumber(tc.guardian),
		}); err != nil {
			t.Fatalf("SetBody: %v", err)
		}
		if err := c.HandleMessage(msg); err != nil {
			t.Fatalf("HandleMessage: %v", err)
		}

		if len(got) != len(tc.want) {
			t.Errorf("%s: got %d reports %v, want %d %v", tc.name, len(got), got, len(tc.want), tc.want)
			continue
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Errorf("%s: report %d = %+v, want %+v", tc.name, i, got[i], tc.want[i])
			}
		}
	}
}

// The guardian number is normalized once per event, so an unusable one is reported once
// — not once for the row and again for the verification check.
func TestUnusableGuardianPhoneIsReportedOnce(t *testing.T) {
	var calls int
	w := &cqrstest.Writer{}
	c := consumer{w: w, normalizer: testNormalizer{},
		unusablePhone: func(string, string, int) { calls++ }}

	msg := cqrstest.NewMessage(cqrs.SubjectFromStr("NATHEJK.2026.spejder.member-1.updated"))
	if err := msg.SetBody(messages.NathejkScoutUpdated{
		MemberID: "member-1", Phone: "30112233", PhoneContact: "3068640"}); err != nil {
		t.Fatalf("SetBody: %v", err)
	}
	if err := c.HandleMessage(msg); err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}

	if calls != 1 {
		t.Errorf("want 1 report, got %d", calls)
	}
}

// A nil sink must be usable: nothing here may require the application's logger.
func TestUnusablePhoneWithNoSinkDoesNotPanic(t *testing.T) {
	if stmts := fold(t, spejder("member-1", "3068640", "3068640")); len(stmts) == 0 {
		t.Fatal("want statements")
	}
}

// IsVerified is the read-side half of the same rule, and must not simply mirror
// verifiedAt.
func TestIsVerifiedRequiresTheNumberToStillMatch(t *testing.T) {
	now := time.Now()
	ptr := func(s string) *string { return &s }

	for _, tc := range []struct {
		name string
		p    Person
		want bool
	}{
		{"never verified", Person{PhoneParent: ptr("+4540556677")}, false},
		{"verified and matching",
			Person{VerifiedAt: &now, PhoneParent: ptr("+4540556677"), AcknowledgedPhone: ptr("+4540556677")}, true},
		{"verified but the number changed",
			Person{VerifiedAt: &now, PhoneParent: ptr("+4511111111"), AcknowledgedPhone: ptr("+4540556677")}, false},
		{"verified but the number was removed",
			Person{VerifiedAt: &now, PhoneParent: nil, AcknowledgedPhone: ptr("+4540556677")}, false},
		{"verified with no record of what was acknowledged",
			Person{VerifiedAt: &now, PhoneParent: ptr("+4540556677")}, false},
		// A population with no guardian number can never be verified in this sense.
		// Callers must not read that as "nag them" — there is nothing to confirm.
		{"population without a guardian number", Person{VerifiedAt: &now}, false},
	} {
		if got := tc.p.IsVerified(); got != tc.want {
			t.Errorf("%s: IsVerified() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// The two halves must agree, or one of them is lying to the UI.
func TestIsVerifiedAgreesWithTheProjectorsCondition(t *testing.T) {
	// The projector clears verifiedAt exactly when NOT (acknowledgedPhone <=> parent).
	// IsVerified must return false in precisely those cases, which the table above
	// covers; this test pins the shared intent so a change to one prompts a change to
	// the other.
	now := time.Now()
	ptr := func(s string) *string { return &s }
	matching := Person{VerifiedAt: &now, PhoneParent: ptr("+45x"), AcknowledgedPhone: ptr("+45x")}
	changed := Person{VerifiedAt: &now, PhoneParent: ptr("+45y"), AcknowledgedPhone: ptr("+45x")}

	if !matching.IsVerified() || changed.IsVerified() {
		t.Error("IsVerified must accept a matching pair and reject a changed one")
	}
}
