package photo

import (
	"fmt"
	"strings"
	"time"

	"github.com/jrgensen/cqrs"
)

// consumer folds the photograph events into the read model.
//
// Every fold is idempotent, because these tables are rebuilt from sequence zero on every boot: uploads
// are upserts, updates apply only the fields they carry, and a tag is keyed by (photo, team) so a replay
// cannot duplicate one. As in the glimt and album projections, "idempotent" is not a nice property here
// — it is the only way the rows survive a restart.
type consumer struct {
	w cqrs.Writer
}

// Consumes lists the subjects this projection subscribes to.
//
// Dot form after NATHEJK, matching what this app already publishes for `portrait`, `glimt` and `album`.
// Several upstream producers use a colon, but those are their events and copying the quirk into a new
// entity would spread it.
//
// Every verb is subscribed here, including the ones whose publishing handlers arrive in later tasks
// (the tags in 367, the deletion in 379). Subscribing early is free and the alternative is worse: a
// projection that ignores an event it was not yet taught about would silently keep showing a photograph
// somebody took down, and a consumer's subject list is exactly the thing nobody remembers to extend.
// This is the rule the album consumer established for its own removal verbs.
func (c consumer) Consumes() []cqrs.Subject {
	return []cqrs.Subject{
		cqrs.SubjectFromStr("NATHEJK.*.photo.*.uploaded"),
		cqrs.SubjectFromStr("NATHEJK.*.photo.*.updated"),
		cqrs.SubjectFromStr("NATHEJK.*.photo.*.locationcleared"),
		cqrs.SubjectFromStr("NATHEJK.*.photo.*.patroltagged"),
		cqrs.SubjectFromStr("NATHEJK.*.photo.*.patroluntagged"),
		cqrs.SubjectFromStr("NATHEJK.*.photo.*.deleted"),
	}
}

// HandleMessage folds one event into the read model.
//
// Errors are annotated with the subject for the reason the person, checkpoint, glimt and album packages
// all record: the stream library logs a handler error and *drops* the message rather than dead-lettering
// it, so the log line is the only trace it existed, and a bare decode error is unattributable among tens
// of thousands of messages.
func (c consumer) HandleMessage(msg cqrs.Message) error {
	subject := msg.Subject()
	if err := c.handleMessage(msg, subject); err != nil {
		return fmt.Errorf("photo: %s: %w", subject.Subject(), err)
	}
	return nil
}

func (c consumer) handleMessage(msg cqrs.Message, subject cqrs.Subject) error {
	year := subjectYear(subject)
	if year == "" {
		return fmt.Errorf("no year in subject")
	}

	switch {
	case subject.Match("nathejk.*.photo.*.uploaded"):
		return c.handleUploaded(msg, year)
	case subject.Match("nathejk.*.photo.*.updated"):
		return c.handleUpdated(msg, year)
	case subject.Match("nathejk.*.photo.*.locationcleared"):
		return c.handleLocationCleared(msg, year)
	case subject.Match("nathejk.*.photo.*.patroltagged"):
		return c.handlePatrolTagged(msg, year)
	case subject.Match("nathejk.*.photo.*.patroluntagged"):
		return c.handlePatrolUntagged(msg, year)
	case subject.Match("nathejk.*.photo.*.deleted"):
		return c.handleDeleted(msg, year)
	}
	return nil
}

// handleUploaded writes the photograph row.
//
// Note what the update clause does **not** touch: `deleted`, and `caption`.
//
// `deleted` is left alone for the reason recorded at length in table.sql and on the Uploaded event.
// Because the id is the content hash, a re-upload of the same file republishes *this* event — so
// clearing the flag here would make a deletion reversible by a photographer re-dragging a folder, which
// is the one way this projection could quietly undo a takedown. This is album's create-fold rule, and it
// matters more here.
//
// `caption` is left alone because a re-upload must not discard a curator's words. The upload event
// carries no caption at all (there is nothing to caption a file with at upload time), so including the
// column in the clause would mean writing an empty string over an evening's editing. The insert lists it
// so the NOT NULL column has a value on first arrival.
func (c consumer) handleUploaded(msg cqrs.Message, year string) error {
	var body Uploaded
	if err := msg.Body(&body); err != nil {
		return err
	}

	photoID := body.PhotoID
	if photoID == "" {
		photoID = subjectEntityID(msg.Subject())
	}
	if photoID == "" {
		return fmt.Errorf("photo uploaded with no photoId")
	}
	if !validRef(body.Ref) {
		// A photograph with no usable content hash has nothing to show, and the ref is the one string
		// here that could otherwise become a filesystem path.
		return fmt.Errorf("photo uploaded with an invalid ref")
	}
	// A malformed thumbnail ref costs the thumbnail, not the photograph — readers fall back to the full
	// image. The same rule the glimt, portrait and album folds apply.
	thumbRef := body.ThumbRef
	if thumbRef != "" && !validRef(thumbRef) {
		thumbRef = ""
	}

	uploadedAt := body.UploadedAt
	if uploadedAt.IsZero() {
		// No replay-stable fallback exists: time.Now() would differ on every rebuild, and this column
		// orders the entire contact sheet. The same refusal the glimt and album folds make.
		return fmt.Errorf("photo uploaded with no uploadedAt")
	}

	lat, lng, verdict := locationColumns(body.Location)

	return c.w.Consume(fmt.Sprintf(
		"INSERT INTO photo SET photoId=%s, year=%s, blobRef=%s, thumbRef=%s, caption=\"\", "+
			"width=%d, height=%d, bytes=%d, latitude=%s, longitude=%s, boundsVerdict=%s, "+
			"uploadedAt=%s "+
			"ON DUPLICATE KEY UPDATE "+
			"year=VALUES(year), blobRef=VALUES(blobRef), thumbRef=VALUES(thumbRef), "+
			"width=VALUES(width), height=VALUES(height), bytes=VALUES(bytes), "+
			"latitude=VALUES(latitude), longitude=VALUES(longitude), "+
			"boundsVerdict=VALUES(boundsVerdict), uploadedAt=VALUES(uploadedAt)",
		quote(photoID), quote(year), quote(body.Ref), quote(thumbRef),
		body.Width, body.Height, body.Bytes, lat, lng, quote(verdict),
		quote(formatTime(uploadedAt)),
	))
}

// handleUpdated applies a delta.
//
// Only the fields the event carries are written, which is what the pointer shape on `Updated` is for:
// "not mentioned" and "set to empty" are different messages. That matters more here than anywhere else
// in this service, because this is the event a **bulk** action publishes — setting a location on forty
// selected photographs must not blank forty captions.
//
// An update carrying nothing is not an error. It is a no-op, and refusing it would turn a harmless
// client bug into a dropped message in a log nobody reads.
func (c consumer) handleUpdated(msg cqrs.Message, year string) error {
	var body Updated
	if err := msg.Body(&body); err != nil {
		return err
	}

	photoID := body.PhotoID
	if photoID == "" {
		photoID = subjectEntityID(msg.Subject())
	}
	if photoID == "" {
		return fmt.Errorf("photo updated with no photoId")
	}

	var sets []string
	if body.Caption != nil {
		sets = append(sets, "caption="+quote(*body.Caption))
	}
	if body.Location != nil {
		// The coordinate and its verdict are written together, always. The Location type exists to make
		// writing one without the other unsayable — see its doc comment.
		lat, lng, verdict := locationColumns(body.Location)
		sets = append(sets,
			"latitude="+lat,
			"longitude="+lng,
			"boundsVerdict="+quote(verdict),
		)
	}
	if len(sets) == 0 {
		return nil
	}

	return c.w.Consume(fmt.Sprintf(
		"UPDATE photo SET %s WHERE photoId=%s AND year=%s",
		strings.Join(sets, ", "), quote(photoID), quote(year),
	))
}

// handleLocationCleared removes the coordinate and resets the verdict.
//
// The verdict goes back to `none` along with the NULLs, because a verdict about a coordinate that no
// longer exists would claim a judgement about nothing — and, concretely, leaving `inside` behind would
// leave the row matching the map read's filter with nothing to plot.
func (c consumer) handleLocationCleared(msg cqrs.Message, year string) error {
	var body LocationCleared
	if err := msg.Body(&body); err != nil {
		return err
	}

	photoID := body.PhotoID
	if photoID == "" {
		photoID = subjectEntityID(msg.Subject())
	}
	if photoID == "" {
		return fmt.Errorf("photo location cleared with no photoId")
	}

	return c.w.Consume(fmt.Sprintf(
		"UPDATE photo SET latitude=NULL, longitude=NULL, boundsVerdict=%s "+
			"WHERE photoId=%s AND year=%s",
		quote(BoundsNone), quote(photoID), quote(year),
	))
}

// handlePatrolTagged records an attribution.
//
// Keyed `(year, photoId, teamId)` and upserted, so a replay converges and tagging an already-tagged
// photograph is a no-op rather than a duplicate — which is what makes the bulk tag action safe to
// re-run over a selection that partly overlaps what is already tagged.
//
// `deleted=0` is set on insert **and** on update, unlike the photograph's own flag: tagging is a
// deliberate act that supersedes an earlier untag, whereas an upload is not a statement about takedown
// history. This is the distinction album's item fold draws for the same reason.
func (c consumer) handlePatrolTagged(msg cqrs.Message, year string) error {
	var body PatrolTagged
	if err := msg.Body(&body); err != nil {
		return err
	}

	photoID := body.PhotoID
	if photoID == "" {
		photoID = subjectEntityID(msg.Subject())
	}
	if photoID == "" {
		return fmt.Errorf("photo tagged with no photoId")
	}
	if body.TeamID == "" {
		// The id is the identity of the tag; the number is only what it was resolved from. A tag with
		// no id is not a weaker tag, it is a row that can never be matched, untagged or joined.
		return fmt.Errorf("photo tagged with no teamId")
	}

	taggedAt := body.TaggedAt
	if taggedAt.IsZero() {
		return fmt.Errorf("photo tagged with no taggedAt")
	}

	return c.w.Consume(fmt.Sprintf(
		"INSERT INTO photo_patrol SET year=%s, photoId=%s, teamId=%s, teamNumber=%s, "+
			"taggedAt=%s, deleted=0 "+
			"ON DUPLICATE KEY UPDATE "+
			"teamNumber=VALUES(teamNumber), taggedAt=VALUES(taggedAt), deleted=0",
		quote(year), quote(photoID), quote(body.TeamID), quote(body.Number),
		quote(formatTime(taggedAt)),
	))
}

// handlePatrolUntagged marks one attribution removed.
//
// A soft delete, matching every other removal in this projection: the row survives so a re-tag is
// expressible and an accidental untag is recoverable from the log.
func (c consumer) handlePatrolUntagged(msg cqrs.Message, year string) error {
	var body PatrolUntagged
	if err := msg.Body(&body); err != nil {
		return err
	}

	photoID := body.PhotoID
	if photoID == "" {
		photoID = subjectEntityID(msg.Subject())
	}
	if photoID == "" {
		return fmt.Errorf("photo untagged with no photoId")
	}
	if body.TeamID == "" {
		return fmt.Errorf("photo untagged with no teamId")
	}

	return c.w.Consume(fmt.Sprintf(
		"UPDATE photo_patrol SET deleted=1 WHERE year=%s AND photoId=%s AND teamId=%s",
		quote(year), quote(photoID), quote(body.TeamID),
	))
}

// handleDeleted marks a photograph removed.
//
// # Why this does not touch album_item
//
// The album fold marks its items deleted when a whole album goes, because the public map read looks at
// items across every album and has no reason to join the parent. The mirror of that argument does *not*
// apply here, and the difference is worth stating because it looks like an omission.
//
// After PRD 022 §8.3 every read that resolves a photograph's bytes, caption or coordinate goes through
// this table — `album_item` holds a `photoId` and nothing else worth showing. So a deleted photograph
// disappears from every album by virtue of the join, and there is nothing a stale membership row can
// leak. Marking them would also destroy information: which albums the photograph was in is exactly what
// an undelete would need (PRD 022 §11 Q6).
//
// The requirement this rests on is that **every join to `album_item` filters `photo.deleted = 0`**. It
// is enforced by the querier rather than by convention, and by the test that asserts no deleted
// photograph is reachable on a public surface (task 382).
func (c consumer) handleDeleted(msg cqrs.Message, year string) error {
	var body Deleted
	if err := msg.Body(&body); err != nil {
		return err
	}

	photoID := body.PhotoID
	if photoID == "" {
		photoID = subjectEntityID(msg.Subject())
	}
	if photoID == "" {
		return fmt.Errorf("photo deleted with no photoId")
	}

	return c.w.Consume(fmt.Sprintf(
		"UPDATE photo SET deleted=1 WHERE photoId=%s AND year=%s",
		quote(photoID), quote(year),
	))
}

// locationColumns renders a Location as the three SQL values that describe a position.
//
// One function so that every write of a coordinate agrees, and so the two rules below hold everywhere
// rather than in whichever fold remembered them:
//
//  1. **An unrecognised verdict becomes `unknown`, never `inside`.** The map read filters on this
//     column, so the failure direction matters: a typo must make a photograph unplottable rather than
//     plot one whose coordinate was never checked.
//  2. **No coordinate means the verdict is `none`.** A verdict about a value nothing stored would claim
//     a judgement about nothing, and would leave the row matching a filter with nothing to draw.
func locationColumns(loc *Location) (lat, lng, verdict string) {
	if loc == nil {
		return "NULL", "NULL", BoundsNone
	}

	verdict = loc.BoundsVerdict
	switch verdict {
	case BoundsNone, BoundsInside, BoundsOutside, BoundsUnknown:
	default:
		verdict = BoundsUnknown
	}
	// A verdict of `none` beside a real coordinate is contradictory, and the coordinate is the more
	// trustworthy half: something placed it. Judged unknown rather than silently plotted.
	if verdict == BoundsNone {
		verdict = BoundsUnknown
	}

	return fmt.Sprintf("%f", loc.Lat), fmt.Sprintf("%f", loc.Lng), verdict
}

// subjectYear extracts the year, which is the second token.
func subjectYear(s cqrs.Subject) string {
	parts := s.Parts()
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// subjectEntityID extracts the photo id, which is the part before the verb.
func subjectEntityID(s cqrs.Subject) string {
	parts := s.Parts()
	if len(parts) < 4 {
		return ""
	}
	return parts[3]
}

// validRef reports whether ref looks like a content hash from the blob store.
//
// Duplicates blob.Ref.Valid rather than calling it, for the reason person/portrait.go and album record:
// this package may not import internal/... . The check is 64 lowercase hex characters — a sha256 in hex
// — and it matters because a ref arrives in an event body and ends up in a SQL statement and later in a
// URL path. "../../etc/passwd" is a ref-shaped string.
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
// Always UTC: the column has no zone, so writing local time would make the ordering depend on the
// server's clock configuration — and this column is what orders the contact sheet.
func formatTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05")
}

// quote renders a Go string as a SQL string literal.
//
// Same reasoning as the person, checkpoint, glimt, album and maphandout packages: cqrs.Writer takes a
// finished statement rather than a statement plus arguments, so escaping is this file's responsibility.
func quote(s string) string { return fmt.Sprintf("%q", s) }

// validSubjectToken rejects anything that would not survive as a single NATS subject token.
//
// Not cosmetic, and the reason is the one person/portrait.go gives: an id containing a dot splits into
// extra tokens, still matches `NATHEJK.>` and publishes successfully — while quietly no longer matching
// the per-photo patterns, which would make that photograph impossible to edit or take down.
//
// A content-hash id cannot contain a dot, so for the ids this projection mints the check can never fire.
// It stays because the year and the verb pass through here too, and because "the id is always a hash" is
// a property of today's publisher rather than of this function's contract.
func validSubjectToken(s, what string) error {
	if s == "" {
		return fmt.Errorf("%s is empty", what)
	}
	if strings.ContainsAny(s, ". \t\r\n*>") {
		return fmt.Errorf("%s %q is not a valid subject token", what, s)
	}
	return nil
}

// Subject builds the subject for one photograph and one verb:
//
//	NATHEJK.<year>.photo.<photoId>.<verb>
//
// One constructor for every verb rather than six near-identical ones, following the glimt and album
// packages: the verbs are a closed set used by handlers that already know which they want, and six
// functions would be six places to get the token validation wrong.
func Subject(year, photoID, verb string) (cqrs.Subject, error) {
	if err := validSubjectToken(year, "year"); err != nil {
		return nil, err
	}
	if err := validSubjectToken(photoID, "photo id"); err != nil {
		return nil, err
	}
	if err := validSubjectToken(verb, "verb"); err != nil {
		return nil, err
	}
	return cqrs.SubjectFromStr(
		fmt.Sprintf("NATHEJK.%s.photo.%s.%s", year, photoID, verb)), nil
}

// The verbs Subject accepts.
//
// Constants so a handler cannot publish "untag" where the projection listens for "patroluntagged" — a
// typo that would fail silently, since an unmatched subject is simply never delivered to anything.
//
// One word, no hyphen: a subject token may not contain a dot, and while a hyphen is legal it reads badly
// next to the other entities' single-word verbs.
const (
	VerbUploaded        = "uploaded"
	VerbUpdated         = "updated"
	VerbLocationCleared = "locationcleared"
	VerbPatrolTagged    = "patroltagged"
	VerbPatrolUntagged  = "patroluntagged"
	VerbDeleted         = "deleted"
)

var _ cqrs.Consumer = consumer{}
