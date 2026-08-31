package main

import (
	"errors"
	"sync"
	"testing"
	"time"

	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/person"
)

// fakeQueries stands in for the projection so the adapter can be tested without a
// database. The real querier is covered by its own sqlmock tests.
type fakeQueries struct {
	byPhone map[string][]person.Person
	byID    map[string]person.Person
	err     error
}

func (f fakeQueries) Lookup(_, phone string) ([]person.Person, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byPhone[phone], nil
}

func (f fakeQueries) Get(_, id string) (person.Person, bool, error) {
	if f.err != nil {
		return person.Person{}, false, f.err
	}
	p, ok := f.byID[id]
	return p, ok, nil
}

// The directory adapter never calls this; it is here to satisfy person.Queries.
func (f fakeQueries) ExpiredPortraits(string, time.Time, int) ([]person.ExpiredPortrait, error) {
	return nil, nil
}

// Nor this — contacts listing has its own tests (contacts_test.go).
func (f fakeQueries) ListByAppRoles(string, []string) ([]person.Person, error) {
	return nil, nil
}

// Every app role the projection can produce must survive the trip to users.Role. The
// values are deliberate duplicates on both sides, so this is the test that catches the
// two lists drifting apart — at which point a real member would be refused a login.
func TestDirectoryTranslatesEveryAppRole(t *testing.T) {
	for _, role := range users.AllRoles {
		q := fakeQueries{byPhone: map[string][]person.Person{
			"+4530112233": {{PersonID: "p1", AppRole: string(role), Name: "Test"}},
		}}
		d := newPersonDirectory(q, "2026", quietLogger())

		u, found := d.Lookup("+4530112233")
		if !found {
			t.Errorf("role %q: not found — the person and users role lists have drifted", role)
			continue
		}
		if u.Role != role {
			t.Errorf("role %q: got %q", role, u.Role)
		}
	}
}

// An unclassified row must not log in. Passing the string through would put an unknown
// value in the session and in GET /api/me, where the frontend router guard compares
// against a fixed enum; defaulting to RoleCrew would grant access off the back of a data
// problem.
func TestDirectoryRefusesAnUnrecognisedRole(t *testing.T) {
	for _, appRole := range []string{"", "traffikvagt", "admin", "Spejder"} {
		q := fakeQueries{
			byPhone: map[string][]person.Person{
				"+4530112233": {{PersonID: "p1", AppRole: appRole}},
			},
			byID: map[string]person.Person{"p1": {PersonID: "p1", AppRole: appRole}},
		}
		d := newPersonDirectory(q, "2026", quietLogger())

		if _, found := d.Lookup("+4530112233"); found {
			t.Errorf("appRole %q: must not resolve", appRole)
		}
		if got := d.LookupAll("+4530112233"); len(got) != 0 {
			t.Errorf("appRole %q: must not appear among candidates, got %v", appRole, got)
		}
		if _, found := d.Get("p1"); found {
			t.Errorf("appRole %q: must not resolve by id either", appRole)
		}
	}
}

// The collision rule belongs to users.Directory, and the adapter must implement it:
// several owners means NOT FOUND from Lookup, so a caller that has not thought about
// shared numbers refuses the login instead of signing someone in as their sibling.
func TestDirectoryLookupIsNotFoundForASharedNumber(t *testing.T) {
	q := fakeQueries{byPhone: map[string][]person.Person{
		"+4530112233": {
			{PersonID: "p1", AppRole: string(users.RoleSpejder), Name: "Anders"},
			{PersonID: "p2", AppRole: string(users.RoleSpejder), Name: "Sofie"},
		},
	}}
	d := newPersonDirectory(q, "2026", quietLogger())

	if _, found := d.Lookup("+4530112233"); found {
		t.Error("a shared number must not resolve to a single user")
	}
	// LookupAll is the shape the chooser needs, and must still return both.
	if got := d.LookupAll("+4530112233"); len(got) != 2 {
		t.Errorf("want 2 candidates, got %d", len(got))
	}
}

// Anti-enumeration: a database failure must look exactly like an unknown number. If the
// two were distinguishable the login form becomes a membership oracle — "is this phone
// number registered for Nathejk?" answerable by anyone.
func TestDirectoryFailureIsIndistinguishableFromUnknown(t *testing.T) {
	broken := newPersonDirectory(fakeQueries{err: errors.New("connection refused")}, "2026", quietLogger())
	empty := newPersonDirectory(fakeQueries{}, "2026", quietLogger())

	for _, phone := range []string{"+4530112233", "+4599999999"} {
		bu, bfound := broken.Lookup(phone)
		eu, efound := empty.Lookup(phone)
		if bfound != efound || bu != eu {
			t.Errorf("%s: a query error (%v/%v) differs from unknown (%v/%v)",
				phone, bu, bfound, eu, efound)
		}

		if len(broken.LookupAll(phone)) != len(empty.LookupAll(phone)) {
			t.Errorf("%s: LookupAll distinguishes an error from unknown", phone)
		}
	}

	if _, found := broken.Get("p1"); found {
		t.Error("Get must not resolve when the query failed")
	}
}

// The fields task 079's chooser depends on must survive: a first name, plus the team or
// section that is often the only thing telling two candidates apart.
func TestDirectoryCarriesTheDisambiguationFields(t *testing.T) {
	q := fakeQueries{byPhone: map[string][]person.Person{
		"+4530112233": {{
			PersonID: "p1", AppRole: string(users.RoleSpejder), Name: "Freja",
			TeamID: "team-9", TeamName: "Ravnene",
		}, {
			PersonID: "p2", AppRole: string(users.RoleSamarit), Name: "Mette",
			SectionName: "Samaritter",
		}},
	}}
	d := newPersonDirectory(q, "2026", quietLogger())

	got := d.LookupAll("+4530112233")
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	if got[0].Name != "Freja" || got[0].PatrolName != "Ravnene" || got[0].PatrolID != "team-9" {
		t.Errorf("team fields lost: %+v", got[0])
	}
	if got[1].Section != "Samaritter" {
		t.Errorf("section lost: %+v", got[1])
	}
	// Crew have no patrol, and empty must mean "no patrol" rather than an error.
	if got[1].PatrolID != "" || got[1].PatrolName != "" {
		t.Errorf("crew must have no patrol: %+v", got[1])
	}
}

// The year is fixed at construction, so every lookup in a request agrees about which
// event is running.
func TestDirectoryUsesTheConfiguredYear(t *testing.T) {
	var seen []string
	q := yearRecorder{seen: &seen}
	d := newPersonDirectory(q, "2025", quietLogger())

	d.LookupAll("+4530112233")
	d.Get("p1")

	for _, y := range seen {
		if y != "2025" {
			t.Errorf("query used year %q, want 2025", y)
		}
	}
	if len(seen) != 2 {
		t.Errorf("want 2 recorded queries, got %d", len(seen))
	}
}

type yearRecorder struct{ seen *[]string }

func (r yearRecorder) Lookup(year, _ string) ([]person.Person, error) {
	*r.seen = append(*r.seen, year)
	return nil, nil
}

func (r yearRecorder) Get(year, _ string) (person.Person, bool, error) {
	*r.seen = append(*r.seen, year)
	return person.Person{}, false, nil
}

// Unused by the directory adapter; present to satisfy person.Queries.
func (r yearRecorder) ExpiredPortraits(string, time.Time, int) ([]person.ExpiredPortrait, error) {
	return nil, nil
}

func (r yearRecorder) ListByAppRoles(year string, _ []string) ([]person.Person, error) {
	*r.seen = append(*r.seen, year)
	return nil, nil
}

// The switch starts on the mock so the app is usable before — or without — a broker, and
// moves to the real projection in place, without handlers holding a stale reference.
func TestSwitchableDirectoryStartsOnTheMockAndSwaps(t *testing.T) {
	mock := users.NewMockDirectory()
	s := newSwitchableDirectory(mock)

	// Whatever the mock recognises, the switch must recognise identically to begin with.
	// A specific seeded number rather than a probe loop: an earlier version searched for
	// one and silently skipped the whole test when it guessed wrong.
	const known = "+4530000001"
	if _, found := mock.Lookup(known); !found {
		t.Fatalf("the mock no longer seeds %s; this test needs a known number", known)
	}
	if _, found := s.Lookup(known); !found {
		t.Fatal("the switch must delegate to the mock before any swap")
	}

	// After the swap the real directory answers, and the mock's numbers stop working.
	real := newPersonDirectory(fakeQueries{byPhone: map[string][]person.Person{
		"+4599887766": {{PersonID: "real-1", AppRole: string(users.RoleBandit), Name: "Bandit"}},
	}}, "2026", quietLogger())
	s.set(real)

	if _, found := s.Lookup(known); found {
		t.Error("after the swap, the mock's numbers must no longer resolve")
	}
	u, found := s.Lookup("+4599887766")
	if !found || u.ID != "real-1" {
		t.Errorf("want the projection-backed user, got %+v/%v", u, found)
	}
}

// The swap happens on a background goroutine while handlers are already serving, so the
// race detector needs to see this pattern exercised.
func TestSwitchableDirectoryIsSafeUnderConcurrentSwap(t *testing.T) {
	s := newSwitchableDirectory(users.NewMockDirectory())
	real := newPersonDirectory(fakeQueries{}, "2026", quietLogger())

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				s.Lookup("+4530112233")
				s.LookupAll("+4530112233")
				s.Get("p1")
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 50; j++ {
			s.set(real)
		}
	}()
	wg.Wait()
}
