package person

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/jrgensen/cqrs"

	"github.com/nathejk/shared-go/types"
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
	// TeamNumber is the number a patrulje is known by in the field ("138"), and is what
	// PRD 007's patrol lookup matches against. Empty for klaner, sections and anyone
	// whose team has not been numbered yet — all normal states, not missing data.
	TeamNumber string

	// SectionSlug and SectionName are the crew affiliation. Empty for spejder,
	// bandit and gøgler, who belong to a team instead — see TeamName.
	SectionSlug string
	SectionName string

	MemberStatus string
	ArmNumber    string

	VerifiedAt *time.Time
	// AcknowledgedPhone is the number the member said can be reached. Authoritative for
	// contacting a guardian during the event, because it may be a number the member supplied
	// when they could not recognise the registered one (task 148).
	AcknowledgedPhone *string

	// PhoneVerifiedAt and VerifiedPhone are the member's OWN number, proven by challenge-response:
	// they received a PIN by SMS on it and typed it back to log in (PRD 015, task 226). A
	// different fact from the contact number above, by a different mechanism — see the schema
	// comment on why the two never share a column.
	//
	// Read here for exactly one purpose: suppressing a re-publish when the number already recorded
	// is the one the PIN just proved. They are deliberately NOT part of IsVerified — a member who
	// logged in and skipped the contact check has not verified a contact number, and treating this
	// as though they had would fast-track at check-in the records that still need one.
	PhoneVerifiedAt *time.Time
	VerifiedPhone   *string

	// StartedPhone and StartedPhoneContact are what check-in recorded when this member started,
	// carried on the start event and projected since task 230 (PRD 015).
	//
	// After the start these are the numbers staff actually hold, which is why `ContactNumber()`
	// prefers them: showing the register's value while the counter holds a different one would put
	// a number on the member's screen that nobody would dial.
	StartedPhone        *string
	StartedPhoneContact *string

	PortraitRef string
	// PortraitThumbRef is the default (smallest) thumbnail's content hash, or empty when
	// the portrait predates thumbnails (task 104). Readers fall back to PortraitRef.
	PortraitThumbRef string
	// PortraitThumbs is every thumbnail rendition, each with its own size and byte count.
	// Empty for a portrait captured before the list existed.
	PortraitThumbs []PortraitThumb
	// PortraitOriginalRef is the content hash of the stored original (task 111), or empty
	// when none was kept. Never served to a client — it exists so renditions can be
	// produced again later.
	PortraitOriginalRef string
	// PortraitOrientation is the EXIF orientation the upload declared (1–8; 0 unknown).
	// Needed to re-render from the original, whose metadata was stripped.
	PortraitOrientation int
	// PortraitCapturedAt is when the current portrait was taken, or nil when there is
	// none (or when the event carried no timestamp). The retention job reads it; see
	// portrait.go.
	PortraitCapturedAt *time.Time
}

// MemberStatusRacing is the one lifecycle value this projection writes.
//
// Taken from shared-go rather than duplicated as a local string, which it was until the
// dependency was bumped for the verification message (task 147). The old comment justified the
// copy by saying this package cannot depend on `internal/` — true, and beside the point: it
// already imports `shared-go/messages`, so it can import `shared-go/types` and get the persisted
// value with its authoritative definition attached. That matters more than usual here, because
// shared-go's own doc warns that changing one of these strings is a data migration rather than a
// rename.
const MemberStatusRacing = string(types.MemberStatusRacing)

// IsVerified reports whether this person's contact number has been verified by the member.
//
// # It no longer expires when the register moves (PRD 015, task 225)
//
// Until task 225 this was `verifiedAt` **and** the register still holding the number the
// acknowledgement was made against: a changed `phoneParent` invalidated the verification and the
// member was asked again. That followed from treating the acknowledgement as consent about one
// specific number, where showing staff a tick for a number nobody agreed to is the expensive kind
// of wrong.
//
// PRD 015 changed what the tick is *for*. It is no longer a standing claim staff rely on during an
// emergency — it is a fast track past one question at check-in, and check-in is the backstop for
// everyone it does not cover. Under that framing the member verified *a* reachable number, which
// is what check-in wanted to know, and a later edit to the register does not unmake it. Sending
// the member round the check again because an organizer fixed a typo in their address (spejder
// details are re-published on any edit) costs more than it protects.
//
// So: a verification survives a `phoneParent` change. If that ever needs reversing, what comes
// back is the registered number on the event — the projection cannot reconstruct it, because the
// current row is not what the register held at the time.
//
// A person with no contact number at all (crew, bandit, gøgler — see HasGuardianPhone) can never
// be verified in this sense, and callers must not read that as "unverified, nag them": there is
// nothing for them to confirm.
func (p Person) IsVerified() bool {
	if p.VerifiedAt == nil {
		return false
	}
	// No number on file at all: whatever was verified, it is not something we can point at now,
	// so this is not a state in which a tick may be shown.
	return p.PhoneParent != nil
}

// ContactNumber returns the contact number that is actually in force for this member, and whether
// there is one at all.
//
// Two sources, in order (PRD 015, tasks 230/231):
//
//  1. What **check-in recorded** when the member started. From that moment it is what staff hold,
//     and the app must not show a different number — a member reading the register's stale value
//     off their own screen would believe we will call a phone nobody at the counter wrote down.
//  2. Otherwise `PhoneParent`, the register's value.
//
// The second return distinguishes "no contact number for this population" (bandit, crew, gøgler)
// from "one is expected", which the profile page renders differently and `confirmationRequired`
// reads — so it must not be collapsed into an empty string by a caller.
func (p Person) ContactNumber() (string, bool) {
	if p.StartedPhoneContact != nil && *p.StartedPhoneContact != "" {
		return *p.StartedPhoneContact, true
	}
	if p.PhoneParent == nil {
		return "", false
	}
	return *p.PhoneParent, true
}

// PhoneVerifiedIs reports whether the member's own number is already recorded as verified, and as
// this exact number.
//
// The suppression rule for the login publish (PRD 015, task 226), as a predicate here rather than
// a comparison at the call site — which is how "once per member per year" and "last verification
// wins" would drift into two different rules.
//
// Both halves matter. `PhoneVerifiedAt` alone would re-publish on every login forever; ignoring
// the number would mean a member whose phone changed never gets the new one recorded, because the
// old verification suppresses it. So: publish unless what we hold is already this number.
//
// The caller normalizes before comparing. Storing an unnormalized number here would make this
// return false for a number we already hold, and the cost is a duplicate event per login — quiet,
// and exactly the kind of quiet that survives a release.
func (p Person) PhoneVerifiedIs(normalized string) bool {
	if p.PhoneVerifiedAt == nil || p.VerifiedPhone == nil {
		return false
	}
	return *p.VerifiedPhone == normalized
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

	// ListByAppRoles returns every live person in the year holding one of the given app
	// roles, ordered by name.
	//
	// Roles rather than a free-form filter, because the one caller (PRD 007's contacts
	// manifest) decides who it may list from the *authorization* matrix, and a query that
	// took arbitrary predicates would let that decision leak into SQL. An empty roles
	// slice returns nothing rather than everything — the safe reading of "no roles are
	// permitted", and the difference matters because the caller derives the slice from a
	// permission check.
	//
	// Soft-deleted rows are excluded here, for the same reason Lookup excludes them.
	ListByAppRoles(year string, roles []string) ([]Person, error)

	// ListPatrolByNumber returns the members of one patrol, found by the number the patrol
	// is known by in the field.
	//
	// Two steps by design (PRD 007, task 157): the number resolves to a teamId via any one
	// numbered member, then every member of that team is returned. A patrol whose members
	// were added after the number was assigned carries the number on only some rows — see
	// handlePatrolNumberAssigned's note — and matching on teamId rather than on the number
	// itself means those members are still found.
	//
	// Exact match on the whole number, never a prefix: this backs an authorization-sensitive
	// lookup where partial matching would turn one permitted question ("show me patrol 138")
	// into an enumeration tool ("show me every patrol starting with 1").
	//
	// Empty slice, not an error, when no such patrol exists — the caller must answer the same
	// way for "no such patrol" as for "not allowed".
	ListPatrolByNumber(year, number string) ([]Person, error)

	// TrackMembers returns who is on one team, with what the lifecycle says about each of them.
	//
	// # Why ids and a status, and nothing else
	//
	// The one caller is the post-race patrol route (PRD 011, tasks 340 and 349), which needs two
	// things: whose points it may read, and **from when it must stop counting them**.
	// `ListPatrolByNumber` would answer the first and hand back whole `Person` values — names, phone
	// numbers and `phoneParent` — into a code path that renders an **unauthenticated** page.
	//
	// So this read is narrowed to the three fields that path has any use for. It is the same
	// discipline `ExpiredPortraits` follows: the retention job gets refs rather than people, because
	// it has no business holding a member's address while it deletes an image.
	//
	// This replaced a `MemberIDs` that returned ids alone. The status had to come with them because
	// a person who left the race may have been driven home, and their phone keeps recording from the
	// car — so their later points are not the patrol walking (PRD 011 §0b.6). Two reads would have
	// been two chances for the ids and the statuses to disagree about who is in the patrol.
	//
	// Soft-deleted rows are excluded, as in every other read here. A member whose record was
	// removed should not have their positions drawn.
	//
	// Empty slice, not an error, for an unknown team — and an empty team id returns nothing
	// rather than every member with no team, which is the safe reading and the one personnel
	// roles need.
	TrackMembers(year, teamID string) ([]TrackMember, error)

	// CrewRoster returns the year's crew as an id and a name, for the photo credit picker
	// (PRD 025 §6 R2, task 449).
	//
	// # Why it is not a Person
	//
	// The same discipline as TrackMembers and ExpiredPortraits, and for a sharper reason: the
	// caller is a list the curator's tool renders, so every field this returns is a field
	// something is about to draw. A `Person` would hand a photograph-credit picker a phone
	// number, an email, an address, a birthday and — for anyone who is also on a spejder row —
	// a guardian's number. Two fields is what the picker needs, so two fields is what exists.
	//
	// # Who counts as crew
	//
	// `CrewRoles` — the union of four (see classify.go), which is the same rule the credit resolver
	// applies. A spejder, a bandit or a gøgler can never appear.
	//
	// It was `RoleCrew` alone for one commit, which was the narrow reading of PRD 025 and the wrong
	// one: a photographer registered under `guider` or `samarit` classifies to that role and would
	// have been invisible to the picker, which is precisely the curator-types-it-by-hand case the
	// feature exists to remove. The maintainer widened it (2026-09-28). The safety property is
	// unchanged either way — participants are excluded by both readings.
	//
	// It must not drift from the resolver: a picker offering a name the resolver then refuses to
	// publish is the failure this shared definition prevents.
	//
	// sectionSlug narrows to one section; "" is the whole year's crew. The caller defaults it to
	// "pr", where the photographers are, and `pr` is the default filter rather than the boundary.
	//
	// A row with no name is skipped: the assignment event can land before the member's details
	// (see handleSectionAssigned's stub row), and a blank entry in a picker is an option that
	// credits a photograph to nobody.
	//
	// Empty slice, not an error, when nothing matches.
	CrewRoster(year, sectionSlug string) ([]CrewMember, error)

	// CreditNames resolves photo credit references to names (PRD 025 §6 R3, task 451).
	//
	// **This is the only path from the person projection to a public page, and it is meant to stay
	// the only one.** PRD 022 §6 originally forbade it outright — the credit line was admitted as
	// the one field naming a human being *because* nothing derived it — and PRD 025 reversed that
	// for one reason: erasure. A name copied into `photo.credit` is also on the append-only event
	// log and could never be deleted; a reference can be, in one place. So the join exists, and
	// everything about this method is an attempt to keep it the narrow thing that was agreed.
	//
	// Four bounds, each one load-bearing:
	//
	//   1. **`CrewRoles` only.** A spejder's, a bandit's or a gøgler's id resolves to nothing, so a
	//      mistyped, stale or malicious reference cannot publish a participant's name. This is the
	//      bound that matters; the others are hygiene around it.
	//   2. **The name column only.** Nothing else on that row is read, ever. The row holds a phone
	//      number, a guardian's number, an email, an address and a birthday.
	//   3. **One year.** The photograph's own, with no fallback to another — the same rule every
	//      read here follows. Somebody who was crew in 2026 and is not in 2027 still has their 2026
	//      photographs credited, because those resolve against 2026.
	//   4. **Absent means absent.** An id with no live crew row in that year is simply missing from
	//      the result, and the caller renders no credit line (R5). That is the erasure path: delete
	//      the person and the name is gone from every photograph, with nothing to rewrite.
	//
	// A map keyed by id, and batched, because the album page resolves a page of photographs at once
	// — a query per photograph would be the obvious way to make this correct and unusable.
	//
	// Empty map, not an error, when nothing matches.
	CreditNames(year string, personIDs []string) (map[string]string, error)

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

// TrackMember is one member of a team, as the post-race route path needs them.
//
// Deliberately three fields. A `Person` would carry a name and a guardian's number into the handler that
// renders an unauthenticated page; this carries an id, a lifecycle value and a time.
type TrackMember struct {
	PersonID string

	// MemberStatus is the raw `types.MemberStatus` value, or "" for a member no lifecycle event has
	// touched.
	//
	// **Raw, not interpreted.** Whether a status means "left the race" is a question shared-go should own
	// (task 175) and `cmd/api` currently answers in one place; this projection storing the answer would be
	// a second definition of it, and two definitions of "left the race" is exactly what task 175 exists to
	// prevent.
	MemberStatus string

	// StatusAt is when that status was recorded, or nil when it is unknown.
	//
	// Taken from the event's own envelope time, which **every** event in the stream carries — verified across
	// the whole 2026 stream rather than assumed (task 349's correction). So nil is not the normal case: on
	// current data every status write is stamped.
	//
	// It happens for one real reason, and it is worth knowing because it looks like a bug: these projections
	// are never truncated, so a status written from an **earlier state of the stream** outlives the event
	// that produced it, and a column added later stays NULL on such a row because nothing current rewrites
	// it. Task 350. Both readings — no event, or an event from a stream state that no longer exists — leave
	// the caller unable to say *when* a withdrawal happened, which is why PRD 011 §0b.6 has it exclude that
	// member's points entirely rather than guess a cutoff.
	StatusAt *time.Time
}

// CrewMember is one crew member as the credit picker needs them: an id to store and a name to show.
//
// **Two fields, and a third is a decision rather than a convenience.** Everything else on the person
// row — phone, phoneParent, email, address, birthday, the portrait refs — is something a photo-credit
// picker has no use for and a rendered list would expose. `cmd/api`'s adminCrewMember mirrors this
// shape, and its test fails if either one grows.
type CrewMember struct {
	PersonID string
	Name     string
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
	teamId, teamName, teamNumber,
	sectionSlug, sectionName,
	memberStatus, armNumber,
	verifiedAt, acknowledgedPhone,
	phoneVerifiedAt, verifiedPhone,
	startedPhone, startedPhoneContact,
	portraitRef, portraitThumbRef, portraitThumbs,
	portraitOriginalRef, portraitOrientation, portraitCapturedAt`

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

func (q querier) ListByAppRoles(year string, roles []string) ([]Person, error) {
	// No roles means no permitted populations, which is not the same as "unfiltered".
	// Returning early rather than building `IN ()` also avoids a SQL syntax error, but
	// the reason it is written as a guard is the first one: the caller got here from an
	// authorization decision, and the empty case must fail closed.
	if len(roles) == 0 {
		return nil, nil
	}

	placeholders := strings.Repeat("?,", len(roles)-1) + "?"
	args := make([]any, 0, len(roles)+1)
	args = append(args, year)
	for _, r := range roles {
		args = append(args, r)
	}

	// Ordered by name so the payload is stable between requests: the manifest is
	// content-hashed for its ETag, and an unstable order would make every poll look like
	// a change. personId breaks ties, since duplicate names are common in this data
	// (task 078 found 70 shared numbers whose rows carry the same name).
	rows, err := q.db.Query(`
		SELECT `+personColumns+`
		FROM person
		WHERE year = ? AND deleted = 0 AND appRole IN (`+placeholders+`)
		ORDER BY name, personId`, args...)
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

func (q querier) ListPatrolByNumber(year, number string) ([]Person, error) {
	// Guard against an empty number matching the column default, the same trap Lookup
	// guards against with an empty phone: without this, a caller who sends nothing gets
	// every unnumbered person in the event — which is most of them.
	number = strings.TrimSpace(number)
	if number == "" {
		return nil, nil
	}

	// One statement rather than two round trips: the subquery resolves the number to a team,
	// the outer query returns that team's members. LIMIT 1 in the subquery because every
	// numbered member of a patrol agrees about its teamId, and MySQL needs a scalar here.
	rows, err := q.db.Query(`
		SELECT `+personColumns+`
		FROM person
		WHERE year = ? AND deleted = 0
		  AND teamId = (
		    SELECT teamId FROM person
		    WHERE year = ? AND deleted = 0 AND teamNumber = ? AND teamId <> ""
		    LIMIT 1
		  )
		ORDER BY name, personId`, year, year, number)
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

// TrackMembers returns who is on one team, with what the lifecycle says about each of them.
//
// Selects three columns, deliberately: this read exists so the post-race route path never holds a
// `Person`. See the interface's doc — the caller renders an unauthenticated page, and the cheapest way to
// guarantee it cannot leak a name is for the name never to arrive.
func (q querier) TrackMembers(year, teamID string) ([]TrackMember, error) {
	if teamID == "" {
		// An empty team id would otherwise match every member with no team — which is every personnel
		// role in the event. The same reading `reveal.Revealed` applies to an empty patrol id.
		return nil, nil
	}

	rows, err := q.db.Query(`
		SELECT personId, memberStatus, memberStatusAt
		FROM person
		WHERE year = ? AND deleted = 0 AND teamId = ?
		ORDER BY personId`, year, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TrackMember
	for rows.Next() {
		var m TrackMember
		var at sql.NullTime
		if err := rows.Scan(&m.PersonID, &m.MemberStatus, &at); err != nil {
			return nil, err
		}
		if at.Valid {
			t := at.Time
			m.StatusAt = &t
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// CreditNames resolves credit references to crew names. See the interface for the four bounds.
//
// # Why this reads the same `appRole IN (…)` as the roster
//
// Because a picker that offers a name the reader then refuses to publish is worse than either behaviour on its
// own: the curator credits a photograph, the tool says it worked, and the public page shows nothing. Both sides
// go through `CrewRoles` so that cannot happen.
func (q querier) CreditNames(year string, personIDs []string) (map[string]string, error) {
	out := map[string]string{}
	if year == "" || len(personIDs) == 0 {
		// An empty year would match the column default rather than "every year". Nothing crosses a year here.
		return out, nil
	}

	// One placeholder per id and one per role, all bound. The ids come from a projection this service wrote,
	// but they reach here through an HTTP request's worth of indirection, and this statement is assembled by
	// string concatenation like every other in this package.
	idMarks := make([]string, 0, len(personIDs))
	args := []any{year}
	for _, id := range personIDs {
		if id == "" {
			continue
		}
		idMarks = append(idMarks, "?")
		args = append(args, id)
	}
	if len(idMarks) == 0 {
		return out, nil
	}
	roleMarks := make([]string, len(CrewRoles))
	for i, role := range CrewRoles {
		roleMarks[i] = "?"
		args = append(args, role)
	}

	// `name` and nothing else. Worth stating at the SELECT as well as in the doc, because this is the line a
	// future change would widen — the temptation being "while we are here, the section name would look nice
	// under the photograph".
	//
	// `deleted = 0` is the erasure mechanism (PRD 025 §6 R4): a deleted crew member's photographs stop being
	// credited, which is the whole reason the credit is a reference.
	rows, err := q.db.Query(`
		SELECT personId, name
		FROM person
		WHERE year = ? AND personId IN (`+strings.Join(idMarks, ", ")+`)
		  AND appRole IN (`+strings.Join(roleMarks, ", ")+`)
		  AND deleted = 0 AND name <> ""`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

// CrewRoster returns the year's crew, optionally narrowed to one section.
//
// Selects two columns, for the reason given on the interface: the caller renders what it is handed.
func (q querier) CrewRoster(year, sectionSlug string) ([]CrewMember, error) {
	if year == "" {
		// An empty year would match the column default rather than "every year", the same trap the
		// other reads guard their key against. Nothing crosses a year in this service.
		return nil, nil
	}

	// `appRole IN (…)` rather than a slug-derived role computed in Go: appRole is what the classifier wrote
	// at fold time and `KEY year_role` indexes, and re-deriving it from sectionSlug here would be a second
	// opinion about who is crew.
	marks := make([]string, len(CrewRoles))
	args := []any{year}
	for i, role := range CrewRoles {
		marks[i] = "?"
		args = append(args, role)
	}
	where := `year = ? AND deleted = 0 AND appRole IN (` + strings.Join(marks, ", ") + `) AND name <> ""`

	// Folded with the exported normalizer, not compared raw: sectionSlug is stored as the organizer
	// typed it, and a caller writing `"PR"` must not silently get an empty roster. Case is then MySQL's
	// to ignore under the table's collation — the same exact match handleSectionAdded's back-fill makes.
	if slug := NormalizeSectionSlug(sectionSlug); slug != "" {
		where += ` AND sectionSlug = ?`
		args = append(args, slug)
	}

	// By name, because this is read as a list a person scans. personId breaks ties, since duplicate
	// names are ordinary in this data (task 078) and an unstable order in a picker moves the entry
	// under the cursor between requests.
	rows, err := q.db.Query(`
		SELECT personId, name
		FROM person
		WHERE `+where+`
		ORDER BY name, personId`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CrewMember
	for rows.Next() {
		var m CrewMember
		if err := rows.Scan(&m.PersonID, &m.Name); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
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
		SELECT personId, portraitRef, portraitThumbRef, portraitThumbs, portraitOriginalRef
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
		if err := rows.Scan(&p.PersonID, &p.PortraitRef, &p.PortraitThumbRef, &thumbs,
			&p.PortraitOriginalRef); err != nil {
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
		&p.TeamID, &p.TeamName, &p.TeamNumber,
		&p.SectionSlug, &p.SectionName,
		&p.MemberStatus, &p.ArmNumber,
		&p.VerifiedAt, &p.AcknowledgedPhone,
		&p.PhoneVerifiedAt, &p.VerifiedPhone,
		&p.StartedPhone, &p.StartedPhoneContact,
		&p.PortraitRef, &p.PortraitThumbRef, &thumbs,
		&p.PortraitOriginalRef, &p.PortraitOrientation,
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
	// the same object twice. The original is included: retention deletes the portrait
	// entirely, and an original left behind is a full-resolution face still on disk.
	candidates := append([]string{p.PortraitThumbRef, p.PortraitOriginalRef},
		thumbRefs(p.PortraitThumbs)...)
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
