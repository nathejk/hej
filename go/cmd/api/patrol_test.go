package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nathejk/shared-go/types"

	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/person"
)

// patrolApp wires an app whose projection answers one patrol number.
func patrolApp(t *testing.T, number string, members []person.Person) (*application, *stubPeople) {
	t.Helper()
	stub := &stubPeople{patrol: map[string][]person.Person{number: members}}
	app := newTestAppWithPeople(t, stub)
	app.contactsVersions = newVersionCache(0)
	return app, stub
}

func patrolMembers() []person.Person {
	guardian := "+4520000001"
	return []person.Person{
		{
			PersonID: "p-freja", AppRole: string(users.RoleSpejder), Name: "Freja Mikkelsen",
			Phone: "+4530001111", PhoneParent: &guardian,
			TeamID: "team-138", TeamName: "Patrulje Ravnene", TeamNumber: "138",
			MemberStatus: string(types.MemberStatusRacing), PortraitRef: "abc", PortraitThumbRef: "abc",
		},
		{
			PersonID: "p-villads", AppRole: string(users.RoleSpejder), Name: "Villads Mikkelsen",
			Phone: "+4530002222", PhoneParent: &guardian,
			TeamID: "team-138", TeamName: "Patrulje Ravnene", TeamNumber: "138",
			MemberStatus: string(types.MemberStatusWaiting),
		},
	}
}

func lookupPatrol(t *testing.T, app *application, srv *httptest.Server, phone, normalized, number string) *http.Response {
	t.Helper()
	cookies := authedCookies(t, app, srv, phone, normalized)
	return getWithCookies(t, srv.URL+"/api/contacts/patrols/"+number, cookies)
}

func TestPatrolLookup_CrewSeesMembersWithStatusAndPhone(t *testing.T) {
	app, _ := patrolApp(t, "138", patrolMembers())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp := lookupPatrol(t, app, srv, "30000005", "+4530000005", "138")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", resp.StatusCode, body)
	}

	var out patrolLookupResponse
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if out.Number != "138" {
		t.Errorf("number = %q, want 138", out.Number)
	}
	if len(out.Members) != 2 {
		t.Fatalf("want 2 members, got %d", len(out.Members))
	}

	freja := out.Members[0]
	if freja.Name != "Freja Mikkelsen" || freja.Phone != "+4530001111" {
		t.Errorf("unexpected member %+v", freja)
	}
	if freja.Status != string(types.MemberStatusRacing) {
		t.Errorf("status = %q, want racing", freja.Status)
	}
	if !freja.HasPortrait {
		t.Error("hasPortrait should be true for a member with a portrait")
	}
	if out.Members[1].HasPortrait {
		t.Error("hasPortrait should be false for a member without one")
	}
	// The full lifecycle status, not the directory's single bit.
	if out.Members[1].Status != string(types.MemberStatusWaiting) {
		t.Errorf("status = %q, want waiting", out.Members[1].Status)
	}
}

// The single most important header in this feature: nothing from a lookup may be stored.
func TestPatrolLookup_IsNeverCacheable(t *testing.T) {
	app, _ := patrolApp(t, "138", patrolMembers())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	// Success, refusal and absence must all be no-store — a cacheable refusal is still a
	// response we would rather not have sitting in a proxy.
	cases := []struct{ name, phone, normalized, number string }{
		{"success", "30000005", "+4530000005", "138"},
		{"refused", "30000002", "+4530000002", "138"},
		{"absent", "30000005", "+4530000005", "999"},
		{"spejder", "30000001", "+4530000001", "138"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := lookupPatrol(t, app, srv, tc.phone, tc.normalized, tc.number)
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			if got := resp.Header.Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", got)
			}
			if resp.Header.Get("ETag") != "" {
				t.Error("an ETag invites conditional caching of a response that must not be stored")
			}
		})
	}
}

// Refusal and absence must be indistinguishable, or the endpoint maps the patrol numbering.
func TestPatrolLookup_RefusalIsIndistinguishableFromAbsence(t *testing.T) {
	app, _ := patrolApp(t, "138", patrolMembers())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	// A bandit asking for a patrol that exists.
	refused := lookupPatrol(t, app, srv, "30000002", "+4530000002", "138")
	refusedBody, _ := io.ReadAll(refused.Body)
	refused.Body.Close()

	// Crew asking for a patrol that does not.
	absent := lookupPatrol(t, app, srv, "30000005", "+4530000005", "999")
	absentBody, _ := io.ReadAll(absent.Body)
	absent.Body.Close()

	if refused.StatusCode != absent.StatusCode {
		t.Errorf("refusal %d differs from absence %d — the endpoint reveals which patrols exist",
			refused.StatusCode, absent.StatusCode)
	}
	if refused.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for both", refused.StatusCode)
	}
	if !bytes.Equal(refusedBody, absentBody) {
		t.Errorf("bodies differ:\nrefused: %s\nabsent:  %s", refusedBody, absentBody)
	}
}

// Only crew, and every crew role including the unclassified fallback.
func TestPatrolLookup_CrewOnly(t *testing.T) {
	app, _ := patrolApp(t, "138", patrolMembers())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	allowed := []struct{ name, phone, normalized string }{
		{"samarit", "30000005", "+4530000005"},
		{"guide", "30000004", "+4530000004"},
		{"postmandskab", "30000003", "+4530000003"},
		{"crew fallback", "30000007", "+4530000007"},
	}
	for _, tc := range allowed {
		t.Run(tc.name, func(t *testing.T) {
			resp := lookupPatrol(t, app, srv, tc.phone, tc.normalized, "138")
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("status = %d, want 200 — all crew may look patrols up", resp.StatusCode)
			}
		})
	}

	refused := []struct{ name, phone, normalized string }{
		{"bandit", "30000002", "+4530000002"},
		{"gøgler", "30000006", "+4530000006"},
		{"spejder", "30000001", "+4530000001"},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			resp := lookupPatrol(t, app, srv, tc.phone, tc.normalized, "138")
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				t.Fatalf("%s was allowed to look up a patrol", tc.name)
			}
			if bytes.Contains(body, []byte("Freja")) {
				t.Error("member data leaked into a refusal")
			}
		})
	}
}

func TestPatrolLookup_KeepsPhoneUntilReleased(t *testing.T) {
	// The samarit case: a member waiting by the trail or in a car is exactly who needs ringing,
	// so only `released` — handed to a guardian and off site — removes the number.
	for _, status := range []types.MemberStatus{
		types.MemberStatusWaiting,
		types.MemberStatusTransit,
		types.MemberStatusSheltered,
		types.MemberStatusReunited,
	} {
		t.Run(string(status), func(t *testing.T) {
			members := patrolMembers()
			members[0].MemberStatus = string(status)

			app, _ := patrolApp(t, "138", members)
			srv := httptest.NewServer(app.routes())
			defer srv.Close()

			resp := lookupPatrol(t, app, srv, "30000005", "+4530000005", "138")
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			var out patrolLookupResponse
			if err := json.Unmarshal(body, &out); err != nil {
				t.Fatalf("decoding: %v", err)
			}
			if out.Members[0].Phone == "" {
				t.Errorf("status %q lost the number; only released should", status)
			}
		})
	}
}

func TestPatrolLookup_ReleasedMemberLosesPhone(t *testing.T) {
	members := patrolMembers()
	members[0].MemberStatus = string(types.MemberStatusReleased)

	app, _ := patrolApp(t, "138", members)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp := lookupPatrol(t, app, srv, "30000005", "+4530000005", "138")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var out patrolLookupResponse
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if out.Members[0].Phone != "" {
		t.Errorf("a released member's number was served: %q", out.Members[0].Phone)
	}
	// Still listed, with their status: a samarit looking for them should learn they were
	// collected, not find a gap in the patrol.
	if out.Members[0].Name == "" || out.Members[0].Status != string(types.MemberStatusReleased) {
		t.Errorf("a released member must stay listed with their status: %+v", out.Members[0])
	}
}

// Guardian numbers are exactly what these records carry, and exactly what must not ship.
func TestPatrolLookup_NeverCarriesGuardianNumber(t *testing.T) {
	app, _ := patrolApp(t, "138", patrolMembers())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp := lookupPatrol(t, app, srv, "30000005", "+4530000005", "138")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if strings.Contains(string(body), "+4520000001") {
		t.Error("a guardian number reached the patrol lookup response")
	}
	for _, key := range []string{"phoneParent", "phone_parent", "address", "postalCode"} {
		if strings.Contains(string(body), key) {
			t.Errorf("the lookup carries %q", key)
		}
	}
}

// Exact match only: the query must receive exactly what was typed, and a prefix must not
// resolve. Prefix matching would turn one permitted question into an enumeration tool.
func TestPatrolLookup_ExactMatchOnly(t *testing.T) {
	app, stub := patrolApp(t, "138", patrolMembers())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	for _, number := range []string{"13", "1", "1380", "138 "} {
		resp := lookupPatrol(t, app, srv, "30000005", "+4530000005", number)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		// "138 " trims to "138" and legitimately resolves; the rest must not.
		if strings.TrimSpace(number) == "138" {
			if resp.StatusCode != http.StatusOK {
				t.Errorf("%q should resolve after trimming, got %d", number, resp.StatusCode)
			}
			continue
		}
		if resp.StatusCode == http.StatusOK {
			t.Errorf("partial number %q resolved to a patrol", number)
		}
	}

	for _, asked := range stub.patrolAsked {
		if asked != strings.TrimSpace(asked) || asked == "" {
			t.Errorf("the query was asked for %q; it must receive a trimmed, non-empty number", asked)
		}
	}
}

func TestPatrolLookup_RequiresAuth(t *testing.T) {
	app, _ := patrolApp(t, "138", patrolMembers())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/contacts/patrols/138")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

// The patrol photo route must be reachable only in the context of the right patrol: a
// person id alone must not fetch a face.
func TestPatrolPhoto_ScopedToThePatrol(t *testing.T) {
	members := patrolMembers()

	app, stub := patrolApp(t, "138", members)
	ref, err := app.blobs.Put(t.Context(), []byte("frejas-face"))
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	members[0].PortraitRef = string(ref)
	members[0].PortraitThumbRef = string(ref)
	stub.patrol = map[string][]person.Person{"138": members}

	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000005", "+4530000005")

	ok := getWithCookies(t, srv.URL+"/api/contacts/patrols/138/photo/p-freja", cookies)
	body, _ := io.ReadAll(ok.Body)
	ok.Body.Close()
	if ok.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", ok.StatusCode)
	}
	if !bytes.Equal(body, []byte("frejas-face")) {
		t.Error("served bytes differ from the stored portrait")
	}
	if got := ok.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store — this is the header that keeps spejder faces off devices", got)
	}

	// The same person under the wrong patrol number must not resolve.
	wrong := getWithCookies(t, srv.URL+"/api/contacts/patrols/999/photo/p-freja", cookies)
	io.Copy(io.Discard, wrong.Body)
	wrong.Body.Close()
	if wrong.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for a person outside the named patrol", wrong.StatusCode)
	}

	// And a non-crew caller gets nothing.
	banditCookies := authedCookies(t, app, srv, "30000002", "+4530000002")
	refused := getWithCookies(t, srv.URL+"/api/contacts/patrols/138/photo/p-freja", banditCookies)
	refusedBody, _ := io.ReadAll(refused.Body)
	refused.Body.Close()
	if refused.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", refused.StatusCode)
	}
	if bytes.Contains(refusedBody, []byte("frejas-face")) {
		t.Error("portrait bytes leaked to a bandit")
	}
}

// A member with no portrait is a normal state, not an error.
func TestPatrolPhoto_NoPortraitIsNotFound(t *testing.T) {
	app, _ := patrolApp(t, "138", patrolMembers())
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000005", "+4530000005")
	resp := getWithCookies(t, srv.URL+"/api/contacts/patrols/138/photo/p-villads", cookies)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}
