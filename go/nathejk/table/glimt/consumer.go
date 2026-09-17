package glimt

import (
	"fmt"
	"strings"
	"time"

	"github.com/jrgensen/cqrs"
)

// consumer folds the Glimt events into the read model.
//
// Every fold is idempotent, because these tables are rebuilt from sequence zero on every boot:
// creates are upserts, the takedown events set an absolute state rather than toggling, and a report
// is keyed by its reporter so a replay cannot inflate the count. "Idempotent" is not a nice
// property here, it is the only way the numbers survive a restart.
type consumer struct {
	w cqrs.Writer
}

// Consumes lists the subjects this projection subscribes to.
//
// Dot form after NATHEJK, matching what this app already publishes for `portrait`. (Several
// upstream producers use a colon — `NATHEJK:*.klan.*.signedup` — but those are their events, not
// ours, and copying the quirk into a new entity would spread it.)
func (c consumer) Consumes() []cqrs.Subject {
	return []cqrs.Subject{
		cqrs.SubjectFromStr("NATHEJK.*.glimt.*.created"),
		cqrs.SubjectFromStr("NATHEJK.*.glimt.*.deleted"),
		cqrs.SubjectFromStr("NATHEJK.*.glimt.*.reported"),
		cqrs.SubjectFromStr("NATHEJK.*.glimt.*.hidden"),
		cqrs.SubjectFromStr("NATHEJK.*.glimt.*.unhidden"),
		cqrs.SubjectFromStr("NATHEJK.*.glimt.*.purged"),
	}
}

// HandleMessage folds one event into the read model.
//
// Errors are annotated with the subject for the reason the person, checkpoint and maphandout
// packages all record: the stream library logs a handler error and *drops* the message rather than
// dead-lettering it, so the log line is the only trace it existed, and a bare decode error is
// unattributable among tens of thousands of messages.
func (c consumer) HandleMessage(msg cqrs.Message) error {
	subject := msg.Subject()
	if err := c.handleMessage(msg, subject); err != nil {
		return fmt.Errorf("glimt: %s: %w", subject.Subject(), err)
	}
	return nil
}

func (c consumer) handleMessage(msg cqrs.Message, subject cqrs.Subject) error {
	year := subjectYear(subject)
	if year == "" {
		return fmt.Errorf("no year in subject")
	}

	switch {
	case subject.Match("nathejk.*.glimt.*.created"):
		return c.handleCreated(msg, year)
	case subject.Match("nathejk.*.glimt.*.deleted"):
		return c.handleDeleted(msg, year)
	case subject.Match("nathejk.*.glimt.*.reported"):
		return c.handleReported(msg, year)
	case subject.Match("nathejk.*.glimt.*.hidden"):
		return c.handleHidden(msg, year)
	case subject.Match("nathejk.*.glimt.*.unhidden"):
		return c.handleUnhidden(msg, year)
	case subject.Match("nathejk.*.glimt.*.purged"):
		return c.handlePurged(msg, year)
	}
	return nil
}

// handleCreated writes the glimt and its media.
//
// An upsert on the parent and a delete-then-insert on the media, so a replay converges instead of
// accumulating: the media are an ordered list owned entirely by this one event, and the honest way
// to re-apply a list is to replace it.
//
// Note what is *not* reset: `hiddenAt`, `hiddenBy` and `reportCount`. Those come from later events,
// and a replay applies this one first — so writing zeros here would be correct on the way through,
// while writing them on a re-delivery of an *old* create after a hide would silently un-hide a
// reported glimt. Left out of the update clause entirely rather than reasoned about per replay.
func (c consumer) handleCreated(msg cqrs.Message, year string) error {
	var body Created
	if err := msg.Body(&body); err != nil {
		return err
	}

	glimtID := body.GlimtID
	if glimtID == "" {
		glimtID = subjectEntityID(msg.Subject())
	}
	if glimtID == "" {
		return fmt.Errorf("glimt created with no glimtId")
	}
	if body.AuthorPersonID == "" {
		// Refused rather than stored: ownership is the only thing that authorises a
		// delete, and a glimt nobody owns is a photo its subject cannot get taken down
		// by the person who posted it. An unowned row would also make the "own glimt"
		// visibility branch match every caller with an empty id.
		return fmt.Errorf("glimt created with no authorPersonId")
	}
	if body.Audience == "" {
		return fmt.Errorf("glimt created with no audience")
	}

	media := body.validMedia()
	if len(media) == 0 {
		// A glimt is media plus an optional caption; with no usable media there is
		// nothing to show. Failing keeps the disagreement visible rather than putting an
		// empty card in everyone's feed.
		return fmt.Errorf("glimt created with no valid media")
	}

	createdAt := body.CreatedAt
	if createdAt.IsZero() {
		// No replay-stable fallback exists — time.Now() would differ on every rebuild,
		// and retention (task 310) measures from this column. Refusing is better than a
		// timestamp that moves, which would make the purge window unpredictable for one
		// row and only for that row.
		return fmt.Errorf("glimt created with no createdAt")
	}

	if err := c.w.Consume(fmt.Sprintf(
		"INSERT INTO glimt SET glimtId=%s, year=%s, authorPersonId=%s, authorGroup=%s, "+
			"teamNumber=%s, teamName=%s, audience=%s, caption=%s, createdAt=%s, "+
			"mediaCount=%d, deleted=0 "+
			"ON DUPLICATE KEY UPDATE "+
			"authorPersonId=VALUES(authorPersonId), authorGroup=VALUES(authorGroup), "+
			"teamNumber=VALUES(teamNumber), teamName=VALUES(teamName), "+
			"audience=VALUES(audience), caption=VALUES(caption), "+
			"createdAt=VALUES(createdAt), mediaCount=VALUES(mediaCount)",
		quote(glimtID), quote(year), quote(body.AuthorPersonID), quote(body.AuthorGroup),
		quote(body.TeamNumber), quote(body.TeamName), quote(body.Audience),
		quote(body.Caption), quote(formatTime(createdAt)), len(media),
	)); err != nil {
		return err
	}

	// Replace the media list wholesale. Ordinals come from the event rather than from the
	// loop index, so a gap in the author's numbering survives instead of being silently
	// renumbered into a different order.
	if err := c.w.Consume(fmt.Sprintf(
		"DELETE FROM glimt_media WHERE glimtId=%s AND year=%s",
		quote(glimtID), quote(year),
	)); err != nil {
		return err
	}
	for _, m := range media {
		if err := c.w.Consume(fmt.Sprintf(
			"INSERT INTO glimt_media SET glimtId=%s, year=%s, ordinal=%d, blobRef=%s, "+
				"thumbRef=%s, kind=%s, contentType=%s, bytes=%d, width=%d, height=%d, "+
				"durationMs=%d "+
				"ON DUPLICATE KEY UPDATE blobRef=VALUES(blobRef), thumbRef=VALUES(thumbRef), "+
				"kind=VALUES(kind), contentType=VALUES(contentType), bytes=VALUES(bytes), "+
				"width=VALUES(width), height=VALUES(height), durationMs=VALUES(durationMs)",
			quote(glimtID), quote(year), m.Ordinal, quote(m.Ref), quote(m.ThumbRef),
			quote(m.kindOrDefault()), quote(m.ContentType), m.Bytes, m.Width, m.Height,
			m.DurationMs,
		)); err != nil {
			return err
		}
	}
	return nil
}

// validMedia returns the items with a usable content hash, in ordinal order.
//
// A malformed ref costs that item, not the whole glimt — the same rule the portrait fold applies to
// a rendition. Dropping one photo from a set of five is recoverable; refusing the event would lose
// the other four as well. If *every* item is unusable the caller fails, because then there is
// nothing to show.
func (c Created) validMedia() []Media {
	out := make([]Media, 0, len(c.Media))
	for _, m := range c.Media {
		if !validRef(m.Ref) {
			continue
		}
		if !validRef(m.ThumbRef) {
			// Not fatal for the item: a thumbnail that cannot be addressed just means
			// the client falls back to the full media. Blanked rather than stored so
			// nothing later builds a URL from it.
			m.ThumbRef = ""
		}
		out = append(out, m)
	}
	return out
}

// kindOrDefault treats an unrecognised kind as an image.
//
// Deliberately lenient in one direction only. Calling an unknown thing an image means the client
// renders an <img> that may fail — visible and harmless. The reverse, treating an unknown thing as
// video, would have the client hand arbitrary bytes to a media element and try to play them.
func (m Media) kindOrDefault() string {
	if m.Kind == MediaKindVideo {
		return MediaKindVideo
	}
	return MediaKindImage
}

// handleDeleted tombstones the glimt and drops its media rows.
//
// The blobs are purged by the handler that published this event, not here: this projection has no
// blob store and must not have one. The media rows go because nothing may serve them afterwards,
// while the parent row stays so a replay cannot resurrect the glimt and so a feed can tell "gone"
// from "never existed".
func (c consumer) handleDeleted(msg cqrs.Message, year string) error {
	var body Deleted
	if err := msg.Body(&body); err != nil {
		return err
	}
	glimtID := c.entityID(body.GlimtID, msg)
	if glimtID == "" {
		return fmt.Errorf("glimt deleted with no glimtId")
	}

	if err := c.w.Consume(fmt.Sprintf(
		"UPDATE glimt SET deleted=1, mediaCount=0 WHERE glimtId=%s AND year=%s",
		quote(glimtID), quote(year),
	)); err != nil {
		return err
	}
	return c.w.Consume(fmt.Sprintf(
		"DELETE FROM glimt_media WHERE glimtId=%s AND year=%s",
		quote(glimtID), quote(year),
	))
}

// handleReported records the report **and hides the glimt in the same fold**.
//
// The hide is not a separate step and must not become one. The public scope publishes with no
// approval queue (PRD 019 §0), so a report is the only fast mechanism there is; a gap between
// recording a report and acting on it is a gap in which the thing somebody objected to is still on
// the open web. Two statements rather than one because they touch two tables, but no decision sits
// between them.
//
// `hiddenAt` is only set if it is not already, so a second report does not reset the timestamp of
// the first takedown — the moderation queue sorts on it.
func (c consumer) handleReported(msg cqrs.Message, year string) error {
	var body Reported
	if err := msg.Body(&body); err != nil {
		return err
	}
	glimtID := c.entityID(body.GlimtID, msg)
	if glimtID == "" {
		return fmt.Errorf("glimt reported with no glimtId")
	}
	if body.ReporterPersonID == "" {
		// The reporter is the primary key that makes the count honest: without it, one
		// person tapping twice would look like two people objecting, which is exactly
		// the signal the moderation queue sorts on.
		return fmt.Errorf("glimt reported with no reporterPersonId")
	}

	reportedAt := body.ReportedAt
	if reportedAt.IsZero() {
		reportedAt = msg.Time().UTC()
	}

	// INSERT IGNORE, not an upsert: a repeat report from the same person is not new
	// information, and letting it rewrite the row would move the first report's timestamp.
	if err := c.w.Consume(fmt.Sprintf(
		"INSERT IGNORE INTO glimt_report SET glimtId=%s, year=%s, reporterPersonId=%s, "+
			"reason=%s, createdAt=%s",
		quote(glimtID), quote(year), quote(body.ReporterPersonID),
		quote(body.Reason), quote(formatTime(reportedAt)),
	)); err != nil {
		return err
	}

	// The count is derived from the report table rather than incremented, so it is
	// replay-stable: `reportCount = reportCount + 1` would climb on every rebuild.
	return c.w.Consume(fmt.Sprintf(
		"UPDATE glimt SET "+
			"reportCount=(SELECT COUNT(*) FROM glimt_report r WHERE r.glimtId=%s AND r.year=%s), "+
			"hiddenAt=IF(hiddenAt IS NULL, %s, hiddenAt) "+
			"WHERE glimtId=%s AND year=%s",
		quote(glimtID), quote(year), quote(formatTime(reportedAt)),
		quote(glimtID), quote(year),
	))
}

// handleHidden records a Team-section takedown.
//
// Sets an absolute state rather than flipping one, so a replay converges. The media survives —
// that is the whole difference from a delete, and what makes hiding cheap enough to be the default
// response to a report.
func (c consumer) handleHidden(msg cqrs.Message, year string) error {
	var body Hidden
	if err := msg.Body(&body); err != nil {
		return err
	}
	glimtID := c.entityID(body.GlimtID, msg)
	if glimtID == "" {
		return fmt.Errorf("glimt hidden with no glimtId")
	}
	hiddenAt := body.HiddenAt
	if hiddenAt.IsZero() {
		hiddenAt = msg.Time().UTC()
	}
	return c.w.Consume(fmt.Sprintf(
		"UPDATE glimt SET hiddenAt=%s, hiddenBy=%s WHERE glimtId=%s AND year=%s",
		quote(formatTime(hiddenAt)), quote(body.HiddenBy), quote(glimtID), quote(year),
	))
}

// handleUnhidden restores a glimt.
//
// Clears `hiddenBy` along with the timestamp: leaving the name of whoever hid it on a visible row
// would read, to the next moderator, as though it were still hidden by them. The audit trail lives
// on the stream, which is the right place for it.
func (c consumer) handleUnhidden(msg cqrs.Message, year string) error {
	var body Unhidden
	if err := msg.Body(&body); err != nil {
		return err
	}
	glimtID := c.entityID(body.GlimtID, msg)
	if glimtID == "" {
		return fmt.Errorf("glimt unhidden with no glimtId")
	}
	return c.w.Consume(fmt.Sprintf(
		"UPDATE glimt SET hiddenAt=NULL, hiddenBy='' WHERE glimtId=%s AND year=%s",
		quote(glimtID), quote(year),
	))
}

// handlePurged folds a retention deletion.
//
// The same row effect as a delete, on purpose: the difference between the two is *why*, and that
// difference is recorded on the stream rather than in a column. Nobody reading the feed needs to
// know whether a glimt went because its author removed it or because the window closed; anybody
// auditing it reads the log.
func (c consumer) handlePurged(msg cqrs.Message, year string) error {
	var body Purged
	if err := msg.Body(&body); err != nil {
		return err
	}
	glimtID := c.entityID(body.GlimtID, msg)
	if glimtID == "" {
		return fmt.Errorf("glimt purged with no glimtId")
	}
	if err := c.w.Consume(fmt.Sprintf(
		"UPDATE glimt SET deleted=1, mediaCount=0 WHERE glimtId=%s AND year=%s",
		quote(glimtID), quote(year),
	)); err != nil {
		return err
	}
	return c.w.Consume(fmt.Sprintf(
		"DELETE FROM glimt_media WHERE glimtId=%s AND year=%s",
		quote(glimtID), quote(year),
	))
}

// entityID prefers the body's id and falls back to the subject's.
//
// The subject is authoritative where they disagree, because that is what the stream routed on — but
// the body is what a publisher filled in deliberately, so it is tried first and the subject covers
// an omission.
func (c consumer) entityID(fromBody string, msg cqrs.Message) string {
	if fromBody != "" {
		return fromBody
	}
	return subjectEntityID(msg.Subject())
}

// subjectYear extracts the year from NATHEJK.<year>.glimt....
func subjectYear(s cqrs.Subject) string {
	parts := s.Parts()
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// subjectEntityID extracts the glimt id, which is the part before the verb.
func subjectEntityID(s cqrs.Subject) string {
	parts := s.Parts()
	if len(parts) < 4 {
		return ""
	}
	return parts[3]
}

// validRef reports whether ref looks like a content hash from the blob store.
//
// Duplicates blob.Ref.Valid rather than calling it, for the reason person/portrait.go records: this
// package may not import internal/... . The check is 64 lowercase hex characters — a sha256 in hex
// — and it matters because a Ref arrives in an event body and ends up in a SQL statement and later
// in a URL path. "../../etc/passwd" is a Ref-shaped string.
func validRef(ref string) bool {
	if len(ref) != 64 {
		return false
	}
	for _, c := range ref {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}

// formatTime renders a time as MariaDB's DATETIME literal, in UTC.
//
// Always UTC: the column has no zone, so writing local time would make the retention window and
// the feed order depend on the server's clock configuration.
func formatTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05")
}

// quote renders a Go string as a SQL string literal.
//
// Same reasoning as the person, checkpoint and maphandout packages: cqrs.Writer takes a finished
// statement rather than a statement plus arguments, so escaping is this file's responsibility. These
// values are captions typed by participants, which is the least trusted input in the whole service.
func quote(s string) string { return fmt.Sprintf("%q", s) }

// validSubjectToken rejects anything that would not survive as a single NATS subject token.
//
// Not cosmetic, and the reason is the same one person/portrait.go gives: an id containing a dot
// splits into extra tokens, still matches `NATHEJK.>` and publishes successfully — while quietly no
// longer matching the per-glimt patterns, which would make that glimt impossible to hide or delete.
func validSubjectToken(s, what string) error {
	if s == "" {
		return fmt.Errorf("%s is empty", what)
	}
	if strings.ContainsAny(s, ". \t\r\n*>") {
		return fmt.Errorf("%s %q is not a valid subject token", what, s)
	}
	return nil
}

// Subject builds the subject for one glimt and one verb:
//
//	NATHEJK.<year>.glimt.<glimtId>.<verb>
//
// One constructor for every verb rather than six near-identical ones, because the verbs are a
// closed set used by handlers that already know which they want, and six functions would be six
// places to get the token validation wrong.
func Subject(year, glimtID, verb string) (cqrs.Subject, error) {
	if err := validSubjectToken(year, "year"); err != nil {
		return nil, err
	}
	if err := validSubjectToken(glimtID, "glimt id"); err != nil {
		return nil, err
	}
	if err := validSubjectToken(verb, "verb"); err != nil {
		return nil, err
	}
	return cqrs.SubjectFromStr(
		fmt.Sprintf("NATHEJK.%s.glimt.%s.%s", year, glimtID, verb)), nil
}

// The verbs Subject accepts. Constants so a handler cannot publish "hide" where the projection
// listens for "hidden" — a typo that would fail silently, since an unmatched subject is simply
// never delivered to anything.
const (
	VerbCreated  = "created"
	VerbDeleted  = "deleted"
	VerbReported = "reported"
	VerbHidden   = "hidden"
	VerbUnhidden = "unhidden"
	VerbPurged   = "purged"
)

var _ cqrs.Consumer = consumer{}
