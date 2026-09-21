package pagereport

import (
	"fmt"
	"strings"

	"github.com/jrgensen/cqrs"
)

// consumer folds a report into the audit table.
//
// The fold is an `INSERT ... ON DUPLICATE KEY UPDATE` keyed by report id, which makes it idempotent: the
// table is rebuilt from sequence zero on every boot, and a replay must produce the same rows rather than a
// second copy of every report ever filed.
type consumer struct {
	w cqrs.Writer
}

// Consumes subscribes to every year's reports.
//
// A dot after NATHEJK, not a colon: this is an event *this* app publishes, like glimt and album, and the
// colon spelling belongs to the upstream services' subjects.
func (c consumer) Consumes() []cqrs.Subject {
	return []cqrs.Subject{
		cqrs.SubjectFromStr("NATHEJK.*.pagereport.*.reported"),
	}
}

// HandleMessage folds one event.
//
// Errors are annotated with the subject for the reason every projection here records: the stream library
// logs a handler error and drops the message, so the log line is the only trace it existed.
func (c consumer) HandleMessage(msg cqrs.Message) error {
	subject := msg.Subject()
	if err := c.handleMessage(msg, subject); err != nil {
		return fmt.Errorf("pagereport: %s: %w", subject.Subject(), err)
	}
	return nil
}

func (c consumer) handleMessage(msg cqrs.Message, subject cqrs.Subject) error {
	year := subjectYear(subject)
	if year == "" {
		return fmt.Errorf("no year in subject")
	}
	if !subject.Match("NATHEJK.*.pagereport.*.reported") {
		// A no-op rather than an error: this projection subscribes to one verb and the stream carries
		// many, so erroring on the rest would fill the log with noise about events that were never ours.
		return nil
	}

	var body Reported
	if err := msg.Body(&body); err != nil {
		return err
	}

	reportID := body.ReportID
	if reportID == "" {
		reportID = subjectEntityID(subject)
	}
	if reportID == "" {
		// Refused rather than stored. A row with no id cannot be addressed and a replay would insert a
		// fresh one every boot, so this would be rubbish that multiplies.
		return fmt.Errorf("page report with no reportId")
	}
	if body.Ref == "" {
		// A report about nothing is not a report. Refused loudly, because the interesting failure this
		// catches is a publisher that forgot to fill in which page was being complained about — and a row
		// with an empty `ref` would sit in the table looking like a handled complaint.
		return fmt.Errorf("page report with no ref")
	}

	// **The reporter column is normalised to the sentinel here, not just at the publisher.**
	//
	// The publisher already sends `public`, and this is the second lock on the same door: the projection
	// is the last point before something is written down, and what must never be written down is an
	// identifier for somebody outside the app. If a future caller ever sends an address in this field, it
	// stops here instead of landing in an append-only table.
	reporter := strings.TrimSpace(body.ReporterPersonID)
	if reporter == "" || looksLikeAddress(reporter) {
		reporter = ReporterSentinel
	}

	// reportedAt comes off the event rather than being NOW(): a replay must not restamp every report with
	// the time of the last deploy.
	return c.w.Consume(fmt.Sprintf(
		"INSERT INTO public_page_report SET reportId=%s, year=%s, page=%s, ref=%s, reason=%s, "+
			"reporterPersonId=%s, reportedAt=%s "+
			"ON DUPLICATE KEY UPDATE year=VALUES(year), page=VALUES(page), ref=VALUES(ref), "+
			"reason=VALUES(reason), reporterPersonId=VALUES(reporterPersonId), "+
			"reportedAt=VALUES(reportedAt)",
		quote(reportID), quote(year), quote(body.Page), quote(body.Ref), quote(body.Reason),
		quote(reporter), quote(body.ReportedAt.UTC().Format("2006-01-02 15:04:05")),
	))
}

// ReporterSentinel stands in for an anonymous reporter, as PRD 019 established for glimt reports.
//
// Recognisable in a report list as "somebody outside the app", which is a useful thing for an organizer to
// know: a report from the open web has different weight to one from a participant.
const ReporterSentinel = "public"

// looksLikeAddress is a crude guard against an IP arriving in the reporter field.
//
// Crude on purpose: it is not validating input, it is refusing a *category* of value that this table must
// never contain, and the cost of a false positive is that an identifier becomes the sentinel — which is
// the safe direction. Anything with a colon (IPv6, or host:port) or four dot-separated numeric groups
// (IPv4) is treated as an address.
func looksLikeAddress(s string) bool {
	if strings.Contains(s, ":") {
		return true
	}
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

// Subject builds the subject for one report:
//
//	NATHEJK.<year>.pagereport.<reportId>.reported
func Subject(year, reportID, verb string) (cqrs.Subject, error) {
	if err := validSubjectToken(year, "year"); err != nil {
		return nil, err
	}
	if err := validSubjectToken(reportID, "report id"); err != nil {
		return nil, err
	}
	if err := validSubjectToken(verb, "verb"); err != nil {
		return nil, err
	}
	return cqrs.SubjectFromStr(
		fmt.Sprintf("NATHEJK.%s.pagereport.%s.%s", year, reportID, verb)), nil
}

// VerbReported is the only verb. A constant anyway, so a handler cannot publish a spelling the projection
// does not listen for — a typo that fails silently, since an unmatched subject is delivered to nothing.
const VerbReported = "reported"

// validSubjectToken rejects anything that would not survive as a single NATS subject token.
//
// Same guard as the album package's, for the same reason: an id containing a dot splits into extra tokens,
// still matches `NATHEJK.>` and publishes successfully — while no longer matching the per-entity pattern.
func validSubjectToken(s, what string) error {
	if s == "" {
		return fmt.Errorf("%s is empty", what)
	}
	if strings.ContainsAny(s, ". \t\r\n*>") {
		return fmt.Errorf("%s %q is not a valid subject token", what, s)
	}
	return nil
}

// subjectYear extracts the year, which is the second token.
func subjectYear(s cqrs.Subject) string {
	parts := s.Parts()
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// subjectEntityID extracts the report id, which is the part before the verb.
func subjectEntityID(s cqrs.Subject) string {
	parts := s.Parts()
	if len(parts) < 4 {
		return ""
	}
	return parts[3]
}

// quote renders a Go string as a SQL string literal.
//
// Same reasoning as every other consumer here: cqrs.Writer takes a finished statement rather than a
// statement plus arguments, so escaping is this file's responsibility — and `reason` is free text typed by
// an anonymous stranger, which makes it the least trusted input in the repository.
func quote(s string) string { return fmt.Sprintf("%q", s) }

var _ cqrs.Consumer = consumer{}
