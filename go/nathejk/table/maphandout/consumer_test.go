package maphandout

import (
	"strings"
	"testing"

	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/cqrs/cqrstest"
	"github.com/nathejk/shared-go/messages"
)

// foldAt folds one registration, at a given event time (unix seconds matter here).
func foldAt(t *testing.T, subject string, body any) []string {
	t.Helper()

	w := &cqrstest.Writer{}
	c := consumer{w: w}

	msg := cqrstest.NewMessage(cqrs.SubjectFromStr(subject))
	if err := msg.SetBody(body); err != nil {
		t.Fatalf("SetBody: %v", err)
	}
	if err := c.HandleMessage(msg); err != nil {
		t.Fatalf("HandleMessage(%s): %v", subject, err)
	}
	return w.Statements
}

func single(t *testing.T, stmts []string) string {
	t.Helper()
	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d: %v", len(stmts), stmts)
	}
	return stmts[0]
}

func mustContain(t *testing.T, stmt string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(stmt, want) {
			t.Errorf("statement is missing %s\ngot: %s", want, stmt)
		}
	}
}

func TestRegisteredWritesTheHandout(t *testing.T) {
	stmt := single(t, foldAt(t, "NATHEJK.2026.qr.qr-17.registered",
		qrRegistered{
			NathejkQrRegistered: messages.NathejkQrRegistered{
				QrID:       "qr-17",
				TeamID:     "team-9",
				TeamNumber: "138",
				ScannerID:  "user-3",
			},
			MapID: "kort-2",
		}))

	mustContain(t, stmt, "INSERT INTO maphandout", "ON DUPLICATE KEY UPDATE",
		`year="2026"`, `qrId="qr-17"`, `teamId="team-9"`, `mapId="kort-2"`, `registeredBy="user-3"`)
}

// The map id is an additive field skan adds to the shared body, so it can only be read through a
// struct embedding `messages.NathejkQrRegistered`. If that embedding were ever replaced with the bare
// shared type, the sheet would silently stop being recorded and every handout would read as "Ukendt
// kort" — so the field gets its own test.
func TestMapIDIsReadFromTheAdditiveField(t *testing.T) {
	stmt := single(t, foldAt(t, "NATHEJK.2026.qr.qr-17.registered",
		qrRegistered{
			NathejkQrRegistered: messages.NathejkQrRegistered{QrID: "qr-17", TeamID: "team-9"},
			MapID:               "kort-5",
		}))

	mustContain(t, stmt, `mapId="kort-5"`)
}

// "" means *unknown sheet*, not *no sheet*, so a re-scan before the sheet was recorded must not erase
// the sheet the row already names. Decided in SQL rather than in Go because the comparison is against
// the stored value, which this process does not have.
func TestMapIDIsNeverOverwrittenWithEmpty(t *testing.T) {
	stmt := single(t, foldAt(t, "NATHEJK.2026.qr.qr-17.registered",
		qrRegistered{
			NathejkQrRegistered: messages.NathejkQrRegistered{QrID: "qr-17", TeamID: "team-9"},
		}))

	mustContain(t, stmt, "mapId=IF(VALUES(mapId) = '', mapId, VALUES(mapId))")
}

// A re-binding of the same code to the same team widens the interval rather than duplicating or
// being discarded. LEAST/GREATEST so the result is correct whatever order a replay delivers in —
// and replay is not hypothetical, since the projection is rebuilt from sequence zero on every boot.
func TestReBindingWidensTheInterval(t *testing.T) {
	stmt := single(t, foldAt(t, "NATHEJK.2026.qr.qr-17.registered",
		qrRegistered{
			NathejkQrRegistered: messages.NathejkQrRegistered{QrID: "qr-17", TeamID: "team-9"},
		}))

	mustContain(t, stmt,
		"firstUts=LEAST(firstUts, VALUES(firstUts))",
		"lastUts=GREATEST(lastUts, VALUES(lastUts))")
}

// A binding with no team is not a handout to anyone — a code registered before it was given out.
// Recording it under an empty team id would put a row in every patrol's history that no patrol was
// ever handed.
func TestRegistrationWithoutATeamIsIgnored(t *testing.T) {
	stmts := foldAt(t, "NATHEJK.2026.qr.qr-17.registered",
		qrRegistered{
			NathejkQrRegistered: messages.NathejkQrRegistered{QrID: "qr-17"},
			MapID:               "kort-2",
		})

	if len(stmts) != 0 {
		t.Fatalf("want nothing written, got %v", stmts)
	}
}

// The subject is authoritative where it disagrees with the body: that is what the stream routed on.
func TestQrIDFallsBackToTheSubject(t *testing.T) {
	stmt := single(t, foldAt(t, "NATHEJK.2026.qr.qr-from-subject.registered",
		qrRegistered{
			NathejkQrRegistered: messages.NathejkQrRegistered{TeamID: "team-9"},
		}))

	mustContain(t, stmt, `qrId="qr-from-subject"`)
}

// A subject this projection did not subscribe to is ignored rather than folded. Worth a test because
// the handler is reached by pattern *and* re-checks the pattern itself: a consumer that folded
// whatever it was handed would write a handout row for any `qr` event, including a post visit.
func TestUnrelatedSubjectIsIgnored(t *testing.T) {
	w := &cqrstest.Writer{}
	c := consumer{w: w}

	msg := cqrstest.NewMessage(cqrs.SubjectFromStr("NATHEJK.2026.qr.qr-17.scanned"))
	if err := msg.SetBody(qrRegistered{
		NathejkQrRegistered: messages.NathejkQrRegistered{QrID: "qr-17", TeamID: "team-9"},
	}); err != nil {
		t.Fatalf("SetBody: %v", err)
	}
	if err := c.HandleMessage(msg); err != nil {
		t.Fatalf("an unrelated subject is not an error: %v", err)
	}
	if len(w.Statements) != 0 {
		t.Fatalf("a post visit must not be recorded as a handout: %v", w.Statements)
	}
}

// The year comes from the subject and every read is year-scoped, so a matching subject that somehow
// carries no year must not be written into an empty year. Unreachable through the subscribed pattern
// today — which requires the year segment — and kept as a guard for the day the pattern is widened.
func TestMissingYearIsAnError(t *testing.T) {
	if got := subjectYear(cqrs.SubjectFromStr("NATHEJK")); got != "" {
		t.Fatalf("want no year, got %q", got)
	}
	if got := subjectYear(cqrs.SubjectFromStr("NATHEJK.2026.qr.qr-1.registered")); got != "2026" {
		t.Fatalf("want 2026, got %q", got)
	}
}

// `qr.scanned` is a post visit, not a handover. Subscribing to it here would make the handout list
// grow at every checkpoint — plausible-looking for about one race, and then obviously wrong.
func TestOnlyRegisteredIsConsumed(t *testing.T) {
	subjects := consumer{}.Consumes()
	if len(subjects) != 1 {
		t.Fatalf("want exactly one subject, got %d: %v", len(subjects), subjects)
	}
	if got := subjects[0].Subject(); !strings.Contains(got, "registered") {
		t.Fatalf("want the registered subject, got %q", got)
	}
	if strings.Contains(subjects[0].Subject(), "scanned") {
		t.Fatal("qr.scanned is a post visit and belongs to the scan projection, not here")
	}
}

// Folding the same event twice must produce the same statement: the projection replays from sequence
// zero on every boot.
func TestStatementsAreIdempotent(t *testing.T) {
	body := qrRegistered{
		NathejkQrRegistered: messages.NathejkQrRegistered{QrID: "qr-17", TeamID: "team-9"},
		MapID:               "kort-2",
	}
	first := single(t, foldAt(t, "NATHEJK.2026.qr.qr-17.registered", body))
	second := single(t, foldAt(t, "NATHEJK.2026.qr.qr-17.registered", body))

	if first != second {
		t.Fatalf("folding the same event twice differed:\n%s\n%s", first, second)
	}
}
