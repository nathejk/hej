package person

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/jrgensen/cqrs"
)

// Person is one row of the directory.
//
// Nullable columns are pointers so the caller can tell "not applicable" from
// "empty". That matters most for PhoneParent: only spejder have a guardian number,
// so nil means "this population does not have one" while a pointer to "" would mean
// "should have one and it is missing" — PRD 005's confirmation step and PRD 003's
// profile page render those two differently.
type Person struct {
	PersonID string
	Year     string
	AppRole  string

	Name        string
	Phone       string
	PhoneParent *string

	Address    string
	PostalCode string
	City       string
	Email      string
	Birthday   *time.Time

	TeamID   string
	TeamName string

	// SectionSlug and SectionName are the crew affiliation. Empty for spejder,
	// bandit and gøgler, who belong to a team instead — see TeamName.
	SectionSlug string
	SectionName string

	MemberStatus string
	ArmNumber    string

	VerifiedAt        *time.Time
	AcknowledgedPhone *string
	PortraitRef       string
	// PortraitThumbRef is the default (smallest) thumbnail's content hash, or empty when
	// the portrait predates thumbnails (task 104). Readers fall back to PortraitRef.
	PortraitThumbRef string
	// PortraitThumbs is every thumbnail rendition, each with its own size and byte count.
	// Empty for a portrait captured before the list existed.
	PortraitThumbs []PortraitThumb
	// PortraitCapturedAt is when the current portrait was taken, or nil when there is
	// none (or when the event carried no timestamp). The retention job reads it; see
	// portrait.go.
	PortraitCapturedAt *time.Time
}

// MemberStatusRacing is the one lifecycle value this projection writes.
//
// The string matches `types.MemberStatus`'s `racing` exactly, so a status here means
// what it means in `hq` and in shared-go. It is duplicated rather than imported for
// the usual reason (this package cannot depend on `internal/`, and keeping its imports
// minimal keeps the shared-go lift cheap) — but note the value is a *persisted* one:
// shared-go's own doc warns that changing these strings is a data migration, not a
// rename.
const MemberStatusRacing = "racing"

// IsVerified reports whether this person's guardian number is currently confirmed.
//
// Note what this is *not*: a read of `verifiedAt`. It is `verifiedAt` **and** the
// acknowledged number still matching the guardian number on file. The projector already
// clears `verifiedAt` when the number changes (see invalidateVerification), so in a
// healthy row the extra comparison is redundant — which is the point of doing it here
// as well.
//
// The two guards fail differently. The projector's clear depends on the change arriving
// through handleSpejderUpdated; this one holds regardless of which event path moved the
// number, including one nobody has written yet. The cost of the belt-and-braces is a
// string compare, and the cost of being wrong is telling staff a guardian consented to
// being called on a number they never saw — during an emergency. That trade is not
// close.
//
// A person with no guardian number at all (crew, bandit, gøgler — see HasGuardianPhone)
// can never be verified in this sense, and callers must not read that as "unverified,
// nag them": there is nothing for them to confirm.
func (p Person) IsVerified() bool {
	if p.VerifiedAt == nil {
		return false
	}
	if p.PhoneParent == nil || p.AcknowledgedPhone == nil {
		// Verified at some point, but there is now no number on file, or no record of
		// which number was confirmed. Neither is a state in which a tick can be shown.
		return false
	}
	return *p.AcknowledgedPhone == *p.PhoneParent
}

// HasStarted reports whether the member has begun the event.
//
// PRD 005's confirmation step is skipped for these members: starting implies their
// data was already checked at the counter. Exposed as a method so the rule is written
// once here rather than as a string comparison at each call site — which is how a
// second, subtly different definition of "started" gets born.
//
// It is deliberately NOT the signal for the portrait nudge. A member who started the
// event without a photo must still be nudged (PRD 005, clarified 2026-08-25): having
// verified a guardian number says nothing about whether there is a face on file. The
// two live next to each other in the flow and are easy to wire to the same signal by
// mistake, so keep them apart.
func (p Person) HasStarted() bool {
	return p.MemberStatus != ""
}

// NeedsPortrait reports whether this person should be nudged to add a photo.
//
// Driven only by the absence of a portrait, independent of verification or lifecycle
// status — see HasStarted.
func (p Person) NeedsPortrait() bool {
	return p.PortraitRef == ""
}

// Queries is the read API handed to the application through data.Models.
//
// Lookup returns a slice rather than a single Person on purpose: two people can
// share a phone number, and an interface returning one value would bake a
// collision policy into the type signature where nobody can see it. The policy is
// "disambiguate after PIN verification" (task 071); this shape is what lets the
// caller apply it.
type Queries interface {
	// Lookup returns every person in the given year registered with the given phone
	// number. The input is normalized here, so callers may pass either raw or
	// canonical form and cannot get it wrong. Empty slice, not an error, when nothing
	// matches.
	Lookup(year, phone string) ([]Person, error)

	// Get resolves a person by id, scoped to a year.
	Get(year, personID string) (Person, bool, error)

	// ExpiredPortraits returns the portraits that are due to be deleted: captured
	// before `before`, or with no capture time recorded at all.
	//
	// Scoped to portraits rather than returning whole people, because the retention job
	// (task 109) needs the refs and has no business holding a member's address while it
	// deletes an image.
	//
	// A NULL capture time counts as expired. "Unknown age" must not mean "kept
	// forever" — for a photograph of a minor held on a safety basis, the failure that
	// matters is the one where a row quietly becomes immortal.
	ExpiredPortraits(year string, before time.Time, limit int) ([]ExpiredPortrait, error)
}

// ExpiredPortrait is one portrait the retention job should remove.
type ExpiredPortrait struct {
	PersonID string
	// Refs is every object to delete: the full image and every thumbnail rendition.
	// A list rather than a full/thumb pair, so adding a size cannot leave a
	// recognisable face on disk after the record says the portrait was deleted.
	Refs []string
}

type querier struct {
	db         cqrs.Reader
	normalizer PhoneNormalizer
}

const personColumns = `
	personId, year, appRole,
	name, phone, phoneParent,
	address, postalCode, city, email, birthday,
	teamId, teamName,
	sectionSlug, sectionName,
	memberStatus, armNumber,
	verifiedAt, acknowledgedPhone, portraitRef, portraitThumbRef, portraitThumbs,
	portraitCapturedAt`

// Lookup finds people by phone number.
//
// The input is normalized here rather than being assumed already-canonical. That is
// the difference between an interface a caller can misuse and one they cannot: the
// projector and this lookup now provably fold numbers the same way, because it is
// literally the same injected implementation (see interfaces.go).
//
// Soft-deleted rows are excluded here rather than at the call site: a deleted member
// must lose their login, and leaving that filter to every caller is how one of them
// eventually forgets (task 076).
//
// # The guardian number is not a login key
//
// This matches on `phone` and must NEVER also match on `phoneParent`. Decided
// 2026-08-26: nobody logs in with a guardian's number — not the guardian, not the
// member.
//
// The temptation is concrete and will recur. 36 live 2026 spejder have no phone of their
// own but do have a guardian number on file (PRD 006 §11 Q13), so adding
// `OR phoneParent = ?` here looks like it rescues 36 locked-out children with one line.
// What it actually does is let a parent's handset authenticate *as the child*, silently
// and with no audit trail — and on a number shared between siblings, as one of several
// children. A member with no phone simply has no app, which is an accepted outcome.
//
// TestLoginNeverMatchesOnTheGuardianNumber enforces this, because a comment alone would
// not survive someone earnestly fixing a bug report.
func (q querier) Lookup(year, phoneInput string) ([]Person, error) {
	normalized := normalizeOrEmpty(q.normalizer, phoneInput)

	// Guard against an empty phone matching the column default. Without this, any
	// row that has no phone recorded would answer a lookup for "" — which is what an
	// unparseable input normalizes to.
	if normalized == "" {
		return nil, nil
	}

	rows, err := q.db.Query(`
		SELECT `+personColumns+`
		FROM person
		WHERE year = ? AND phone = ? AND deleted = 0
		ORDER BY personId`, year, normalized)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Person
	for rows.Next() {
		p, err := scanPerson(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (q querier) Get(year, personID string) (Person, bool, error) {
	row := q.db.QueryRow(`
		SELECT `+personColumns+`
		FROM person
		WHERE year = ? AND personId = ? AND deleted = 0`, year, personID)

	p, err := scanPerson(row)
	if err == sql.ErrNoRows {
		return Person{}, false, nil
	}
	if err != nil {
		return Person{}, false, err
	}
	return p, true, nil
}

func (q querier) ExpiredPortraits(year string, before time.Time, limit int) ([]ExpiredPortrait, error) {
	if limit <= 0 {
		limit = 500
	}

	// Deleted members are NOT excluded here, unlike in Lookup. A member removed from
	// the event still has a photograph of them on disk, and that is precisely a record
	// that must still expire — filtering it out would make deletion a way to keep an
	// image forever.
	rows, err := q.db.Query(`
		SELECT personId, portraitRef, portraitThumbRef, portraitThumbs
		FROM person
		WHERE year = ? AND portraitRef <> ""
		  AND (portraitCapturedAt IS NULL OR portraitCapturedAt < ?)
		ORDER BY personId
		LIMIT ?`, year, before.UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ExpiredPortrait
	for rows.Next() {
		var (
			p      Person
			thumbs *string
		)
		if err := rows.Scan(&p.PersonID, &p.PortraitRef, &p.PortraitThumbRef, &thumbs); err != nil {
			return nil, err
		}
		p.PortraitThumbs = decodeThumbs(thumbs)
		// Reuses Person.PortraitRefs so the purge and any other deletion path cannot
		// disagree about what a portrait consists of.
		out = append(out, ExpiredPortrait{PersonID: p.PersonID, Refs: p.PortraitRefs()})
	}
	return out, rows.Err()
}

// scanner covers both *sql.Row and *sql.Rows so one scan body serves Get and Lookup.
type scanner interface {
	Scan(dest ...any) error
}

func scanPerson(s scanner) (Person, error) {
	var p Person
	// Nullable in the database, and absent for every row written before the column
	// existed, so it cannot be scanned straight into a slice.
	var thumbs *string
	err := s.Scan(
		&p.PersonID, &p.Year, &p.AppRole,
		&p.Name, &p.Phone, &p.PhoneParent,
		&p.Address, &p.PostalCode, &p.City, &p.Email, &p.Birthday,
		&p.TeamID, &p.TeamName,
		&p.SectionSlug, &p.SectionName,
		&p.MemberStatus, &p.ArmNumber,
		&p.VerifiedAt, &p.AcknowledgedPhone, &p.PortraitRef, &p.PortraitThumbRef, &thumbs,
		&p.PortraitCapturedAt,
	)
	if err != nil {
		return p, err
	}
	p.PortraitThumbs = decodeThumbs(thumbs)
	return p, nil
}

// decodeThumbs parses the stored rendition list.
//
// Unparseable JSON yields no thumbnails rather than an error: the consequence is that
// readers fall back to the full image, which is worse-but-working, whereas failing the
// read would take down a **login** over a cosmetic column. This row is written by this
// package alone, so it should be unreachable — which is exactly why it must not be the
// thing that breaks the directory.
func decodeThumbs(encoded *string) []PortraitThumb {
	if encoded == nil || *encoded == "" {
		return nil
	}
	var out []PortraitThumb
	if err := json.Unmarshal([]byte(*encoded), &out); err != nil {
		return nil
	}
	return out
}

// Thumb returns the rendition with the given name, e.g. "thumb256".
func (p Person) Thumb(name string) (PortraitThumb, bool) {
	for _, t := range p.PortraitThumbs {
		if t.Name == name {
			return t, true
		}
	}
	return PortraitThumb{}, false
}

// PortraitRefs returns every blob this person's portrait occupies: the full image and
// every rendition.
//
// Exists so the retention job cannot delete a portrait and leave a rendition behind — the
// failure mode being a recognisable face still on disk after the record says it was
// deleted. One function, so adding a size does not mean finding every deletion site.
func (p Person) PortraitRefs() []string {
	if p.PortraitRef == "" {
		return nil
	}
	refs := []string{p.PortraitRef}
	seen := map[string]bool{p.PortraitRef: true}
	// PortraitThumbRef duplicates one of the renditions, so dedupe rather than returning
	// the same object twice.
	candidates := append([]string{p.PortraitThumbRef}, thumbRefs(p.PortraitThumbs)...)
	for _, ref := range candidates {
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		refs = append(refs, ref)
	}
	return refs
}

func thumbRefs(thumbs []PortraitThumb) []string {
	out := make([]string, 0, len(thumbs))
	for _, t := range thumbs {
		out = append(out, t.Ref)
	}
	return out
}

var _ Queries = querier{}
