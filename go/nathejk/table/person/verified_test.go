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
	if got := s.Subject(); got != "NATHEJK.2026.spejder.member-1.verified" {
		t.Errorf("subject = %q", got)
	}
	// The publish side and the consume side are two strings in two files, and a subject
	// that does not match is completely silent — the projection simply never writes.
	if !s.Match("nathejk.*.spejder.*.verified") {
		t.Errorf("published subject %q does not match the consumed pattern", s.Subject())
	}
	// Four parts, so it must not be caught by the five-part lifecycle patterns that share the
	// prefix and resolve to a member status. A verification is not a status.
	if s.Match("nathejk.*.spejder.*.status.overridden") {
		t.Errorf("subject %q collides with the lifecycle patterns", s.Subject())
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
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.spejder.member-1.verified",
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

// An event with neither number is a tick against nothing: nothing later could say which number
// it vouched for, so it could never be superseded. Refused rather than stored as a timestamp
// with no subject.
func TestMemberVerifiedRejectsEventWithNoNumbers(t *testing.T) {
	if _, err := handle(t, "NATHEJK.2026.spejder.member-1.verified", messages.NathejkMemberVerified{
		MemberID:   "member-1",
		VerifiedAt: time.Now().UTC(),
	}); err == nil {
		t.Fatal("want an error for a verification naming no number at all")
	}
}

// The login shape: the member's own number, proven by the SMS PIN, and no contact number.
//
// The assertions that matter here are the *absences*. A login must not touch the contact
// columns, because a member who confirmed their contact number last week and logs in today
// would otherwise have that confirmation silently wiped — and then be asked again at check-in
// with nobody able to say why.
func TestMemberVerifiedOwnPhoneOnlyLeavesContactAlone(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.spejder.member-1.verified",
		messages.NathejkMemberVerified{
			MemberID:   "member-1",
			Phone:      "4530000001",
			VerifiedAt: time.Date(2026, 8, 30, 19, 5, 0, 0, time.UTC),
		}))

	if !strings.Contains(stmt, `verifiedPhone="4530000001"`) {
		t.Errorf("own number missing from %q", stmt)
	}
	if !strings.Contains(stmt, `phoneVerifiedAt="2026-08-30 19:05:00"`) {
		t.Errorf("own-phone timestamp missing or not UTC-formatted in %q", stmt)
	}
	if strings.Contains(stmt, "acknowledgedPhone") || strings.Contains(stmt, "verifiedAt=") {
		t.Errorf("a login must not write the contact columns: %q", stmt)
	}
	// Specifically not "verifiedAt=NULL" either: clearing is as wrong as overwriting.
	if strings.Contains(stmt, "verifiedAgainstPhone") {
		t.Errorf("a login must not touch verifiedAgainstPhone: %q", stmt)
	}
}

// The skip shape is the login shape (PRD 015 §6): own number verified, contact number not. With
// `omitempty` the two are byte-identical on the wire, which is accepted — both answer "is there a
// verified contact number yet?" with "not yet". Asserted so that reading is deliberate.
func TestMemberVerifiedSkipIsIndistinguishableFromLogin(t *testing.T) {
	at := time.Date(2026, 8, 30, 19, 5, 0, 0, time.UTC)
	skip := onlyStatement(t, mustHandle(t, "NATHEJK.2026.spejder.member-1.verified",
		messages.NathejkMemberVerified{
			MemberID: "member-1", Phone: "4530000001", PhoneContact: "", VerifiedAt: at,
		}))
	login := onlyStatement(t, mustHandle(t, "NATHEJK.2026.spejder.member-1.verified",
		messages.NathejkMemberVerified{
			MemberID: "member-1", Phone: "4530000001", VerifiedAt: at,
		}))
	if skip != login {
		t.Errorf("skip and login must project identically:\n skip  = %q\n login = %q", skip, login)
	}
}

// Both numbers in one event: the member confirmed their contact number in the same session they
// logged in, and the publisher had both facts to hand.
func TestMemberVerifiedWritesBothPairsWhenBothPresent(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.spejder.member-1.verified",
		messages.NathejkMemberVerified{
			MemberID:     "member-1",
			Phone:        "4530000001",
			PhoneContact: "4512345678",
			VerifiedAt:   time.Date(2026, 8, 30, 19, 5, 0, 0, time.UTC),
		}))

	for _, want := range []string{
		`acknowledgedPhone="4512345678"`,
		`verifiedAt="2026-08-30 19:05:00"`,
		`verifiedPhone="4530000001"`,
		`phoneVerifiedAt="2026-08-30 19:05:00"`,
	} {
		if !strings.Contains(stmt, want) {
			t.Errorf("%s missing from %q", want, stmt)
		}
	}
}

// The member id may be taken from the subject when the body omits it, like every other handler
// here — the subject is the authoritative key.
func TestMemberVerifiedFallsBackToSubjectID(t *testing.T) {
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.spejder.member-9.verified",
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
	stmt := onlyStatement(t, mustHandle(t, "NATHEJK.2026.spejder.member-1.verified",
		messages.NathejkMemberVerified{
			MemberID:     "member-1",
			PhoneContact: "4512345678",
		}))
	if strings.Contains(stmt, "0000-00-00") || strings.Contains(stmt, `verifiedAt=""`) {
		t.Errorf("zero timestamp reached the statement: %q", stmt)
	}
}
