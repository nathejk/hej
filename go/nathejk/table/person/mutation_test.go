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

// A verification is no longer invalidated when the register's number changes (PRD 015, task 225).
//
// This replaces three tests that asserted the opposite — a conditional clear, a normalized
// comparison, and a clear on removal. They are not gone because they were wrong; they were right
// about a different question. The tick used to be standing consent about one specific number, so a
// changed number had to revoke it. It is now a fast track past one question at check-in, and
// check-in is the backstop for everyone it does not cover, so an edit to the register does not
// unmake the member's answer.
//
// What is asserted here is the *absence* of a second statement, because that is the whole change:
// `handleSpejderUpdated` now writes the row and nothing else.
func TestGuardianChangeNoLongerInvalidatesVerification(t *testing.T) {
	stmts := fold(t, spejder("member-1", "30112233", "40556677"))
	if len(stmts) != 1 {
		t.Fatalf("want the upsert alone, got %d: %v", len(stmts), stmts)
	}
	if strings.Contains(stmts[0], "verifiedAt=NULL") {
		t.Errorf("a details update must not clear a verification\ngot: %s", stmts[0])
	}
	// The column the old rule compared against is gone from the projection entirely; a statement
	// still naming it would be writing to something table.sql no longer creates.
	if strings.Contains(stmts[0], "verifiedAgainstPhone") {
		t.Errorf("verifiedAgainstPhone is removed (task 225)\ngot: %s", stmts[0])
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

// IsVerified is now "the member verified, and there is still a number on file" (PRD 015, task
// 225). It no longer expires when the register moves.
//
// The three cases marked below are the ones that flipped, and they are the whole point of the
// change: a member who answered the question has answered it, and check-in — not a re-prompt — is
// what covers everyone else.
func TestIsVerified(t *testing.T) {
	now := time.Now()
	ptr := func(s string) *string { return &s }

	for _, tc := range []struct {
		name string
		p    Person
		want bool
	}{
		{"never verified", Person{PhoneParent: ptr("+4540556677")}, false},
		{"verified and matching", Person{
			VerifiedAt:        &now,
			PhoneParent:       ptr("+4540556677"),
			AcknowledgedPhone: ptr("+4540556677"),
		}, true},
		// FLIPPED (was false). The register moved after the member verified. They still told us a
		// number they could reach, which is what check-in wanted to know — and spejder details are
		// re-published on any edit, so the old rule sent members back through the check because
		// somebody fixed a typo.
		{"verified, then the register changed", Person{
			VerifiedAt:        &now,
			PhoneParent:       ptr("+4511111111"),
			AcknowledgedPhone: ptr("+4540556677"),
		}, true},
		// Still false: with no number on file there is nothing for the tick to point at.
		{"verified but the number was removed", Person{
			VerifiedAt:        &now,
			PhoneParent:       nil,
			AcknowledgedPhone: ptr("+4540556677"),
		}, false},
		// The member could not recognise our number and supplied the right one. Verified since
		// task 148, and now for a simpler reason: the acknowledged number is not compared to
		// anything.
		{"corrected: acknowledged a different number than the register holds", Person{
			VerifiedAt:        &now,
			PhoneParent:       ptr("+4540556677"),
			AcknowledgedPhone: ptr("+4522334455"),
		}, true},
		// FLIPPED (was false): a correction followed by another register edit.
		{"corrected, then the register changed again", Person{
			VerifiedAt:        &now,
			PhoneParent:       ptr("+4599999999"),
			AcknowledgedPhone: ptr("+4522334455"),
		}, true},
		// FLIPPED (was false): verifications recorded before verifiedAgainstPhone existed used to
		// be un-vouched-for. Nothing is compared any more, so there is nothing to be missing.
		{"verified with no record of what the register held", Person{
			VerifiedAt:        &now,
			PhoneParent:       ptr("+4540556677"),
			AcknowledgedPhone: ptr("+4540556677"),
		}, true},
		// A population with no contact number can never be verified in this sense.
		// Callers must not read that as "nag them" — there is nothing to confirm.
		{"population without a contact number", Person{VerifiedAt: &now}, false},
	} {
		if got := tc.p.IsVerified(); got != tc.want {
			t.Errorf("%s: IsVerified() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// An own-phone verification must not satisfy the contact check. This is the trap task 224 was
// written to avoid, asserted from the read side: a member who logs in and skips the check has a
// `phoneVerifiedAt`, and if that ever reached `verifiedAt` they would be silently fast-tracked at
// check-in with no contact number on file.
//
// It holds structurally — the querier does not select the own-phone columns, so IsVerified cannot
// see them — and this pins the intent so a future reader who adds them to Person has a failing
// test to read rather than a comment to overlook.
func TestOwnPhoneVerificationIsNotAContactVerification(t *testing.T) {
	ptr := func(s string) *string { return &s }
	skipped := Person{PhoneParent: ptr("+4540556677")}
	if skipped.IsVerified() {
		t.Error("a member who only verified their own number has not verified a contact number")
	}
}
