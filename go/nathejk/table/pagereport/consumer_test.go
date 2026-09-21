package pagereport

import (
	"strings"
	"testing"
	"time"

	"github.com/jrgensen/cqrs"
	"github.com/jrgensen/cqrs/cqrstest"
)

func fold(t *testing.T, subject string, body any) []string {
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

func foldErr(t *testing.T, subject string, body any) error {
	t.Helper()

	w := &cqrstest.Writer{}
	c := consumer{w: w}

	msg := cqrstest.NewMessage(cqrs.SubjectFromStr(subject))
	if err := msg.SetBody(body); err != nil {
		t.Fatalf("SetBody: %v", err)
	}
	return c.HandleMessage(msg)
}

func aReport() Reported {
	return Reported{
		ReportID:         "rep-1",
		Year:             "2026",
		Page:             PagePatrol,
		Ref:              "42",
		Reason:           "Ruten er ikke vores",
		ReporterPersonID: ReporterSentinel,
		ReportedAt:       time.Date(2026, 9, 21, 8, 30, 0, 0, time.UTC),
	}
}

func mustContain(t *testing.T, stmt string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(stmt, want) {
			t.Errorf("statement is missing %s\n%s", want, stmt)
		}
	}
}

func TestReportedIsWrittenToTheAuditTable(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.pagereport.rep-1.reported", aReport())

	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d", len(stmts))
	}
	mustContain(t, stmts[0],
		"INSERT INTO public_page_report", "ON DUPLICATE KEY UPDATE",
		`reportId="rep-1"`, `year="2026"`, `page="patrol"`, `ref="42"`,
		`reason="Ruten er ikke vores"`, `reporterPersonId="public"`,
		// The event's own timestamp, not NOW(): a replay must not restamp every report with the time
		// of the last deploy.
		`reportedAt="2026-09-21 08:30:00"`,
	)
	if strings.Contains(stmts[0], "NOW()") {
		t.Error("reportedAt must come off the event, or a replay rewrites history")
	}
}

// **The fold is idempotent**, because every table here is rebuilt from sequence zero on boot. Folding the
// same report twice must converge on one row rather than filing the complaint again.
func TestTheFoldIsIdempotent(t *testing.T) {
	first := fold(t, "NATHEJK.2026.pagereport.rep-1.reported", aReport())
	second := fold(t, "NATHEJK.2026.pagereport.rep-1.reported", aReport())

	if first[0] != second[0] {
		t.Errorf("the same event folded to two different statements\n%s\n%s", first[0], second[0])
	}
	if !strings.Contains(first[0], "ON DUPLICATE KEY UPDATE") {
		t.Error("without an upsert a replay inserts every report again")
	}
}

// **An address in the reporter field never reaches the table.** The publisher sends the sentinel already;
// this asserts the second lock, because the projection is the last point before something is written down
// and what must never be written down is an identifier for somebody outside the app.
func TestAnAddressInTheReporterFieldBecomesTheSentinel(t *testing.T) {
	for _, reporter := range []string{
		"192.168.1.34",
		"2001:db8::42",
		"10.0.0.1:51823",
		"", // nothing supplied at all
		"   ",
	} {
		body := aReport()
		body.ReporterPersonID = reporter

		stmts := fold(t, "NATHEJK.2026.pagereport.rep-1.reported", body)
		if !strings.Contains(stmts[0], `reporterPersonId="public"`) {
			t.Errorf("reporter %q was not replaced by the sentinel\n%s", reporter, stmts[0])
		}
		if reporter != "" && strings.TrimSpace(reporter) != "" &&
			strings.Contains(stmts[0], reporter) {
			t.Errorf("the statement still contains %q — an address was written down\n%s",
				reporter, stmts[0])
		}
	}
}

// A real person id is *not* mangled: the sentinel is for anonymity, not a blanket erasure. If an organizer
// tool ever files a report while signed in, the trail should say who.
func TestAKnownReporterIsKept(t *testing.T) {
	body := aReport()
	body.ReporterPersonID = "person-9"

	stmts := fold(t, "NATHEJK.2026.pagereport.rep-1.reported", body)
	mustContain(t, stmts[0], `reporterPersonId="person-9"`)
}

func TestAReportWithNoRefIsRefused(t *testing.T) {
	body := aReport()
	body.Ref = ""

	if err := foldErr(t, "NATHEJK.2026.pagereport.rep-1.reported", body); err == nil {
		t.Error("a report about no page must be refused, not stored as a handled complaint")
	}
}

// The id falls back to the subject, which is where it also appears. A body missing it is a publisher bug,
// not a reason to drop somebody's complaint.
func TestTheIDFallsBackToTheSubject(t *testing.T) {
	body := aReport()
	body.ReportID = ""

	stmts := fold(t, "NATHEJK.2026.pagereport.rep-7.reported", body)
	mustContain(t, stmts[0], `reportId="rep-7"`)
}

// A subject this projection did not ask for is a no-op, not an error: the stream carries many verbs and
// erroring on the rest fills the log with noise about events that were never ours.
func TestAnUnrelatedSubjectIsANoOp(t *testing.T) {
	stmts := fold(t, "NATHEJK.2026.glimt.g-1.created", aReport())
	if len(stmts) != 0 {
		t.Errorf("want no statements for an unrelated subject, got %v", stmts)
	}
}

func TestSubjectBuildsTheVerb(t *testing.T) {
	s, err := Subject("2026", "rep-1", VerbReported)
	if err != nil {
		t.Fatalf("Subject: %v", err)
	}
	if got, want := s.Subject(), "NATHEJK.2026.pagereport.rep-1.reported"; got != want {
		t.Errorf("Subject() = %q, want %q", got, want)
	}
	// And the subject it builds is one the consumer subscribes to — the pairing that a typo in either
	// place breaks silently, since an unmatched subject is delivered to nothing.
	if !s.Match(consumer{}.Consumes()[0].Subject()) {
		t.Errorf("%s does not match the subscription %s", s.Subject(), consumer{}.Consumes()[0].Subject())
	}
}

func TestSubjectRejectsATokenThatWouldSplit(t *testing.T) {
	if _, err := Subject("2026", "rep.1", VerbReported); err == nil {
		t.Error("an id with a dot must be refused: it publishes fine and matches nothing")
	}
	if _, err := Subject("", "rep-1", VerbReported); err == nil {
		t.Error("an empty year must be refused")
	}
}
