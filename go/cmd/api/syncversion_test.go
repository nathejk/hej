package main

import (
	"errors"
	"testing"
	"time"

	"nathejk.dk/internal/data"
	"nathejk.dk/internal/scans"
	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/checkpoint"
	"nathejk.dk/nathejk/table/person"
)

var errRaceAreaRead = errors.New("connection refused")

// Same two obligations as mapversion_test.go (task 269), because they are the two ways a version can
// be wrong and each fails in its own direction:
//   - it CHANGES when the data changes — a stable-but-wrong version silently strands a device on
//     stale data for a whole event, and nothing reports it;
//   - it is STABLE across repeated calls on unchanged data — an unstable version makes every device
//     refetch every payload on every poll, which is the cost the version exists to avoid.

func ptr[T any](v T) *T { return &v }

func profileFixture() (users.User, person.Person) {
	user := users.User{
		ID: "p-1", Role: users.RoleSpejder, Name: "Sofie Spejder",
		PatrolName: "Patrulje 138", Address: "Vestergade 1", PostalCode: "8000", City: "Aarhus",
		Phone: "+4530000001", PhoneParent: ptr("+4520000001"),
	}
	p := person.Person{
		PersonID: "p-1", Name: "Sofie Spejder", Phone: "+4530000001",
		PhoneParent: ptr("+4520000001"), PortraitRef: "ref-abc",
	}
	return user, p
}

func TestProfileVersion_ChangesAndIsStable(t *testing.T) {
	user, p := profileFixture()

	v := profileVersion(user, p, true)
	if v == "" {
		t.Fatal("version must not be empty")
	}
	if again := profileVersion(user, p, true); again != v {
		t.Errorf("unchanged data must hash the same: %q vs %q", v, again)
	}

	// Every field the profile can report, including the ones it derives. A change the version misses
	// is a change the member never sees.
	userCases := map[string]func(*users.User){
		"name":       func(u *users.User) { u.Name = "Sofie S." },
		"role":       func(u *users.User) { u.Role = users.RoleCrew },
		"team":       func(u *users.User) { u.PatrolName = "Patrulje 139" },
		"section":    func(u *users.User) { u.Section = "Depot" },
		"address":    func(u *users.User) { u.Address = "Vestergade 2" },
		"postalCode": func(u *users.User) { u.PostalCode = "8200" },
		"city":       func(u *users.User) { u.City = "Risskov" },
		"phone":      func(u *users.User) { u.Phone = "+4530000009" },
		// The guardian number is the most consequential field in the record, and this endpoint is the
		// one place `.rules` permits it: a correction that never reached the member it belongs to is
		// the exact silent failure PRD 017 exists to prevent.
		"phoneParent":        func(u *users.User) { u.PhoneParent = ptr("+4520000009") },
		"phoneParentCleared": func(u *users.User) { u.PhoneParent = ptr("") },
		// nil and "" are different answers — "no guardian number for this population" versus "one is
		// expected and missing" — and the profile page renders them differently.
		"phoneParentNil": func(u *users.User) { u.PhoneParent = nil },
	}
	for name, mutate := range userCases {
		t.Run("user/"+name, func(t *testing.T) {
			changed := user
			mutate(&changed)
			if profileVersion(changed, p, true) == v {
				t.Errorf("a change to %s must change the version", name)
			}
		})
	}

	personCases := map[string]func(*person.Person){
		// has_photo, and more: a replaced portrait changes the ref without changing the flag, and the
		// client still has to refetch the bytes.
		"portraitRef":         func(x *person.Person) { x.PortraitRef = "ref-def" },
		"portraitRemoved":     func(x *person.Person) { x.PortraitRef = "" },
		"phoneParent":         func(x *person.Person) { x.PhoneParent = ptr("+4520000099") },
		"startedPhoneContact": func(x *person.Person) { x.StartedPhoneContact = ptr("+4520000077") },
		// contact_settled and confirmation_required both turn on this.
		"memberStatus": func(x *person.Person) { x.MemberStatus = "racing" },
		"verifiedAt":   func(x *person.Person) { x.VerifiedAt = ptr(time.Unix(1700, 0)) },
	}
	for name, mutate := range personCases {
		t.Run("person/"+name, func(t *testing.T) {
			changed := p
			mutate(&changed)
			if profileVersion(user, changed, true) == v {
				t.Errorf("a change to %s must change the version", name)
			}
		})
	}

	// A record disappearing turns a 200 into a 404. That is a change.
	if profileVersion(user, person.Person{}, false) == v {
		t.Error("losing the projection row must change the version")
	}
}

// The timestamp goes in as an instant, not as a rendering: a projection rebuilt in another zone must
// not mint a new version for data nobody touched.
func TestProfileVersion_TimestampIsZoneIndependent(t *testing.T) {
	user, p := profileFixture()
	p.VerifiedAt = ptr(time.Unix(1700, 0).UTC())

	elsewhere := p
	elsewhere.VerifiedAt = ptr(time.Unix(1700, 0).In(time.FixedZone("NZST", 12*3600)))

	if profileVersion(user, p, true) != profileVersion(user, elsewhere, true) {
		t.Error("the same instant in two zones must hash the same")
	}
}

func profileVersionApp(t *testing.T, stub *stubPeople) *application {
	t.Helper()
	app := newTestApp(t)
	app.config.eventYear = "2026"
	app.models = data.NewModels(users.NewMockDirectory(), scans.NewMockSource(), nil, stub, nil)
	return app
}

func TestProfileVersionFor_ChangesWhenDataChanges(t *testing.T) {
	_, p := profileFixture()
	stub := &stubPeople{p: p, found: true}
	app := profileVersionApp(t, stub)
	app.profileVersions = newVersionCache(0) // no caching: recompute every call

	viewer, _ := profileFixture()
	before, err := app.profileVersionFor(viewer)
	if err != nil {
		t.Fatalf("profileVersionFor: %v", err)
	}

	stub.p.PortraitRef = "ref-new"
	after, err := app.profileVersionFor(viewer)
	if err != nil {
		t.Fatalf("profileVersionFor: %v", err)
	}
	if before == after {
		t.Error("a new portrait must change the version through the *For path")
	}
}

// Keyed by user, which is what "keyed by permitted set" means for a dataset whose permitted set is
// one person. Two callers must never share an answer here — that would hand one member another's
// freshness, and with it a wrong decision about whether to refetch their own address.
func TestProfileVersionFor_CachedPerUser(t *testing.T) {
	_, p := profileFixture()
	stub := &stubPeople{p: p, found: true}
	app := profileVersionApp(t, stub)
	app.profileVersions = newVersionCache(time.Hour)

	alice, _ := profileFixture()
	bob := alice
	bob.ID = "p-2"
	bob.Name = "Bo Bandit"

	vAlice, _ := app.profileVersionFor(alice)
	vBob, _ := app.profileVersionFor(bob)

	if vAlice == vBob {
		t.Error("two users must not share a profile version")
	}

	// Alice's second call is cached: the data has changed underneath and she still gets her first
	// answer, proving the cache is consulted rather than being decorative.
	stub.p.PortraitRef = "ref-changed"
	if again, _ := app.profileVersionFor(alice); again != vAlice {
		t.Errorf("a cached version must be reused within the TTL: %q vs %q", vAlice, again)
	}
}

// No projection row is a normal state, not an error: the profile still renders from the directory, so
// it still needs a version — and that version must still track the directory half.
func TestProfileVersionFor_NoProjectionRow(t *testing.T) {
	app := profileVersionApp(t, &stubPeople{found: false})
	app.profileVersions = newVersionCache(0)

	viewer, _ := profileFixture()
	v, err := app.profileVersionFor(viewer)
	if err != nil {
		t.Fatalf("profileVersionFor: %v", err)
	}
	if v == "" {
		t.Fatal("a profile with no projection row must still have a version")
	}

	viewer.City = "Randers"
	if changed, _ := app.profileVersionFor(viewer); changed == v {
		t.Error("the directory half must still move the version with no projection row")
	}
}

func areaFixture() checkpoint.RaceArea {
	return checkpoint.RaceArea{
		Polygon:         []checkpoint.Point{{Lat: 56.1, Lng: 9.5}, {Lat: 56.2, Lng: 9.6}, {Lat: 56.0, Lng: 9.7}},
		SouthWest:       checkpoint.Point{Lat: 56.0, Lng: 9.5},
		NorthEast:       checkpoint.Point{Lat: 56.2, Lng: 9.7},
		AreaKm2:         428.5,
		PositionedCount: 9,
		TotalCount:      12,
	}
}

func TestRaceAreaVersion_ChangesAndIsStable(t *testing.T) {
	base := areaFixture()

	v := raceAreaVersion(base, true)
	if v == "" {
		t.Fatal("version must not be empty")
	}
	if again := raceAreaVersion(base, true); again != v {
		t.Errorf("unchanged data must hash the same: %q vs %q", v, again)
	}

	cases := map[string]func(*checkpoint.RaceArea){
		"vertexMoved":     func(a *checkpoint.RaceArea) { a.Polygon[1].Lat = 56.25 },
		"vertexAdded":     func(a *checkpoint.RaceArea) { a.Polygon = append(a.Polygon, checkpoint.Point{Lat: 56.3, Lng: 9.8}) },
		"southWest":       func(a *checkpoint.RaceArea) { a.SouthWest.Lng = 9.4 },
		"northEast":       func(a *checkpoint.RaceArea) { a.NorthEast.Lat = 56.3 },
		"areaKm2":         func(a *checkpoint.RaceArea) { a.AreaKm2 = 500 },
		"positionedCount": func(a *checkpoint.RaceArea) { a.PositionedCount = 10 },
		"totalCount":      func(a *checkpoint.RaceArea) { a.TotalCount = 13 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			changed := areaFixture()
			mutate(&changed)
			if raceAreaVersion(changed, true) == v {
				t.Errorf("a change to %s must change the version", name)
			}
		})
	}

	// The transition this version exists for above all others: an event that gains an area is an event
	// whose devices can start caching tiles.
	if raceAreaVersion(checkpoint.RaceArea{}, false) == v {
		t.Error("having no area must differ from having one")
	}

	// Vertex *order* is part of the payload — the client draws the polygon in the order it arrives.
	reordered := areaFixture()
	reordered.Polygon[0], reordered.Polygon[2] = reordered.Polygon[2], reordered.Polygon[0]
	if raceAreaVersion(reordered, true) == v {
		t.Error("reordering the polygon must change the version")
	}
}

// Shared by the whole event, which is the point: one entry serves every device, where keying by user
// would multiply a single identical answer by the device count.
func TestRaceAreaVersionFor_SharedAcrossViewers(t *testing.T) {
	area := areaFixture()
	app := raceAreaApp(t, fakeRaceAreas{area: area, ok: true}, "2026")
	app.raceAreaVersions = newVersionCache(time.Hour)

	spejder := users.User{ID: "u1", Role: users.RoleSpejder, PatrolID: "team-1"}
	crew := users.User{ID: "u2", Role: users.RoleCrew}

	v1, err := app.raceAreaVersionFor(spejder)
	if err != nil {
		t.Fatalf("raceAreaVersionFor: %v", err)
	}

	// Different role, different patrol, no patrol at all: still the same event, so still the same hull.
	// Swapping the source proves the second answer came from the cache rather than from a re-read.
	app.models = data.NewModels(users.NewMockDirectory(), scans.NewMockSource(),
		fakeRaceAreas{area: checkpoint.RaceArea{AreaKm2: 1}, ok: true}, nil, nil)
	v2, err := app.raceAreaVersionFor(crew)
	if err != nil {
		t.Fatalf("raceAreaVersionFor: %v", err)
	}
	if v1 != v2 {
		t.Errorf("every device in an event must share the race-area version: %q vs %q", v1, v2)
	}
}

func TestRaceAreaVersionFor_ChangesWhenDataChanges(t *testing.T) {
	app := raceAreaApp(t, fakeRaceAreas{area: areaFixture(), ok: true}, "2026")
	app.raceAreaVersions = newVersionCache(0)

	viewer := users.User{ID: "u1", Role: users.RoleSpejder}
	before, _ := app.raceAreaVersionFor(viewer)

	moved := areaFixture()
	moved.Polygon[0].Lat = 55.9
	app.models = data.NewModels(users.NewMockDirectory(), scans.NewMockSource(),
		fakeRaceAreas{area: moved, ok: true}, nil, nil)

	after, _ := app.raceAreaVersionFor(viewer)
	if before == after {
		t.Error("a moved hull must change the version through the *For path")
	}
}

// An outage must not be cached as a version, or freshness would stay broken for the TTL after the
// projection came back.
func TestRaceAreaVersionFor_NoProjection(t *testing.T) {
	app := raceAreaApp(t, nil, "2026")
	app.raceAreaVersions = newVersionCache(time.Hour)

	viewer := users.User{ID: "u1"}
	v, err := app.raceAreaVersionFor(viewer)
	if err != nil {
		t.Fatalf("raceAreaVersionFor: %v", err)
	}
	if v != raceAreaVersion(checkpoint.RaceArea{}, false) {
		t.Error("no projection must answer like no area")
	}

	// Now the projection is back. The answer must move immediately rather than being pinned by a
	// cached outage.
	app.models = data.NewModels(users.NewMockDirectory(), scans.NewMockSource(),
		fakeRaceAreas{area: areaFixture(), ok: true}, nil, nil)
	if recovered, _ := app.raceAreaVersionFor(viewer); recovered == v {
		t.Error("an outage must not be cached; the version must move when the projection returns")
	}
}

// A read error is reported rather than hashed into a version. Hashing it would be the worst of both:
// a plausible-looking version that means "we could not tell", which the client would store as fact.
func TestRaceAreaVersionFor_ReadErrorPropagates(t *testing.T) {
	app := raceAreaApp(t, fakeRaceAreas{err: errRaceAreaRead}, "2026")
	app.raceAreaVersions = newVersionCache(0)

	if _, err := app.raceAreaVersionFor(users.User{ID: "u1"}); err == nil {
		t.Error("a projection read error must be returned, not hashed into a version")
	}
}
