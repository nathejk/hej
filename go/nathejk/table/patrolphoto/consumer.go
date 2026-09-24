package patrolphoto

import (
	"fmt"
	"strings"
	"time"

	"github.com/jrgensen/cqrs"
)

// consumer folds foto's photograph events into the two tables.
//
// Every fold is idempotent, because the tables are rebuilt from sequence zero on every boot: the writes are
// upserts keyed by content hash, and re-delivering the same photograph rewrites one row with identical values.
type consumer struct {
	w cqrs.Writer
}

// Consumes lists the four verbs.
//
// The colon after NATHEJK is how upstream spells these and `SubjectFromStr` normalises it to a dot — kept to
// match foto's and hq's own declarations character for character, so grepping any of the three repos for how
// these events are spelled gives one answer.
func (c consumer) Consumes() []cqrs.Subject {
	return []cqrs.Subject{
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.photographed"),
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.photopurged"),
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.photocoverselected"),
		cqrs.SubjectFromStr("NATHEJK:*.patrulje.*.photoconsented"),
	}
}

// HandleMessage folds one event.
//
// Errors are annotated with the subject, for the reason every projection here records: the stream library logs a
// handler error and *drops* the message, so the log line is the only trace it existed.
func (c consumer) HandleMessage(msg cqrs.Message) error {
	subject := msg.Subject()
	if err := c.handleMessage(msg, subject); err != nil {
		return fmt.Errorf("patrolphoto: %s: %w", subject.Subject(), err)
	}
	return nil
}

func (c consumer) handleMessage(msg cqrs.Message, subject cqrs.Subject) error {
	switch {
	case subject.Match("NATHEJK.*.patrulje.*.photographed"):
		return c.handlePhotographed(msg, subject)
	case subject.Match("NATHEJK.*.patrulje.*.photopurged"):
		return c.handlePurged(msg, subject)
	case subject.Match("NATHEJK.*.patrulje.*.photocoverselected"):
		return c.handleCoverSelected(msg, subject)
	case subject.Match("NATHEJK.*.patrulje.*.photoconsented"):
		return c.handleConsented(msg, subject)
	}
	// A subject that matches nothing is a no-op, not an error: this consumer subscribes to four verbs and the
	// stream carries many.
	return nil
}

// handlePhotographed records one photograph.
func (c consumer) handlePhotographed(msg cqrs.Message, subject cqrs.Subject) error {
	var body photographed
	if err := msg.Body(&body); err != nil {
		return err
	}

	year, teamID := identify(body.Year, body.TeamID, subject)
	if year == "" || teamID == "" {
		return fmt.Errorf("photographed with no year or team")
	}
	// **Refused rather than stored.** A ref is this row's identity, the key in the blob store, and later a
	// segment of a URL. A row with a malformed one is unusable and would be recreated by every replay.
	if !validRef(body.Ref) {
		return fmt.Errorf("photographed with an invalid ref %q", body.Ref)
	}

	return c.w.Consume(fmt.Sprintf(
		"INSERT INTO patrol_photo SET year=%s, teamId=%s, type=%s, ref=%s, thumbRef=%s, contentType=%s, "+
			"width=%d, height=%d, attention=%d, capturedAt=%s "+
			"ON DUPLICATE KEY UPDATE thumbRef=VALUES(thumbRef), contentType=VALUES(contentType), "+
			"width=VALUES(width), height=VALUES(height), attention=VALUES(attention), "+
			"capturedAt=VALUES(capturedAt)",
		quote(year), quote(teamID), quote(body.Type), quote(body.Ref),
		quote(smallestRendition(body.Renditions)), quote(body.ContentType),
		body.Width, body.Height, boolToInt(body.Attention), datetime(body.CapturedAt),
	))
}

// handlePurged removes exactly the named photographs, and any cover that pointed at one.
//
// # Why the cover is cleared here
//
// Because a purge is how a request to remove a child's photograph is honoured, and a cover row still naming the
// purged ref would leave the diploma and the gallery asking for bytes that must no longer be shown. Clearing it
// is not tidiness: it is the difference between a deletion that took effect and one that only looks like it did.
//
// Refs are validated before use: they arrive in an event body and go into a SQL literal.
func (c consumer) handlePurged(msg cqrs.Message, subject cqrs.Subject) error {
	var body photoPurged
	if err := msg.Body(&body); err != nil {
		return err
	}

	year, teamID := identify(body.Year, body.TeamID, subject)
	if year == "" || teamID == "" {
		return fmt.Errorf("photopurged with no year or team")
	}

	quoted := make([]string, 0, len(body.Refs))
	for _, ref := range body.Refs {
		if validRef(ref) {
			quoted = append(quoted, quote(ref))
		}
	}
	if len(quoted) == 0 {
		// **Not a delete-everything.** A purge naming no usable ref must remove nothing: the caller who meant
		// "this blurred one" would otherwise erase the patrol's whole season.
		return fmt.Errorf("photopurged named no valid ref")
	}
	list := strings.Join(quoted, ", ")

	if err := c.w.Consume(fmt.Sprintf(
		"DELETE FROM patrol_photo WHERE year=%s AND teamId=%s AND ref IN (%s)",
		quote(year), quote(teamID), list,
	)); err != nil {
		return err
	}
	return c.w.Consume(fmt.Sprintf(
		"UPDATE patrol_photo_cover SET ref=\"\" WHERE year=%s AND teamId=%s AND ref IN (%s)",
		quote(year), quote(teamID), list,
	))
}

// handleCoverSelected records which photograph represents the patrol.
//
// An empty ref is valid and means "no choice", which is how a selection is undone. Anything else must be
// ref-shaped: a cover is read straight into a blob lookup.
func (c consumer) handleCoverSelected(msg cqrs.Message, subject cqrs.Subject) error {
	var body coverSelected
	if err := msg.Body(&body); err != nil {
		return err
	}

	year, teamID := identify(body.Year, body.TeamID, subject)
	if year == "" || teamID == "" {
		return fmt.Errorf("photocoverselected with no year or team")
	}
	if body.Ref != "" && !validRef(body.Ref) {
		return fmt.Errorf("photocoverselected with an invalid ref %q", body.Ref)
	}

	return c.w.Consume(fmt.Sprintf(
		"INSERT INTO patrol_photo_cover SET year=%s, teamId=%s, ref=%s, selectedAt=%s "+
			"ON DUPLICATE KEY UPDATE ref=VALUES(ref), selectedAt=VALUES(selectedAt)",
		quote(year), quote(teamID), quote(body.Ref), datetime(body.SelectedAt),
	))
}

// handleConsented records whether the patrol's photographs may be used at all.
//
// A plain overwrite, because the newest message is the whole decision: clearing every box in hq publishes
// `teamRefused:false` with no members, and that must restore consent rather than be ignored. The photographs
// themselves are left in place — a refusal can be withdrawn — and are kept out of every read instead.
func (c consumer) handleConsented(msg cqrs.Message, subject cqrs.Subject) error {
	var body PhotoConsentSet
	if err := msg.Body(&body); err != nil {
		return err
	}
	year, teamID := identify("", string(body.TeamID), subject)
	if year == "" || teamID == "" {
		return fmt.Errorf("photoconsented with no year or team")
	}
	return c.w.Consume(fmt.Sprintf(
		"INSERT INTO patrol_photo_consent SET year=%s, teamId=%s, refused=%d "+
			"ON DUPLICATE KEY UPDATE refused=VALUES(refused)",
		quote(year), quote(teamID), boolToInt(body.Refused()),
	))
}

// identify prefers the body's own ids and falls back to the subject's tokens.
//
//	NATHEJK . <year> . patrulje . <teamId> . <verb>
//	   0         1         2          3         4
//
// The subject is the more trustworthy of the two — the broker matched on it, so it cannot have been silently
// wrong in the way a body can — but the body is what foto considers authoritative, and the two agree in practice.
// Taking the body first and the subject as a fallback is what hq does, and two projections of one event
// disagreeing about which team it belongs to would be the worst possible bug here.
func identify(year, teamID string, subject cqrs.Subject) (string, string) {
	parts := subject.Parts()
	if year == "" && len(parts) > 1 {
		year = parts[1]
	}
	if teamID == "" && len(parts) > 3 {
		teamID = parts[3]
	}
	return year, teamID
}

// datetime renders a time as a SQL literal, or NULL for the zero value.
//
// NULL rather than "0000-00-00": a photograph whose capturedAt is missing has an unknown capture time, and a
// zero date sorts as the oldest photograph ever taken — which is exactly the wrong answer for a query that
// picks the newest.
func datetime(t time.Time) string {
	if t.IsZero() {
		return "NULL"
	}
	return quote(t.UTC().Format("2006-01-02 15:04:05"))
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// quote renders a Go string as a SQL string literal. Same reasoning as every other projection here: cqrs.Writer
// takes a finished statement, so escaping is this file's responsibility.
func quote(s string) string { return fmt.Sprintf("%q", s) }

var _ cqrs.Consumer = consumer{}
