package maphandout

import (
	"fmt"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/messages"
)

// consumer folds QR registrations into the read model.
type consumer struct {
	w cqrs.Writer
}

// qrRegistered is the shared registered body plus the sheet handed over.
//
// The map id is an **additive JSON field that skan adds** to the shared body, so it is not on
// `messages.NathejkQrRegistered` and cannot be read without a struct that embeds the shared one and
// names it.
//
// Worth knowing how we found out: this repo's vendored copy of hq's contract still says the
// QR-to-sheet link "is not built", and reading only that document would have led to the conclusion
// that a per-patrol handout list was impossible. It exists, and hq's own patrol page renders it. The
// lesson is recorded in PRD 016 §11.11 — the code is the contract, the document is a guide to it.
//
// An absent `mapId` (older events) reads as "" — unknown sheet, not no sheet.
type qrRegistered struct {
	messages.NathejkQrRegistered
	MapID string `json:"mapId,omitempty"`
}

// Consumes lists the subjects this projection subscribes to.
//
// Only `registered`. See the package doc for why `scanned` is a different thing entirely.
func (c consumer) Consumes() []cqrs.Subject {
	return []cqrs.Subject{
		cqrs.SubjectFromStr("NATHEJK.*.qr.*.registered"),
	}
}

// HandleMessage folds one registration into the read model.
//
// Errors are annotated with the subject for the reason recorded in the person and checkpoint
// packages: the stream library logs a handler error and *drops* the message rather than
// dead-lettering it, so the log line is the only trace it existed, and a bare decode error is
// unattributable among tens of thousands of messages.
func (c consumer) HandleMessage(msg cqrs.Message) error {
	subject := msg.Subject()
	if err := c.handleMessage(msg, subject); err != nil {
		return fmt.Errorf("maphandout: %s: %w", subject.Subject(), err)
	}
	return nil
}

func (c consumer) handleMessage(msg cqrs.Message, subject cqrs.Subject) error {
	if !subject.Match("nathejk.*.qr.*.registered") {
		return nil
	}

	year := subjectYear(subject)
	if year == "" {
		return fmt.Errorf("no year in subject")
	}

	var body qrRegistered
	if err := msg.Body(&body); err != nil {
		return err
	}

	// A binding with no team is not a handout to anyone — a code registered before it was given
	// out, typically. Nothing to record, and recording it under an empty team id would put a row in
	// every patrol's history that no patrol was ever handed.
	if body.TeamID == "" {
		return nil
	}

	qrID := string(body.QrID)
	if qrID == "" {
		qrID = subjectEntityID(subject)
	}
	if qrID == "" {
		return fmt.Errorf("qr registered with no qrId")
	}

	uts := msg.Time().Unix()

	// ON DUPLICATE KEY UPDATE rather than INSERT IGNORE: the same code can be bound to the same team
	// more than once (a re-scan), and the row should track the latest registrar and widen
	// [firstUts, lastUts] rather than being discarded.
	//
	//   - mapId: never overwritten with "". Same rule skan follows — a code re-scanned before its
	//     sheet was recorded must not lose the sheet it already names. Written as an IF() rather
	//     than decided in Go, because the comparison is against the *stored* value, which this
	//     process does not have.
	//   - firstUts/lastUts: LEAST/GREATEST, so the interval this team held the code is correct
	//     whatever order the log replays in. Replay is not hypothetical: the projection is rebuilt
	//     from sequence zero on every boot.
	return c.w.Consume(fmt.Sprintf(
		"INSERT INTO maphandout SET year=%s, qrId=%s, teamId=%s, "+
			"mapId=%s, registeredBy=%s, firstUts=%d, lastUts=%d "+
			"ON DUPLICATE KEY UPDATE "+
			"mapId=IF(VALUES(mapId) = '', mapId, VALUES(mapId)), "+
			"registeredBy=VALUES(registeredBy), "+
			"firstUts=LEAST(firstUts, VALUES(firstUts)), "+
			"lastUts=GREATEST(lastUts, VALUES(lastUts))",
		quote(year), quote(qrID), quote(string(body.TeamID)),
		quote(body.MapID), quote(body.ScannerID), uts, uts))
}

// subjectYear extracts the year from NATHEJK.<year>.qr....
func subjectYear(s cqrs.Subject) string {
	parts := s.Parts()
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// subjectEntityID extracts the QR id, which is the part before the verb.
//
// The subject is authoritative where it disagrees with the body: that is what the stream routed on.
func subjectEntityID(s cqrs.Subject) string {
	parts := s.Parts()
	if len(parts) < 4 {
		return ""
	}
	return parts[3]
}

// quote renders a Go string as a SQL string literal.
//
// Same reasoning as the checkpoint and person packages: cqrs.Writer takes a finished statement, not a
// statement plus arguments, so escaping is this file's responsibility rather than the driver's — and
// these values come from another service's event bodies, which is not the same as trusted input.
func quote(s string) string { return fmt.Sprintf("%q", s) }

var _ cqrs.Consumer = consumer{}
