package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strconv"
	"time"

	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/checkpoint"
	"nathejk.dk/nathejk/table/person"
)

// Cheap "did this change?" versions for the two remaining sync datasets: the caller's own profile
// and the race area (task 283, PRD 017's foreground-sync check).
//
// Companion to mapversion.go, whose header states the rules and the two traps these follow —
// short opaque string, scoped to the caller's permitted set, computed from a projection read and
// hashed over the *data* rather than the rendered payload, cached by permitted set rather than by
// user. Read it before changing anything here.
//
// What is different about these two is the *scope* of the permitted set, and it differs in opposite
// directions:
//
//   - **Profile is the narrowest dataset in the app.** Its permitted set is one person, so its cache
//     key is genuinely the user id. That is not a violation of "key by permitted set, not by user" —
//     it is that rule's answer for a dataset whose permitted set has one member. No sharing is
//     possible and none should be faked.
//   - **Race area is the widest.** Every device in the event holds the identical hull, so one cache
//     entry per event year serves every caller. Keying it by user would multiply a single shared
//     answer by the device count, which is precisely the load this design exists to avoid.

// hashTime writes a timestamp in a canonical form, nil-distinct.
//
// Unix seconds rather than a formatted string: the projection's timestamps carry a location, and
// two Times describing the same instant in different zones must hash the same, or a replayed
// projection would mint new versions for unchanged data.
func hashTime(h io.Writer, t *time.Time) {
	if t == nil {
		io.WriteString(h, "\x00nil")
		return
	}
	io.WriteString(h, strconv.FormatInt(t.UTC().Unix(), 10))
}

// hashStringPtr writes a *string, distinguishing nil from empty.
//
// Both are real, different answers throughout this codebase — nil means "this population has no
// contact number", "" means "one is expected and missing" — and the profile page renders them
// differently. A hash that flattened them would miss the transition between the two, which is
// exactly the change a member needs to see.
func hashStringPtr(h io.Writer, s *string) {
	if s == nil {
		io.WriteString(h, "\x00nil")
		return
	}
	io.WriteString(h, "v:")
	io.WriteString(h, *s)
}

// profileVersion hashes everything `/me/profile` can report about its owner.
//
// # It hashes inputs, not the rendered response
//
// Four of the payload's fields are derived — `has_photo`, `confirmation_required`, `verified_at`
// and `contact_settled` — and `phone_parent` is derived too (check-in's number takes precedence
// over the register's). Each is a *pure function* of fields hashed below, so covering the inputs
// covers the outputs:
//
//   - `has_photo` from PortraitRef
//   - `phone_parent` from StartedPhoneContact, PhoneParent (person and directory), user Phone
//   - `confirmation_required` from PhoneParent, VerifiedAt, MemberStatus
//   - `verified_at` from VerifiedAt, PhoneParent
//   - `contact_settled` from MemberStatus
//
// Hashing the inputs rather than calling the derivations keeps this out of the business of
// rebuilding the response — which is the one thing a version derivation must never do, since a
// version that costs a payload to compute has no reason to exist.
//
// # The guardian number is deliberately in the hash
//
// `.rules` keeps guardian numbers out of every payload with exactly one exception: a member
// confirming or approving *their own* guardian's number, which is this endpoint. So the number is
// legitimately part of this payload, and therefore has to be part of this version — a corrected
// guardian number that never reached the member it belongs to would be the silent failure PRD 017
// exists to prevent, on the most consequential field in the record. Note the direction of the rule:
// it forbids the number *leaving*, and a version is not a way out of the server. Nothing derived
// here is ever returned except as part of the profile the owner may already read.
//
// `found` is hashed because a record disappearing turns a 200 into a 404, which is a change.
func profileVersion(user users.User, p person.Person, found bool) string {
	h := sha256.New()

	// The directory half, which is what the response is assembled from.
	io.WriteString(h, user.Name)
	io.WriteString(h, "\x00")
	io.WriteString(h, string(user.Role))
	io.WriteString(h, "\x00")
	io.WriteString(h, user.PatrolName)
	io.WriteString(h, "\x00")
	io.WriteString(h, user.Section)
	io.WriteString(h, "\x00")
	io.WriteString(h, user.Address)
	io.WriteString(h, "\x00")
	io.WriteString(h, user.PostalCode)
	io.WriteString(h, "\x00")
	io.WriteString(h, user.City)
	io.WriteString(h, "\x00")
	io.WriteString(h, user.Phone)
	io.WriteString(h, "\x00")
	hashStringPtr(h, user.PhoneParent)
	io.WriteString(h, "\x1d")

	// The projection half, which is what the derived fields read.
	io.WriteString(h, strconv.FormatBool(found))
	if found {
		io.WriteString(h, "\x00")
		io.WriteString(h, p.PortraitRef)
		io.WriteString(h, "\x00")
		hashStringPtr(h, p.PhoneParent)
		io.WriteString(h, "\x00")
		hashStringPtr(h, p.StartedPhoneContact)
		io.WriteString(h, "\x00")
		io.WriteString(h, p.MemberStatus)
		io.WriteString(h, "\x00")
		hashTime(h, p.VerifiedAt)
	}

	return hex.EncodeToString(h.Sum(nil)[:16])
}

// profileVersionFor returns the version of the caller's own profile, cached by user.
//
// One person per key, for the reason in this file's header: the permitted set is one member, so
// there is nothing to share. The TTL still earns its place — it is what keeps a device foregrounding
// repeatedly from turning into a projection read per resume.
func (app *application) profileVersionFor(viewer users.User) (string, error) {
	if v, ok := app.profileVersions.get(viewer.ID); ok {
		return v, nil
	}

	// `person` degrades to "not found" rather than erroring, matching the handler: a profile with no
	// projection row still renders from the directory, so it still has a version.
	p, found := app.person(viewer.ID)
	version := profileVersion(viewer, p, found)
	app.profileVersions.put(viewer.ID, version)
	return version, nil
}

// raceAreaVersion hashes the hull the race-area payload carries.
//
// Every field of `raceAreaResponse` except `BufferKm`, which is a compile-time constant and cannot
// change without a deploy. `PositionedCount`/`TotalCount` are in it because the client shows them,
// so a tenth checkpoint gaining a position changes what the user reads even when the hull is
// unmoved.
//
// `ok` is hashed because "no area yet" is a normal early-season state served as 404, and the
// transition into having one is the single most important change this version can report: it is what
// lets a device start caching tiles.
func raceAreaVersion(area checkpoint.RaceArea, ok bool) string {
	h := sha256.New()
	io.WriteString(h, strconv.FormatBool(ok))
	if !ok {
		return hex.EncodeToString(h.Sum(nil)[:16])
	}
	io.WriteString(h, "\x1d")
	for _, pt := range area.Polygon {
		hashFloat(h, pt.Lat)
		io.WriteString(h, "\x00")
		hashFloat(h, pt.Lng)
		io.WriteString(h, "\x1e")
	}
	io.WriteString(h, "\x1d")
	hashFloat(h, area.SouthWest.Lat)
	io.WriteString(h, "\x00")
	hashFloat(h, area.SouthWest.Lng)
	io.WriteString(h, "\x00")
	hashFloat(h, area.NorthEast.Lat)
	io.WriteString(h, "\x00")
	hashFloat(h, area.NorthEast.Lng)
	io.WriteString(h, "\x00")
	hashFloat(h, area.AreaKm2)
	io.WriteString(h, "\x00")
	io.WriteString(h, strconv.Itoa(area.PositionedCount))
	io.WriteString(h, "\x00")
	io.WriteString(h, strconv.Itoa(area.TotalCount))
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// glimtVersionFor returns the version of what this caller's Glimt feed currently contains
// (PRD 019 §8, task 304).
//
// # Not cached, unlike every other derivation in this file
//
// The others cache by permitted set, because their answer is shared: every device in the event holds
// the same race area, and everyone with the contacts pane holds the same directory. A Glimt feed is
// per-caller by construction — it contains the caller's own group, plus their own posts, including
// ones that have been hidden from everyone else — so a cache would be one entry per member, which is
// a map that grows with the event and shares nothing.
//
// What makes that affordable is that the derivation is already a single indexed aggregate
// (`COUNT`, `MAX(createdAt)`, `MAX(hiddenAt)`) rather than a payload build. If it ever stops being
// that, it needs a cache before it needs anything else — this endpoint is hit by every device on
// every foreground.
//
// Includes hidden-state changes, which is the non-obvious part: a version tracking only creations
// would leave a reported glimt sitting in every client that already held it.
func (app *application) glimtVersionFor(viewer users.User) (string, error) {
	if app.models.Glimt == nil {
		// Reported as unavailable rather than as a version, so the client reads "unchanged,
		// ask again" instead of dropping the dataset. Absence would mean "you may not hold
		// this", which a client cannot recover from.
		return "", errors.New("glimt projection unavailable")
	}
	return app.models.Glimt.Version(app.config.eventYear, app.glimtFilter(viewer.ID, viewer.Role))
}

// raceAreaVersionFor returns the version of the event's race area, shared by every caller.
//
// Keyed by event year alone. The viewer is taken as an argument anyway, so this matches the shape of
// every other derivation and can be called from the same table in sync.go without a special case —
// but it deliberately ignores it, which is why one cache entry serves the whole event.
func (app *application) raceAreaVersionFor(_ users.User) (string, error) {
	key := app.config.eventYear

	if v, ok := app.raceAreaVersions.get(key); ok {
		return v, nil
	}

	if app.models.RaceAreas == nil {
		// No projection is not the same fact as "no area", but it produces the same response (503 vs
		// 404) as far as *change* goes: the client holds nothing either way. Not cached, so freshness
		// resumes the moment the projection does.
		return raceAreaVersion(checkpoint.RaceArea{}, false), nil
	}

	area, ok, err := app.models.RaceAreas.RaceArea(app.config.eventYear)
	if err != nil {
		return "", err
	}
	version := raceAreaVersion(area, ok)
	app.raceAreaVersions.put(key, version)
	return version, nil
}
