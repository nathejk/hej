package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"nathejk.dk/nathejk/table/person"
)

// The crew roster behind the "Fotokredit" picker (PRD 025 §6 R2, task 449).
//
// The first test is the one the task exists for. The rest are the endpoint's behaviour.

// crewPeople is a person.Queries that answers the roster read and records what it was asked.
//
// Only `CrewRoster` has any behaviour, which is itself a small assertion: if the handler ever reaches for
// `Get` to enrich an entry it gets nothing, and the response type could not hold the result anyway — see the
// first test.
type crewPeople struct {
	roster   []person.CrewMember
	err      error
	askedFor []string // the section slugs, in order
	years    []string
}

func (c *crewPeople) CrewRoster(year, sectionSlug string) ([]person.CrewMember, error) {
	c.years = append(c.years, year)
	c.askedFor = append(c.askedFor, sectionSlug)
	return c.roster, c.err
}

// The rest of person.Queries, unused here.
func (c *crewPeople) Lookup(string, string) ([]person.Person, error) { return nil, nil }
func (c *crewPeople) Get(string, string) (person.Person, bool, error) {
	return person.Person{}, false, nil
}
func (c *crewPeople) ListByAppRoles(string, []string) ([]person.Person, error)   { return nil, nil }
func (c *crewPeople) ListPatrolByNumber(string, string) ([]person.Person, error) { return nil, nil }
func (c *crewPeople) TrackMembers(string, string) ([]person.TrackMember, error)  { return nil, nil }
func (c *crewPeople) ExpiredPortraits(string, time.Time, int) ([]person.ExpiredPortrait, error) {
	return nil, nil
}

// crewApp is the admin app with a person projection wired in.
func crewApp(t *testing.T, people person.Queries) *httptest.Server {
	t.Helper()

	app, srv := adminApp(t)
	app.models.People = people
	return srv
}

// **The guard this task is mostly about.**
//
// The person row holds a telephone number, a guardian's telephone number, an email, a postal address and a
// birthday. A read that returns "the crew member" is one somebody later renders more of, so the response type
// is pinned exactly: an id and a name, with those JSON names. Adding a field fails here, which makes it a
// deliberate act with a sentence attached rather than a convenience.
//
// Note what the existing walk covers and what it does not.
// `TestNoStructInTheLibraryOrTheAdminToolNamesAPerson` enumerates every struct declared in an `admin*.go`
// file, so this one is walked: a `Phone`, `PhoneParent`, `Email`, `Address`, `Birthday` or `Portrait…` here
// turns that test red without anybody touching this file. What it cannot catch is a field whose *name* is
// innocent — `Nickname`, `Born`, `Contact` — because `isPersonShaped` judges names and a bare one is
// ambiguous by design. Hence an exact field set rather than a needle list.
func TestTheCrewRosterCarriesAnIdAndANameAndNothingElse(t *testing.T) {
	typ := reflect.TypeOf(adminCrewMember{})
	if typ.NumField() == 0 {
		t.Fatal("adminCrewMember has no fields: the reflection is broken, which would make this pass while " +
			"asserting nothing")
	}

	var got []string
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		got = append(got, f.Name+" "+f.Tag.Get("json"))
	}
	sort.Strings(got)

	want := []string{"ID id", "Name name"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("adminCrewMember is %v, want exactly %v.\n\nThis read carries a crew member's id and their "+
			"name. Nothing else: the person row it comes from holds a telephone number, a guardian's "+
			"telephone number, an email, a postal address and a birthday, and a guardian's number may exist "+
			"on exactly one surface in this project (.rules) — which is not a picker for photograph credits. "+
			"If the picker genuinely needs another field, say why here and in person.CrewRoster, which must "+
			"also stop selecting two columns", got, want)
	}

	// And the projection it is built from is the same two fields, so neither can be widened alone.
	src := reflect.TypeOf(person.CrewMember{})
	if src.NumField() != 2 {
		t.Errorf("person.CrewMember has %d fields: the handler's narrowness is worth little if the value it "+
			"is built from already carries more", src.NumField())
	}
}

// The default is `pr`, where the photographers are, and it is asked for as a section rather than assumed.
func TestTheCrewRosterDefaultsToPr(t *testing.T) {
	people := &crewPeople{roster: []person.CrewMember{
		{PersonID: "user-1", Name: "Anne Sørensen"},
		{PersonID: "user-2", Name: "Bo Nielsen"},
	}}
	srv := crewApp(t, people)

	var out adminCrewResponse
	decodeAdminJSON(t, getAdmin(t, srv, "/api/admin/crew", testAdminUser, testAdminPass), &out)

	if len(out.Crew) != 2 || out.Crew[0].ID != "user-1" || out.Crew[0].Name != "Anne Sørensen" {
		t.Fatalf("unexpected roster: %+v", out.Crew)
	}
	if out.Section != adminCrewDefaultSection {
		t.Errorf("Section = %q, want the default %q echoed back: a list of names with no statement of what "+
			"it was narrowed to reads as \"this is the crew\"", out.Section, adminCrewDefaultSection)
	}
	if !reflect.DeepEqual(people.askedFor, []string{"pr"}) {
		t.Errorf("asked for %v, want the pr section", people.askedFor)
	}
	if !reflect.DeepEqual(people.years, []string{"2026"}) {
		t.Errorf("read year %v, want the year on the request: nothing in this service crosses a year",
			people.years)
	}
}

// `pr` is the default filter, not the boundary (PRD 025 §5): a photographer in another section is reachable.
func TestTheCrewRosterCanReturnTheWholeYearsCrew(t *testing.T) {
	people := &crewPeople{}
	srv := crewApp(t, people)

	var out adminCrewResponse
	decodeAdminJSON(t, getAdmin(t, srv, "/api/admin/crew?section=all", testAdminUser, testAdminPass), &out)

	if !reflect.DeepEqual(people.askedFor, []string{""}) {
		t.Errorf("asked for %v, want an empty slug — the projection reads that as the whole year's crew",
			people.askedFor)
	}
	if out.Section != adminCrewAllSections {
		t.Errorf("Section = %q, want %q echoed back", out.Section, adminCrewAllSections)
	}
	// An empty roster is an empty array rather than null, so the picker renders "nobody" rather than failing
	// to iterate.
	if out.Crew == nil {
		t.Error("Crew must be an empty array, not null")
	}
}

// Any other value narrows to that section, unfolded — the projection owns the folding
// (`NormalizeSectionSlug`), which is what keeps one implementation of it.
func TestTheCrewRosterPassesASectionThrough(t *testing.T) {
	for _, asked := range []string{"hq", "koekken", " PR ", "en-sektion-der-ikke-findes"} {
		people := &crewPeople{}
		srv := crewApp(t, people)

		resp := getAdmin(t, srv, "/api/admin/crew?section="+url.QueryEscape(asked),
			testAdminUser, testAdminPass)
		var out adminCrewResponse
		decodeAdminJSON(t, resp, &out)

		// Trimmed, because surrounding whitespace in a query string is noise; not otherwise folded.
		want := strings.TrimSpace(asked)
		if !reflect.DeepEqual(people.askedFor, []string{want}) {
			t.Errorf("section=%q asked the projection for %v, want %q", asked, people.askedFor, want)
		}
		// An unknown section is an empty roster, not a 400: there is no list of valid sections to check
		// against — organizers author and rename them freely — so the only available refusal would be
		// "this section has no crew", which is a legitimate answer.
		if len(out.Crew) != 0 {
			t.Errorf("section=%q returned %d entries from an empty projection", asked, len(out.Crew))
		}
	}
}

// Behind the credential, like every other read on this surface.
func TestTheCrewRosterNeedsTheAdminCredential(t *testing.T) {
	srv := crewApp(t, &crewPeople{roster: []person.CrewMember{
		{PersonID: "user-1", Name: "Anne Sørensen"},
	}})

	resp := getAdmin(t, srv, "/api/admin/crew", "", "")
	body := adminBody(t, resp)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401 with no credential, got %d", resp.StatusCode)
	}
	if strings.Contains(body, "Anne") {
		t.Error("the refusal leaked a crew member's name")
	}
}

// `no-store`, inherited from `requireAdmin` rather than set here — a roster of the event's crew left in a
// shared laptop's disk cache outlives the session that fetched it.
func TestTheCrewRosterIsNotCacheable(t *testing.T) {
	srv := crewApp(t, &crewPeople{})

	resp := getAdmin(t, srv, "/api/admin/crew", testAdminUser, testAdminPass)
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	if got := resp.Header.Get("X-Robots-Tag"); !strings.Contains(got, "noindex") {
		t.Errorf("X-Robots-Tag = %q, want noindex", got)
	}
}

// No projection is 503, not an empty list. "There is no crew this year" and "this tool cannot see the crew"
// look the same in a picker, and the curator's next move differs.
func TestTheCrewRosterSaysWhenTheRosterIsUnavailable(t *testing.T) {
	srv := crewApp(t, nil)

	resp := getAdmin(t, srv, "/api/admin/crew", testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("want 503 with no person projection, got %d", resp.StatusCode)
	}
}

// A failing read is a 500, not an empty roster: a picker that silently offers nobody sends the curator back
// to typing, which is what this feature exists to stop.
func TestTheCrewRosterSurfacesAReadFailure(t *testing.T) {
	srv := crewApp(t, &crewPeople{err: errors.New("database gone")})

	resp := getAdmin(t, srv, "/api/admin/crew", testAdminUser, testAdminPass)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500 when the roster read fails, got %d", resp.StatusCode)
	}
}

// decodeAdminJSON drains a JSON response and fails loudly if it is not a 200 of the expected shape.
func decodeAdminJSON(t *testing.T, resp *http.Response, into any) {
	t.Helper()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, adminBody(t, resp))
	}
	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		t.Fatalf("decoding the response: %v", err)
	}
}
