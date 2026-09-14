package person

import (
	"strings"
	"testing"
	"time"

	"github.com/nathejk/shared-go/messages"
)

func TestVerifiedSubjectShape(t *testing.T) {
	s, err := VerifiedSubject("2026", "member-1")
	if err != nil {
		t.Fatalf("VerifiedSubject: %v", err)
	}
	if got := s.Subject(); got != "NATHEJK.2026.member.member-1.verified" {
		t.Errorf("subject = %q", got)
	}
	// The publish side and the consume side are two strings in two files, and a subject
	// that does not match is completely silent — the projection simply never writes.
	if !s.Match("nathejk.*.member.*.verified") {
		t.Errorf("published subject %q does not match the consumed pattern", s.Subject())
	}
}

// Same reasoning as the portrait subject: an id that splits into extra tokens still
// matches NATHEJK.> (so the publish succeeds) while no longer matching the per-person
// pattern — which would quietly make this member's verification unerasable, and it carries
// a parent's phone number.
func TestVerifiedSubjectRejectsBadTokens(t *testing.T) {
	for _, tc := range []struct{ year, person string }{
		{"", "member-1"},
		{"2026", ""},
		{"2026", "member.1"},
		{"2026", "member *"},
		{"2026", "member>1"},
		{"20 26", "member-1"},
	} {
		if _, err := VerifiedSubject(tc.year, tc.person); err == nil {
			t.Errorf("VerifiedSubject(%q, %q) = nil error, want a refusal", tc.year, tc.person)
		}
	}
}

func TestMemberVerifiedWritesContactAndTimestamp(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.member.member-1.verified",
		messages.NathejkMemberVerified{
			MemberID:     "member-1",
			PhoneContact: "4512345678",
			VerifiedAt:   time.Date(2026, 8, 30, 19, 5, 0, 0, time.UTC),
		}))

	// An UPDATE, not an upsert: a verification must not invent a person.
	if !strings.HasPrefix(stmt, "UPDATE person SET verifiedAt=") {
		t.Fatalf("statement = %q", stmt)
	}
	// The number and the timestamp together, always. A verifiedAt with no number beside it is a
	// verification of nothing, which reads as verified to a human scanning the table.
	if !strings.Contains(stmt, `acknowledgedPhone="4512345678"`) {
		t.Errorf("contact number missing from %q", stmt)
	}
	if !strings.Contains(stmt, `verifiedAt="2026-08-30 19:05:00"`) {
		t.Errorf("timestamp missing or not UTC-formatted in %q", stmt)
	}
	// The event no longer carries what the register held (task 222), so the column that used to
	// hold it must be NULL rather than a value invented here from the current row.
	if !strings.Contains(stmt, "verifiedAgainstPhone=NULL") {
		t.Errorf("want verifiedAgainstPhone=NULL now the event does not carry it: %q", stmt)
	}
	if !strings.Contains(stmt, `WHERE personId="member-1" AND year="2026"`) {
		t.Errorf("wrong row targeted: %q", stmt)
	}
}

// Until task 224 gives the own-phone verification a column of its own, an event with no contact
// number has nowhere to go, so it is refused rather than stored as a verifiedAt with no subject.
func TestMemberVerifiedRejectsMissingContactPhone(t *testing.T) {
	if _, err := handle(t, "NATHEJK.2026.member.member-1.verified", messages.NathejkMemberVerified{
		MemberID:   "member-1",
		VerifiedAt: time.Now().UTC(),
	}); err == nil {
		t.Fatal("want an error for a verification with no contact phone")
	}
}

// The member id may be taken from the subject when the body omits it, like every other handler
// here — the subject is the authoritative key.
func TestMemberVerifiedFallsBackToSubjectID(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.member.member-9.verified",
		messages.NathejkMemberVerified{
			PhoneContact: "4512345678",
			VerifiedAt:   time.Now().UTC(),
		}))
	if !strings.Contains(stmt, `personId="member-9"`) {
		t.Errorf("subject id not used: %q", stmt)
	}
}

// A zero timestamp is not storable as a MariaDB TIMESTAMP, and a publisher that forgets
// the field must not dead-letter the row: the verification is what matters, the exact
// minute is not.
func TestMemberVerifiedToleratesZeroTimestamp(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.member.member-1.verified",
		messages.NathejkMemberVerified{
			MemberID:     "member-1",
			PhoneContact: "4512345678",
		}))
	if strings.Contains(stmt, "0000-00-00") || strings.Contains(stmt, `verifiedAt=""`) {
		t.Errorf("zero timestamp reached the statement: %q", stmt)
	}
}
