package kort

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jrgensen/cqrs"
	"github.com/nathejk/shared-go/types"
)

// consumer folds the kort and kortsaet events into the read model.
type consumer struct {
	w cqrs.Writer

	// unknownBody reports a body we could not make sense of.
	//
	// Not a returned error: the stream library logs a handler error and *drops* the message, and
	// a sheet lost that way is a sheet whose checkpoints never appear. But an unreadable body is
	// also our only signal that the vendored contract has gone stale (PRD 016 §11.11), so it must
	// not be swallowed either. Hence a sink the application can wire to its logger.
	unknownBody func(subject string, err error)
}

// Consumes lists the subjects this projection subscribes to.
//
// Both entity families and both collection-level sort subjects. Note the `.sorted` subjects carry
// no entity id — they are about the collection — so they are matched separately from the
// `{id}.{verb}` shape below.
func (c consumer) Consumes() []cqrs.Subject {
	return []cqrs.Subject{
		cqrs.SubjectFromStr("NATHEJK.*.kort.*.created"),
		cqrs.SubjectFromStr("NATHEJK.*.kort.*.updated"),
		cqrs.SubjectFromStr("NATHEJK.*.kort.*.deleted"),
		cqrs.SubjectFromStr("NATHEJK.*.kort.sorted"),
		cqrs.SubjectFromStr("NATHEJK.*.kortsaet.*.created"),
		cqrs.SubjectFromStr("NATHEJK.*.kortsaet.*.updated"),
		cqrs.SubjectFromStr("NATHEJK.*.kortsaet.*.deleted"),
		cqrs.SubjectFromStr("NATHEJK.*.kortsaet.sorted"),
	}
}

// HandleMessage folds one event into the read model.
//
// Errors are annotated with the subject for the reason recorded in the person and checkpoint
// packages: the stream library logs a handler error and drops the message rather than
// dead-lettering it, so the log line is the only trace it existed, and a bare decode error is
// unattributable among tens of thousands of messages.
func (c consumer) HandleMessage(msg cqrs.Message) error {
	subject := msg.Subject()
	if err := c.handleMessage(msg, subject); err != nil {
		return fmt.Errorf("kort: %s: %w", subject.Subject(), err)
	}
	return nil
}

func (c consumer) handleMessage(msg cqrs.Message, subject cqrs.Subject) error {
	year := subjectYear(subject)
	if year == "" {
		return fmt.Errorf("no year in subject")
	}

	// Order matters: the `.sorted` subjects must be tested before the `{id}.{verb}` ones, because
	// "kort.sorted" also matches the shape "kort.*.<verb>" with "sorted" read as an id under some
	// matchers. Testing the specific pattern first makes the dispatch independent of that.
	switch {
	case subject.Match("nathejk.*.kort.sorted"):
		return c.handleSheetsSorted(msg, year)
	case subject.Match("nathejk.*.kortsaet.sorted"):
		return c.handleSetsSorted(msg, year)
	case subject.Match("nathejk.*.kort.*.created"):
		return c.handleSheetCreated(msg, year)
	case subject.Match("nathejk.*.kort.*.updated"):
		return c.handleSheetUpdated(msg, year)
	case subject.Match("nathejk.*.kort.*.deleted"):
		return c.handleSheetDeleted(msg, year)
	case subject.Match("nathejk.*.kortsaet.*.created"):
		return c.handleSetCreated(msg, year)
	case subject.Match("nathejk.*.kortsaet.*.updated"):
		return c.handleSetUpdated(msg, year)
	case subject.Match("nathejk.*.kortsaet.*.deleted"):
		return c.handleSetDeleted(msg, year)
	}
	return nil
}

// handleSheetCreated writes the little a create carries.
//
// Only the set and the name, matching how checkpoint.created carries almost nothing: an operator
// adds "Kort 3" and describes it afterwards, so a create that demanded a format and an extent
// would mean a half-known sheet could not be written down at all.
//
// INSERT IGNORE-shaped (via upsert with no updatable columns beyond these): a create replayed after
// the sheet has been described must not undo the description.
func (c consumer) handleSheetCreated(msg cqrs.Message, year string) error {
	var body Created
	if err := c.decode(msg, &body); err != nil {
		return err
	}

	id := string(body.KortID)
	if id == "" {
		id = subjectEntityID(msg.Subject())
	}
	if id == "" {
		return fmt.Errorf("kort created with no kortId")
	}

	// `deleted: 0` because a create after a delete restores the sheet — the last event wins, as in
	// `checkpoint` and `person`.
	return c.w.Consume(upsertKort(map[string]string{
		"id":         quote(id),
		"year":       quote(year),
		"kortsaetId": quote(string(body.KortsaetID)),
		"name":       quote(body.Name),
		"deleted":    "0",
	}))
}

// handleSheetUpdated writes only the columns the event actually carries.
//
// This is the whole shape of this handler, and the reason it is longer than it looks like it should
// be. `Updated` is a **patch**: every field is a pointer and nil means "unchanged", not "cleared".
// Writing all columns unconditionally would let an event that only renames a sheet erase its
// checkpoint list — and since a patrol may see exactly the checkpoints on the sheets it holds, that
// would silently make posts vanish from the map mid-race.
//
// The same trap is documented on this repo's checkpoint consumer, where blanking a position
// silently shrinks the cached map region. Same hazard, same treatment.
func (c consumer) handleSheetUpdated(msg cqrs.Message, year string) error {
	var body Updated
	if err := c.decode(msg, &body); err != nil {
		return err
	}

	id := string(body.KortID)
	if id == "" {
		id = subjectEntityID(msg.Subject())
	}
	if id == "" {
		return fmt.Errorf("kort updated with no kortId")
	}

	cols := map[string]string{
		"id":   quote(id),
		"year": quote(year),
		// An update after a delete restores the sheet.
		"deleted": "0",
	}
	if body.KortsaetID != nil {
		cols["kortsaetId"] = quote(string(*body.KortsaetID))
	}
	if body.Name != nil {
		cols["name"] = quote(*body.Name)
	}
	if body.Format != nil {
		// Stored as sent, even if unrecognised: losing a sheet because a fifth format appeared
		// upstream would be worse than holding a string we do not understand.
		cols["format"] = quote(string(*body.Format))
	}
	if body.Note != nil {
		cols["note"] = quote(*body.Note)
	}
	if body.SortOrder != nil {
		cols["sortOrder"] = fmt.Sprintf("%d", *body.SortOrder)
	}
	if body.HandoutCheckgroupID != nil {
		// Deliberately written even when empty. "" is a value here — the QR rule — and it is how
		// an organizer switches a sheet back from a post, so treating it as absent would make that
		// edit inexpressible and leave the sheet revealing off a post forever.
		cols["handoutCheckgroupId"] = quote(string(*body.HandoutCheckgroupID))
	}
	if body.CheckpointIDs != nil {
		// A pointer to a slice, so an explicitly empty list is a real edit ("this sheet now has no
		// checkpoints") and is written as `[]` rather than skipped.
		encoded, err := encodeJSON(*body.CheckpointIDs)
		if err != nil {
			return fmt.Errorf("encode checkpointIds: %w", err)
		}
		cols["checkpointIds"] = quote(encoded)
	}
	if body.Extents != nil {
		encoded, err := encodeJSON(*body.Extents)
		if err != nil {
			return fmt.Errorf("encode extents: %w", err)
		}
		cols["extents"] = quote(encoded)
	}

	return c.w.Consume(upsertKort(cols))
}

// handleSheetDeleted soft-deletes a sheet.
//
// No INSERT: a delete for a sheet never seen affects zero rows, which is correct and idempotent.
// The sheet's checkpoints are untouched — they exist independently and are almost certainly drawn
// on another sheet too.
func (c consumer) handleSheetDeleted(msg cqrs.Message, year string) error {
	var body Deleted
	// A delete body carrying nothing usable is still a delete: the id is in the subject. So a
	// decode failure here is not fatal, unlike an update whose payload *is* the information.
	_ = c.decode(msg, &body)

	id := string(body.KortID)
	if id == "" {
		id = subjectEntityID(msg.Subject())
	}
	if id == "" {
		return fmt.Errorf("kort delete with no kortId")
	}

	return c.w.Consume(fmt.Sprintf(
		"UPDATE kort SET deleted=1 WHERE id=%s AND year=%s", quote(id), quote(year)))
}

// handleSheetsSorted applies handout order.
//
// Position in the list is the sortOrder. **Ids not named keep their current order**, so this is not
// a full ordering of the year and unnamed sheets must not be renumbered — a consumer that reset
// them would reorder sheets an operator never touched.
//
// One UPDATE per named id rather than a single CASE expression: there are on the order of fifteen
// sheets, the statements are what a replay re-runs, and one statement per id is what a
// dead-lettered log line can be matched back to an event.
func (c consumer) handleSheetsSorted(msg cqrs.Message, year string) error {
	var body Sorted
	if err := c.decode(msg, &body); err != nil {
		return err
	}

	for i, id := range body.KortIDs {
		if id == "" {
			continue
		}
		if err := c.w.Consume(fmt.Sprintf(
			"UPDATE kort SET sortOrder=%d WHERE id=%s AND year=%s",
			i, quote(string(id)), quote(year))); err != nil {
			return err
		}
	}
	return nil
}

// handleSetCreated and handleSetUpdated both write the set's whole record.
//
// The asymmetry with the sheet handlers above is deliberate and must not be "tidied up": a set's
// created/updated carries its **whole editable state**, so an absent teamType means the set has
// none rather than "unchanged", and it is written as NULL. Under patch semantics, clearing a team
// type and not mentioning it would be the same event, and un-marking the patrol set would be
// inexpressible.
//
// Getting this backwards is not a cosmetic bug: the patrol handout list is filtered on exactly this
// column, so a stale team type shows one audience's sheets to another.
func (c consumer) handleSetCreated(msg cqrs.Message, year string) error {
	var body SetCreated
	if err := c.decode(msg, &body); err != nil {
		return err
	}
	id := string(body.KortsaetID)
	if id == "" {
		id = subjectEntityID(msg.Subject())
	}
	if id == "" {
		return fmt.Errorf("kortsaet created with no kortsaetId")
	}
	return c.w.Consume(c.setUpsert(id, year, body.Name, teamTypeLiteral(body.TeamType)))
}

func (c consumer) handleSetUpdated(msg cqrs.Message, year string) error {
	var body SetUpdated
	if err := c.decode(msg, &body); err != nil {
		return err
	}
	id := string(body.KortsaetID)
	if id == "" {
		id = subjectEntityID(msg.Subject())
	}
	if id == "" {
		return fmt.Errorf("kortsaet updated with no kortsaetId")
	}
	return c.w.Consume(c.setUpsert(id, year, body.Name, teamTypeLiteral(body.TeamType)))
}

func (c consumer) setUpsert(id, year, name, teamType string) string {
	return upsertKortsaet(map[string]string{
		"id":       quote(id),
		"year":     quote(year),
		"name":     quote(name),
		"teamType": teamType,
		"deleted":  "0",
	})
}

// handleSetDeleted soft-deletes a set.
//
// Upstream only ever publishes this for a set with no sheets — the command refuses while it still
// holds any — so there is no cascade to apply here. We do not enforce that ourselves: if a delete
// for a non-empty set ever arrives, its sheets keep their rows and simply reference a deleted set,
// which reads as "no set" rather than losing the sheets.
func (c consumer) handleSetDeleted(msg cqrs.Message, year string) error {
	var body SetDeleted
	_ = c.decode(msg, &body)

	id := string(body.KortsaetID)
	if id == "" {
		id = subjectEntityID(msg.Subject())
	}
	if id == "" {
		return fmt.Errorf("kortsaet delete with no kortsaetId")
	}
	return c.w.Consume(fmt.Sprintf(
		"UPDATE kortsaet SET deleted=1 WHERE id=%s AND year=%s", quote(id), quote(year)))
}

func (c consumer) handleSetsSorted(msg cqrs.Message, year string) error {
	var body SetsSorted
	if err := c.decode(msg, &body); err != nil {
		return err
	}
	for i, id := range body.KortsaetIDs {
		if id == "" {
			continue
		}
		if err := c.w.Consume(fmt.Sprintf(
			"UPDATE kortsaet SET sortOrder=%d WHERE id=%s AND year=%s",
			i, quote(string(id)), quote(year))); err != nil {
			return err
		}
	}
	return nil
}

// decode reads a body and reports one we cannot make sense of.
//
// The report is the point (PRD 016 §11.11): these shapes are mirrored from another repo's unstable
// types, and a body that will not decode is our only signal that the mirror has drifted from the
// contract. Unknown *fields* are ignored by design — an additive upstream field must not break a
// consumer mid-event, which is exactly how `mapId` reached qr.registered — so what reaches this
// sink is a genuine shape change, not merely a new key.
func (c consumer) decode(msg cqrs.Message, into any) error {
	if err := msg.Body(into); err != nil {
		if c.unknownBody != nil {
			c.unknownBody(msg.Subject().Subject(), err)
		}
		return err
	}
	return nil
}

// teamTypeLiteral renders a set's team type as a SQL literal.
//
// nil becomes NULL, not "". That is the whole tri-state this column exists to carry: NULL means
// "not for a specific team type" (the ordinary case — the crew set covers gøglere, banditter and
// crew, who are not one team type), and it must be distinguishable from a set marked for one.
func teamTypeLiteral(t *types.TeamType) string {
	if t == nil || *t == "" {
		return "NULL"
	}
	return quote(string(*t))
}

// encodeJSON renders a value as the JSON we store in a TEXT column.
//
// Marshalled rather than re-using the raw bytes from the event, so what we store is what our own
// types mean — a field we do not model cannot ride along into our read model and confuse a later
// reader into thinking we support it.
func encodeJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// subjectYear extracts the year from NATHEJK.<year>.<entity>....
func subjectYear(s cqrs.Subject) string {
	parts := s.Parts()
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// subjectEntityID extracts the id, which is the part before the verb.
//
// The subject is authoritative where it disagrees with the body: that is what the stream routed on.
// Returns "" for the collection-level `.sorted` subjects, which carry no id — callers must not ask.
func subjectEntityID(s cqrs.Subject) string {
	parts := s.Parts()
	if len(parts) < 4 {
		return ""
	}
	return parts[3]
}

// quote renders a Go string as a SQL string literal.
//
// Same reasoning as the checkpoint and person packages: cqrs.Writer takes a finished statement, not
// a statement plus arguments, so escaping is this file's responsibility rather than the driver's —
// and these values come from another service's event bodies, which is not the same as trusted input.
func quote(s string) string { return fmt.Sprintf("%q", s) }

// upsertKort and upsertKortsaet build idempotent INSERT ... ON DUPLICATE KEY UPDATE statements.
//
// Idempotency is not optional: projections are rebuilt by replaying the stream from sequence zero on
// every boot, so every statement runs again on each start.
//
// Only the columns a given event carries are written, which is what makes partial updates safe —
// see handleSheetUpdated.
func upsertKort(cols map[string]string) string     { return upsert("kort", cols, "id", "year") }
func upsertKortsaet(cols map[string]string) string { return upsert("kortsaet", cols, "id", "year") }

func upsert(table string, cols map[string]string, keys ...string) string {
	if len(cols) == 0 {
		return ""
	}

	names := make([]string, 0, len(cols))
	for name := range cols {
		names = append(names, name)
	}
	sortStrings(names)

	isKey := func(n string) bool {
		for _, k := range keys {
			if n == k {
				return true
			}
		}
		return false
	}

	values := make([]string, 0, len(names))
	updates := make([]string, 0, len(names))
	for _, name := range names {
		values = append(values, cols[name])
		if isKey(name) {
			continue
		}
		updates = append(updates, fmt.Sprintf("%s=VALUES(%s)", name, name))
	}

	if len(updates) == 0 {
		return fmt.Sprintf("INSERT IGNORE INTO %s (%s) VALUES (%s)",
			table, strings.Join(names, ", "), strings.Join(values, ", "))
	}
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
		table, strings.Join(names, ", "), strings.Join(values, ", "), strings.Join(updates, ", "))
}

// sortStrings keeps statements deterministic, so a dead-lettered statement can be matched against
// the event that produced it.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

var _ cqrs.Consumer = consumer{}
