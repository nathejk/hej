package album

import (
	"fmt"
	"strings"
	"time"

	"github.com/jrgensen/cqrs"
)

// consumer folds the album events into the read model.
//
// Every fold is idempotent, because these tables are rebuilt from sequence zero on every boot:
// creates are upserts, updates apply only the fields they carry, and an item is keyed by (album,
// ordinal) so a replay cannot duplicate one. As in the glimt projection, "idempotent" is not a nice
// property here — it is the only way the rows survive a restart.
type consumer struct {
	w cqrs.Writer
}

// Consumes lists the subjects this projection subscribes to.
//
// Dot form after NATHEJK, matching what this app already publishes for `portrait` and `glimt`. Several
// upstream producers use a colon, but those are their events and copying the quirk into a new entity
// would spread it.
//
// The removal verbs (`item-removed`, `deleted`) are subscribed here and folded below even though the
// handlers that publish them arrive in task 335. Subscribing early is free and the alternative is
// worse: a projection that ignores an event it was not yet taught about would silently keep showing a
// photograph somebody took down, and a consumer's subject list is exactly the thing nobody remembers
// to extend.
func (c consumer) Consumes() []cqrs.Subject {
	return []cqrs.Subject{
		cqrs.SubjectFromStr("NATHEJK.*.album.*.created"),
		cqrs.SubjectFromStr("NATHEJK.*.album.*.updated"),
		cqrs.SubjectFromStr("NATHEJK.*.album.*.itemadded"),
		cqrs.SubjectFromStr("NATHEJK.*.album.*.itemremoved"),
		cqrs.SubjectFromStr("NATHEJK.*.album.*.deleted"),
	}
}

// HandleMessage folds one event into the read model.
//
// Errors are annotated with the subject for the reason the person, checkpoint and glimt packages all
// record: the stream library logs a handler error and *drops* the message rather than dead-lettering
// it, so the log line is the only trace it existed, and a bare decode error is unattributable among
// tens of thousands of messages.
func (c consumer) HandleMessage(msg cqrs.Message) error {
	subject := msg.Subject()
	if err := c.handleMessage(msg, subject); err != nil {
		return fmt.Errorf("album: %s: %w", subject.Subject(), err)
	}
	return nil
}

func (c consumer) handleMessage(msg cqrs.Message, subject cqrs.Subject) error {
	year := subjectYear(subject)
	if year == "" {
		return fmt.Errorf("no year in subject")
	}

	switch {
	case subject.Match("nathejk.*.album.*.created"):
		return c.handleCreated(msg, year)
	case subject.Match("nathejk.*.album.*.updated"):
		return c.handleUpdated(msg, year)
	case subject.Match("nathejk.*.album.*.itemadded"):
		return c.handleItemAdded(msg, year)
	case subject.Match("nathejk.*.album.*.itemremoved"):
		return c.handleItemRemoved(msg, year)
	case subject.Match("nathejk.*.album.*.deleted"):
		return c.handleDeleted(msg, year)
	}
	return nil
}

// handleCreated writes the album row.
//
// Note what the update clause does **not** touch: `published` and `deleted`. Both come from later
// events, and a replay applies this one first — so setting them here would be correct on the way
// through while a re-delivery of an old create after a takedown would silently republish a deleted
// album. Left out of the clause entirely rather than reasoned about per replay, which is the rule the
// glimt fold established for `hiddenAt`.
func (c consumer) handleCreated(msg cqrs.Message, year string) error {
	var body Created
	if err := msg.Body(&body); err != nil {
		return err
	}

	albumID := body.AlbumID
	if albumID == "" {
		albumID = subjectEntityID(msg.Subject())
	}
	if albumID == "" {
		return fmt.Errorf("album created with no albumId")
	}

	slug := strings.TrimSpace(body.Slug)
	if !validSlug(slug) {
		// Refused rather than stored: the slug is the album's public address, and one containing a
		// slash or a space is a URL that cannot be reached — an album nobody can open, with nothing
		// to indicate why.
		return fmt.Errorf("album created with an unusable slug %q", body.Slug)
	}

	createdAt := body.CreatedAt
	if createdAt.IsZero() {
		// No replay-stable fallback exists: time.Now() would differ on every rebuild. The same
		// refusal the glimt fold makes, and for the same reason.
		return fmt.Errorf("album created with no createdAt")
	}

	return c.w.Consume(fmt.Sprintf(
		"INSERT INTO album SET albumId=%s, year=%s, slug=%s, title=%s, description=%s, "+
			"sortOrder=%d, createdAt=%s "+
			"ON DUPLICATE KEY UPDATE "+
			"year=VALUES(year), slug=VALUES(slug), title=VALUES(title), "+
			"description=VALUES(description), sortOrder=VALUES(sortOrder), "+
			"createdAt=VALUES(createdAt)",
		quote(albumID), quote(year), quote(slug), quote(body.Title), quote(body.Description),
		body.SortOrder, quote(formatTime(createdAt)),
	))
}

// handleUpdated applies a delta.
//
// Only the fields the event carries are written, which is what the pointer shape on `Updated` is for:
// "not mentioned" and "set to empty" are different messages, and conflating them means a curator
// renaming an album wipes its description.
//
// An update carrying nothing is not an error. It is a no-op, and refusing it would turn a harmless
// client bug into a dropped message in a log nobody reads.
func (c consumer) handleUpdated(msg cqrs.Message, year string) error {
	var body Updated
	if err := msg.Body(&body); err != nil {
		return err
	}

	albumID := body.AlbumID
	if albumID == "" {
		albumID = subjectEntityID(msg.Subject())
	}
	if albumID == "" {
		return fmt.Errorf("album updated with no albumId")
	}

	var sets []string
	if body.Title != nil {
		sets = append(sets, "title="+quote(*body.Title))
	}
	if body.Description != nil {
		sets = append(sets, "description="+quote(*body.Description))
	}
	if body.SortOrder != nil {
		sets = append(sets, fmt.Sprintf("sortOrder=%d", *body.SortOrder))
	}
	if body.Published != nil {
		sets = append(sets, fmt.Sprintf("published=%d", boolToInt(*body.Published)))
	}
	if len(sets) == 0 {
		return nil
	}

	return c.w.Consume(fmt.Sprintf(
		"UPDATE album SET %s WHERE albumId=%s AND year=%s",
		strings.Join(sets, ", "), quote(albumID), quote(year),
	))
}

// handleItemAdded writes one photograph.
//
// Keyed by (albumId, ordinal) and upserted, so a replay converges rather than accumulating — and so a
// curator replacing the photograph at position 3 is one event rather than a remove and an add that
// could arrive out of order.
//
// `deleted=0` is set on insert **and** on update here, unlike the parent's `deleted`. That is not an
// inconsistency: adding an item at an ordinal is a deliberate act that supersedes whatever was there,
// including a removal, whereas an album's create event is not a statement about its takedown history.
func (c consumer) handleItemAdded(msg cqrs.Message, year string) error {
	var body ItemAdded
	if err := msg.Body(&body); err != nil {
		return err
	}

	albumID := body.AlbumID
	if albumID == "" {
		albumID = subjectEntityID(msg.Subject())
	}
	if albumID == "" {
		return fmt.Errorf("album item added with no albumId")
	}
	if !validRef(body.Ref) {
		// An item is a photograph; with no usable content hash there is nothing to show, and the ref
		// is the one string here that could otherwise become a filesystem path.
		return fmt.Errorf("album item added with an invalid ref")
	}
	// A malformed thumbnail ref costs the thumbnail, not the item — the page falls back to the full
	// image. The same rule the glimt and portrait folds apply.
	thumbRef := body.ThumbRef
	if thumbRef != "" && !validRef(thumbRef) {
		thumbRef = ""
	}

	addedAt := body.AddedAt
	if addedAt.IsZero() {
		return fmt.Errorf("album item added with no addedAt")
	}

	verdict := body.BoundsVerdict
	switch verdict {
	case BoundsNone, BoundsInside, BoundsOutside, BoundsUnknown:
	default:
		// An unrecognised verdict becomes `unknown`, never `inside`. The map read filters on this
		// column, so the failure direction matters: a typo must make a photograph unplottable rather
		// than plot one whose coordinate was never checked.
		verdict = BoundsUnknown
	}

	// A coordinate is only stored when it is actually usable. Without both halves there is nothing to
	// plot and nothing for a curator to correct, so a half-coordinate is dropped rather than written
	// as a NULL beside a real number — which would read as "we have the latitude", and we do not.
	lat, lng := "NULL", "NULL"
	if body.Lat != nil && body.Lng != nil {
		lat = fmt.Sprintf("%f", *body.Lat)
		lng = fmt.Sprintf("%f", *body.Lng)
	} else if verdict != BoundsNone {
		// A verdict about a coordinate that did not arrive. Recorded as `none`, because the verdict
		// would otherwise claim a judgement about a value nothing stored.
		verdict = BoundsNone
	}

	return c.w.Consume(fmt.Sprintf(
		"INSERT INTO album_item SET albumId=%s, year=%s, ordinal=%d, blobRef=%s, thumbRef=%s, "+
			"caption=%s, width=%d, height=%d, bytes=%d, latitude=%s, longitude=%s, "+
			"boundsVerdict=%s, addedAt=%s, deleted=0 "+
			"ON DUPLICATE KEY UPDATE "+
			"year=VALUES(year), blobRef=VALUES(blobRef), thumbRef=VALUES(thumbRef), "+
			"caption=VALUES(caption), width=VALUES(width), height=VALUES(height), "+
			"bytes=VALUES(bytes), latitude=VALUES(latitude), longitude=VALUES(longitude), "+
			"boundsVerdict=VALUES(boundsVerdict), addedAt=VALUES(addedAt), deleted=0",
		quote(albumID), quote(year), body.Ordinal, quote(body.Ref), quote(thumbRef),
		quote(body.Caption), body.Width, body.Height, body.Bytes, lat, lng,
		quote(verdict), quote(formatTime(addedAt)),
	))
}

// ItemRemoved takes one photograph out of an album (task 335).
//
// The type lives here rather than in events.go with the others because the handler that publishes it
// is task 335's; the fold is here so the projection cannot silently ignore the event in the meantime.
type ItemRemoved struct {
	AlbumID   string    `json:"albumId"`
	Year      string    `json:"year"`
	Ordinal   int       `json:"ordinal"`
	Reason    string    `json:"reason,omitempty"`
	RemovedAt time.Time `json:"removedAt"`
}

// Deleted takes a whole album down (task 335).
type Deleted struct {
	AlbumID   string    `json:"albumId"`
	Year      string    `json:"year"`
	Reason    string    `json:"reason,omitempty"`
	DeletedAt time.Time `json:"deletedAt"`
}

// handleItemRemoved marks one item removed.
//
// A soft delete, so the row survives. The point is not squeamishness about DELETE: a removal here
// honours somebody's objection, and an accidental one must be recoverable without republishing a
// photograph that was taken down on purpose. A destructive delete would make "put it back" require
// re-uploading bytes we deliberately destroyed.
func (c consumer) handleItemRemoved(msg cqrs.Message, year string) error {
	var body ItemRemoved
	if err := msg.Body(&body); err != nil {
		return err
	}
	albumID := body.AlbumID
	if albumID == "" {
		albumID = subjectEntityID(msg.Subject())
	}
	if albumID == "" {
		return fmt.Errorf("album item removed with no albumId")
	}
	return c.w.Consume(fmt.Sprintf(
		"UPDATE album_item SET deleted=1 WHERE albumId=%s AND year=%s AND ordinal=%d",
		quote(albumID), quote(year), body.Ordinal,
	))
}

// handleDeleted marks a whole album, and every item in it, removed.
//
// The items are marked too rather than left to the album's flag hiding them. Two reads would otherwise
// disagree: the map read looks at items across every album and has no reason to join the parent, so an
// album deleted without its items would keep its photographs on the map.
func (c consumer) handleDeleted(msg cqrs.Message, year string) error {
	var body Deleted
	if err := msg.Body(&body); err != nil {
		return err
	}
	albumID := body.AlbumID
	if albumID == "" {
		albumID = subjectEntityID(msg.Subject())
	}
	if albumID == "" {
		return fmt.Errorf("album deleted with no albumId")
	}
	if err := c.w.Consume(fmt.Sprintf(
		"UPDATE album SET deleted=1, published=0 WHERE albumId=%s AND year=%s",
		quote(albumID), quote(year),
	)); err != nil {
		return err
	}
	return c.w.Consume(fmt.Sprintf(
		"UPDATE album_item SET deleted=1 WHERE albumId=%s AND year=%s",
		quote(albumID), quote(year),
	))
}

// validSlug reports whether s is usable as a single URL path segment.
//
// Lowercase letters, digits and hyphens. Narrow on purpose: this string is concatenated into a public
// URL and matched back out of one, so anything needing escaping in either direction is refused rather
// than encoded. A curator wanting "Lørdag morgen" gets `loerdag-morgen`, which is what they would have
// typed anyway.
func validSlug(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-':
		default:
			return false
		}
	}
	// A leading or trailing hyphen is legal in a URL and reads as a mistake, and `--` usually means a
	// slugifier ran over punctuation it did not expect.
	return !strings.HasPrefix(s, "-") && !strings.HasSuffix(s, "-") && !strings.Contains(s, "--")
}

// subjectYear extracts the year, which is the second token.
func subjectYear(s cqrs.Subject) string {
	parts := s.Parts()
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// subjectEntityID extracts the album id, which is the part before the verb.
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
// package may not import internal/... . The check is 64 lowercase hex characters — a sha256 in hex —
// and it matters because a ref arrives in an event body and ends up in a SQL statement and later in a
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
// server's clock configuration.
func formatTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05")
}

// quote renders a Go string as a SQL string literal.
//
// Same reasoning as the person, checkpoint, glimt and maphandout packages: cqrs.Writer takes a
// finished statement rather than a statement plus arguments, so escaping is this file's
// responsibility.
func quote(s string) string { return fmt.Sprintf("%q", s) }

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// validSubjectToken rejects anything that would not survive as a single NATS subject token.
//
// Not cosmetic, and the reason is the one person/portrait.go gives: an id containing a dot splits into
// extra tokens, still matches `NATHEJK.>` and publishes successfully — while quietly no longer matching
// the per-album patterns, which would make that album impossible to update or take down.
func validSubjectToken(s, what string) error {
	if s == "" {
		return fmt.Errorf("%s is empty", what)
	}
	if strings.ContainsAny(s, ". \t\r\n*>") {
		return fmt.Errorf("%s %q is not a valid subject token", what, s)
	}
	return nil
}

// Subject builds the subject for one album and one verb:
//
//	NATHEJK.<year>.album.<albumId>.<verb>
//
// One constructor for every verb rather than five near-identical ones, following the glimt package:
// the verbs are a closed set used by handlers that already know which they want, and five functions
// would be five places to get the token validation wrong.
func Subject(year, albumID, verb string) (cqrs.Subject, error) {
	if err := validSubjectToken(year, "year"); err != nil {
		return nil, err
	}
	if err := validSubjectToken(albumID, "album id"); err != nil {
		return nil, err
	}
	if err := validSubjectToken(verb, "verb"); err != nil {
		return nil, err
	}
	return cqrs.SubjectFromStr(
		fmt.Sprintf("NATHEJK.%s.album.%s.%s", year, albumID, verb)), nil
}

// The verbs Subject accepts.
//
// Constants so a handler cannot publish "remove" where the projection listens for "itemremoved" — a
// typo that would fail silently, since an unmatched subject is simply never delivered to anything.
//
// One word, no hyphen: a subject token may not contain a dot, and while a hyphen is legal it reads
// badly next to the other entities' single-word verbs.
const (
	VerbCreated     = "created"
	VerbUpdated     = "updated"
	VerbItemAdded   = "itemadded"
	VerbItemRemoved = "itemremoved"
	VerbDeleted     = "deleted"
)

var _ cqrs.Consumer = consumer{}
