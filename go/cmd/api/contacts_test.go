package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nathejk/shared-go/types"

	"nathejk.dk/internal/data"
	"nathejk.dk/internal/scans"
	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/person"
)

// contactsTestApp wires an app whose person projection returns the given people.
func contactsTestApp(t *testing.T, people []person.Person) (*application, *stubPeople) {
	t.Helper()
	stub := &stubPeople{listed: people}
	return newTestAppWithPeople(t, stub), stub
}

// newTestAppWithPeople wires an app around an already-built person stub, for tests that need
// to seed more than the listable directory (a patrol, a single person by id).
func newTestAppWithPeople(t *testing.T, stub *stubPeople) *application {
	t.Helper()
	app := newTestApp(t)
	app.config.eventYear = "2026"
	app.models = data.NewModels(users.NewMockDirectory(), scans.NewMockSource(), nil, stub)
	// Same TTL as production (main.go). Tests that care about propagation rather than
	// caching replace this with newVersionCache(0).
	app.contactsVersions = newVersionCache(5 * time.Second)
	return app
}

// The directory rows below mirror the mock directory's ids so a session resolves to the
// right viewer. The mock is phone-keyed; these are what the projection would return.
//
// The klan ids come from the mock's exported constants rather than being invented here:
// own-group marking compares the viewer's klan to the subject's, so a fixture with its own
// id would silently test the "different klan" path while claiming to test the same one.
func banditRow() person.Person {
	return person.Person{
		PersonID: "p-bandit", AppRole: string(users.RoleBandit), Name: "Bo Bandit",
		Phone: "+4530000002", TeamID: users.MockBanditPatrolID, TeamName: "Banditgruppe Nord",
		MemberStatus: string(types.MemberStatusRacing), PortraitThumbRef: "thumb-abc",
	}
}

func goeglerRow() person.Person {
	return person.Person{
		PersonID: "p-goegler", AppRole: string(users.RoleGoegler), Name: "Gørn Gøgler",
		Phone: "+4530000006", MemberStatus: string(types.MemberStatusRacing),
	}
}

func crewRow() person.Person {
	return person.Person{
		PersonID: "p-crew", AppRole: string(users.RoleSamarit), Name: "Sara Samarit",
		Phone: "+4530000005", SectionSlug: "samarit", SectionName: "Samaritter",
		MemberStatus: string(types.MemberStatusRacing),
	}
}

func crewBanditRow() person.Person {
	return person.Person{
		PersonID: "p-crew-bandit", AppRole: string(users.RoleCrew), Name: "Kim Krew",
		Phone: "+4530000007", SectionSlug: "bandit", SectionName: "Banditter",
		TeamID: users.MockBanditPatrolID, TeamName: "Banditgruppe Nord",
		MemberStatus: string(types.MemberStatusRacing),
	}
}

func spejderRow() person.Person {
	guardian := "+4520000001"
	return person.Person{
		PersonID: "p-spejder", AppRole: string(users.RoleSpejder), Name: "Signe Spejder",
		Phone: "+4530000001", PhoneParent: &guardian,
		TeamID: "patrol-138", TeamName: "Patrulje 138",
		MemberStatus: string(types.MemberStatusRacing),
	}
}

func fetchManifest(t *testing.T, app *application, srv *httptest.Server, phone, normalized string) (*http.Response, contactsManifest) {
	t.Helper()
	cookies := authedCookies(t, app, srv, phone, normalized)
	resp := getWithCookies(t, srv.URL+"/api/contacts/manifest", cookies)

	var m contactsManifest
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(body, &m); err != nil {
			t.Fatalf("decoding manifest: %v\nbody: %s", err, body)
		}
	}
	return resp, m
}

func TestContactsManifest_RequiresAuth(t *testing.T) {
	app, _ := contactsTestApp(t, nil)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/contacts/manifest")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

// A spejder must not reach the pane at all. The refusal lives in the handler, not only in
// the frontend's nav gating — a hidden menu item is not access control.
func TestContactsManifest_RefusesSpejder(t *testing.T) {
	app, _ := contactsTestApp(t, []person.Person{banditRow(), crewRow()})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, _ := fetchManifest(t, app, srv, "30000001", "+4530000001")
	if resp.StatusCode == http.StatusOK {
		t.Fatal("a spejder was served the contacts manifest")
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

// Clearing a field must remove it from the payload, not leave the old value behind. A
// device that already synced replaces the record, which is what makes the withdrawal purge
// in task 160 real rather than decorative.
func TestContactsManifest_ClearedFieldDisappears(t *testing.T) {
	app, stub := contactsTestApp(t, []person.Person{banditRow()})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, before := fetchManifest(t, app, srv, "30000002", "+4530000002")
	if len(before.Entries) == 0 || before.Entries[0].Phone == "" {
		t.Fatal("expected a phone number in the first sync")
	}

	cleared := banditRow()
	cleared.Phone = ""
	stub.listed = []person.Person{cleared}

	_, after := fetchManifest(t, app, srv, "30000002", "+4530000002")
	if len(after.Entries) == 0 {
		t.Fatal("the person disappeared entirely")
	}
	if after.Entries[0].Phone != "" {
		t.Errorf("a cleared phone number survived: %q", after.Entries[0].Phone)
	}
	if before.Version == after.Version {
		t.Error("the version did not change, so a device would never refetch and would keep the old number")
	}
}

// The single most important property: no spejder in the payload, whoever asks.
func TestContactsManifest_NeverListsSpejdere(t *testing.T) {
	people := []person.Person{spejderRow(), banditRow(), goeglerRow(), crewRow()}

	for _, viewer := range []struct{ name, phone, normalized string }{
		{"bandit", "30000002", "+4530000002"},
		{"gøgler", "30000006", "+4530000006"},
		{"samarit", "30000005", "+4530000005"},
		{"crew", "30000007", "+4530000007"},
	} {
		t.Run(viewer.name, func(t *testing.T) {
			app, _ := contactsTestApp(t, people)
			srv := httptest.NewServer(app.routes())
			defer srv.Close()

			resp, m := fetchManifest(t, app, srv, viewer.phone, viewer.normalized)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}
			for _, e := range m.Entries {
				if e.ID == "p-spejder" || e.Population == string(users.PopulationSpejder) {
					t.Errorf("%s was served a spejder: %+v", viewer.name, e)
				}
			}
		})
	}
}

// The query must not even fetch spejder rows: the payload is scoped, not filtered.
func TestContactsManifest_DoesNotQuerySpejdere(t *testing.T) {
	app, stub := contactsTestApp(t, []person.Person{banditRow()})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	fetchManifest(t, app, srv, "30000005", "+4530000005")

	if len(stub.listedRoles) == 0 {
		t.Fatal("ListByAppRoles was never called")
	}
	for _, roles := range stub.listedRoles {
		for _, r := range roles {
			if r == string(users.RoleSpejder) {
				t.Error("the manifest query asked for spejder rows")
			}
		}
	}
}

func TestContactsManifest_BanditSeesOwnKlanAndCrew(t *testing.T) {
	app, _ := contactsTestApp(t, []person.Person{banditRow(), goeglerRow(), crewRow()})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, m := fetchManifest(t, app, srv, "30000002", "+4530000002")

	var sawBandit, sawCrew bool
	for _, e := range m.Entries {
		switch e.Population {
		case string(users.PopulationBandit):
			sawBandit = true
			if len(e.Groups) != 1 || e.Groups[0].Label != "Banditgruppe Nord" {
				t.Errorf("bandit not grouped by klan: %+v", e.Groups)
			}
			if !e.Groups[0].IsOwn {
				t.Error("the viewer's own klan is not marked own")
			}
		case string(users.PopulationCrew):
			sawCrew = true
		case string(users.PopulationGoegler):
			t.Error("a bandit was served the gøgler population")
		}
	}
	if !sawBandit || !sawCrew {
		t.Errorf("bandit should see banditter and crew; got bandit=%v crew=%v", sawBandit, sawCrew)
	}
}

// A crew bandit appears twice for a crew viewer — once among banditter, once among crew.
// Both lists answer "who is out as what", so this is intended, not a de-duplication bug.
func TestContactsManifest_CrewBanditAppearsInBothLists(t *testing.T) {
	app, _ := contactsTestApp(t, []person.Person{crewBanditRow()})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, m := fetchManifest(t, app, srv, "30000005", "+4530000005")

	pops := map[string]bool{}
	for _, e := range m.Entries {
		if e.ID == "p-crew-bandit" {
			pops[e.Population] = true
		}
	}
	if !pops[string(users.PopulationBandit)] || !pops[string(users.PopulationCrew)] {
		t.Errorf("crew bandit should be listed in both populations, got %v", pops)
	}
}

// Guardian numbers must not appear anywhere in the payload. `.rules` invariant; the fuller
// tripwire across every contacts surface is task 159.
func TestContactsManifest_NeverCarriesGuardianNumber(t *testing.T) {
	// A crew row given a guardian number it should never have, to prove the absence is
	// structural rather than a property of the fixtures.
	guardian := "+4520000009"
	row := crewRow()
	row.PhoneParent = &guardian

	app, _ := contactsTestApp(t, []person.Person{row})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000005", "+4530000005")
	resp := getWithCookies(t, srv.URL+"/api/contacts/manifest", cookies)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if strings.Contains(string(body), guardian) {
		t.Error("a guardian number reached the contacts manifest")
	}
	for _, key := range []string{"phoneParent", "phone_parent", "address", "postalCode", "city"} {
		if strings.Contains(string(body), key) {
			t.Errorf("the manifest carries %q, which is not on the allow-list", key)
		}
	}
}

// A withdrawn member keeps their name and portrait and loses their number.
func TestContactsManifest_WithdrawnMemberLosesPhone(t *testing.T) {
	row := banditRow()
	row.MemberStatus = string(types.MemberStatusReleased)

	app, _ := contactsTestApp(t, []person.Person{row})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, m := fetchManifest(t, app, srv, "30000002", "+4530000002")

	if len(m.Entries) == 0 {
		t.Fatal("a withdrawn member vanished from the directory; they must stay visible")
	}
	e := m.Entries[0]
	if e.StillInRace {
		t.Error("a released member is marked as still in the race")
	}
	if e.Phone != "" {
		t.Errorf("a withdrawn member's number was served: %q", e.Phone)
	}
	if e.Name == "" || e.PortraitVersion == "" {
		t.Error("a withdrawn member lost their name or portrait; both must be kept until end of race")
	}
}

func TestContactsManifest_FinishedIsNotAWithdrawal(t *testing.T) {
	row := banditRow()
	row.MemberStatus = string(types.MemberStatusFinished)

	app, _ := contactsTestApp(t, []person.Person{row})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, m := fetchManifest(t, app, srv, "30000002", "+4530000002")
	if len(m.Entries) == 0 {
		t.Fatal("no entries")
	}
	if !m.Entries[0].StillInRace {
		t.Error("a finisher was marked as having left the race; finishing is not a withdrawal")
	}
}

func TestContactsManifest_ETagAndNotModified(t *testing.T) {
	app, _ := contactsTestApp(t, []person.Person{banditRow(), crewRow()})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000002", "+4530000002")
	first := getWithCookies(t, srv.URL+"/api/contacts/manifest", cookies)
	io.Copy(io.Discard, first.Body)
	first.Body.Close()

	etag := first.Header.Get("ETag")
	if etag == "" {
		t.Fatal("no ETag on the manifest; the freshness poll depends on it")
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/contacts/manifest", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	req.Header.Set("If-None-Match", etag)
	second, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	io.Copy(io.Discard, second.Body)
	second.Body.Close()

	if second.StatusCode != http.StatusNotModified {
		t.Errorf("status = %d, want 304", second.StatusCode)
	}
}

// The version must change when the directory changes, or devices never refetch.
func TestContactsManifest_VersionTracksContent(t *testing.T) {
	app, stub := contactsTestApp(t, []person.Person{banditRow()})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, before := fetchManifest(t, app, srv, "30000002", "+4530000002")

	changed := banditRow()
	changed.Phone = "+4599999999"
	stub.listed = []person.Person{changed}

	_, after := fetchManifest(t, app, srv, "30000002", "+4530000002")

	if before.Version == after.Version {
		t.Error("the version did not change when a phone number did")
	}
}

// Two viewers with different permitted sets must get different versions, so one person's
// edit does not invalidate everybody's cache.
func TestContactsManifest_VersionIsPerViewer(t *testing.T) {
	people := []person.Person{banditRow(), goeglerRow(), crewRow()}

	app, _ := contactsTestApp(t, people)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, bandit := fetchManifest(t, app, srv, "30000002", "+4530000002")
	_, crew := fetchManifest(t, app, srv, "30000005", "+4530000005")

	if bandit.Version == crew.Version {
		t.Error("a bandit and a crew member got the same version despite different permitted sets")
	}
}

// A run without a database must degrade to an empty directory rather than an error the
// client cannot act on.
func TestContactsManifest_NoProjection(t *testing.T) {
	app := newTestApp(t)
	app.config.eventYear = "2026"
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, m := fetchManifest(t, app, srv, "30000005", "+4530000005")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(m.Entries) != 0 {
		t.Errorf("want no entries, got %d", len(m.Entries))
	}
	if m.Version == "" {
		t.Error("even an empty directory needs a version, so the client can poll")
	}
}
