package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"nathejk.dk/nathejk/table/person"
)

// The guardian-number tripwire (task 159).
//
// `.rules` carries a repo-wide invariant: guardian phone numbers never enter the PWA, with
// exactly one exception — a member confirming or approving *their own*. No contacts surface
// may carry one.
//
// This file is test-only, and it is what makes that rule durable. `PhoneParent` is present on
// both `person.Person` and `users.User`, so "we just do not select it" is one careless change
// away from being false: somebody widens a response struct, swaps a projection for a broader
// one, or adds a field for a good reason, and a guardian's number ships to a few hundred
// devices. The people those numbers belong to are not users of this app and never agreed to
// be in it.
//
// Two layers, because they fail differently:
//
//   - **Body scans** catch a field that is populated. They only work if the fixture actually
//     has a guardian number, which is asserted separately so the test cannot go vacuous.
//   - **Type reflection** catches a field that has been *added* but is empty in the fixture —
//     the more likely accident, and the one a body scan would sail past.

// guardianNumber is deliberately distinctive so a body scan cannot match it by chance.
const guardianNumber = "+4520999888"

// personWithGuardian returns a row that definitely carries a guardian number, for whichever
// population the caller needs.
func personWithGuardian(base person.Person) person.Person {
	g := guardianNumber
	base.PhoneParent = &g
	return base
}

// TestFixturesActuallyCarryAGuardianNumber keeps the scans below honest. Without this, a
// change that stopped populating the fixture would make every assertion in this file pass
// while testing nothing.
func TestFixturesActuallyCarryAGuardianNumber(t *testing.T) {
	for name, p := range map[string]person.Person{
		"crew":    personWithGuardian(crewRow()),
		"bandit":  personWithGuardian(banditRow()),
		"spejder": patrolMembers()[0],
	} {
		if p.PhoneParent == nil || *p.PhoneParent == "" {
			t.Errorf("fixture %q has no guardian number, so the tripwire would be vacuous", name)
		}
	}
}

// forbiddenKeys are the JSON keys that must never appear in a contacts response, and the
// substrings a new field name must not contain.
var forbiddenKeys = []string{
	"phoneparent", "phone_parent", "guardian", "parentphone", "parent_phone",
	// Postal address is not a `.rules` invariant, but it is excluded from the contacts
	// allow-list by PRD 007 §11.4 and belongs in the same tripwire.
	"address", "postalcode", "postal_code",
}

// TestContactsSurfacesNeverCarryAGuardianNumber scans the marshalled body of every JSON
// contacts surface.
//
// Written against the bytes rather than the Go struct on purpose: an embedded struct, a
// renamed field or a `json:"-"` that gets removed would all pass a struct-level check and
// still put the number on the wire.
func TestContactsSurfacesNeverCarryAGuardianNumber(t *testing.T) {
	// A patrol whose members all have guardian numbers — these are the records that
	// actually have one, so the patrol lookup is the surface that matters most here.
	patrol := patrolMembers()

	directoryPeople := []person.Person{
		personWithGuardian(crewRow()),
		personWithGuardian(banditRow()),
		personWithGuardian(goeglerRow()),
		personWithGuardian(crewBanditRow()),
	}

	stub := &stubPeople{
		listed: directoryPeople,
		patrol: map[string][]person.Person{"138": patrol},
		p:      personWithGuardian(crewRow()),
		found:  true,
	}
	app := newTestAppWithPeople(t, stub)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	// Every contacts surface that returns JSON, fetched as crew — the role with the widest
	// view, so anything reachable is reachable here.
	//
	// When task 167 adds the person profile route, add it to this list. A surface missing
	// from here is a surface with no tripwire.
	paths := []string{
		"/api/contacts/manifest",
		"/api/contacts/version",
		"/api/contacts/patrols/138",
	}

	cookies := authedCookies(t, app, srv, "30000005", "+4530000005")

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			resp := getWithCookies(t, srv.URL+path, cookies)
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				t.Fatalf("reading body: %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200 (a refused request tests nothing)", resp.StatusCode)
			}

			if strings.Contains(string(body), guardianNumber) {
				t.Errorf("a guardian number reached %s:\n%s", path, body)
			}
			lower := strings.ToLower(string(body))
			for _, key := range forbiddenKeys {
				if strings.Contains(lower, key) {
					t.Errorf("%s carries the forbidden key %q:\n%s", path, key, body)
				}
			}
		})
	}
}

// TestContactsResponseTypesHaveNoGuardianField is the layer that catches a field somebody
// adds but does not populate.
//
// Walks the JSON tags of every contacts response type. If one of these grows a
// `phoneParent` — or an `address`, which PRD 007 also excludes — this fails at compile-time
// speed with a message naming the field, instead of waiting for a fixture that happens to
// populate it.
func TestContactsResponseTypesHaveNoGuardianField(t *testing.T) {
	types := []any{
		contactsManifest{},
		contactEntry{},
		contactGroup{},
		contactsVersionResponse{},
		patrolLookupResponse{},
		patrolLookupMember{},
	}

	for _, v := range types {
		rt := reflect.TypeOf(v)
		t.Run(rt.Name(), func(t *testing.T) {
			assertNoForbiddenFields(t, rt, rt.Name())
		})
	}
}

// assertNoForbiddenFields walks a struct type recursively, checking JSON names and Go field
// names alike. Recursive because a nested or embedded struct is exactly how such a field
// would arrive without anyone noticing.
func assertNoForbiddenFields(t *testing.T, rt reflect.Type, path string) {
	t.Helper()

	for rt.Kind() == reflect.Pointer || rt.Kind() == reflect.Slice || rt.Kind() == reflect.Array {
		rt = rt.Elem()
	}
	if rt.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)

		name := f.Name
		if tag := f.Tag.Get("json"); tag != "" && tag != "-" {
			name = strings.Split(tag, ",")[0]
		}
		lower := strings.ToLower(name)

		for _, forbidden := range forbiddenKeys {
			if strings.Contains(lower, forbidden) {
				t.Errorf("%s.%s exposes %q, which no contacts surface may carry (see .rules and PRD 007 §11.4)",
					path, f.Name, name)
			}
		}

		assertNoForbiddenFields(t, f.Type, path+"."+f.Name)
	}
}

// The image surfaces return bytes, not JSON, so there is no field to leak — but they do take
// a person id, and a 404 body is still a body. Cheap to assert, and it closes the "every
// contacts surface" claim rather than leaving two of them implicitly trusted.
func TestContactsImageSurfacesLeakNothing(t *testing.T) {
	stub := &stubPeople{
		listed: []person.Person{personWithGuardian(crewRow())},
		patrol: map[string][]person.Person{"138": patrolMembers()},
		p:      personWithGuardian(crewRow()),
		found:  true,
	}
	app := newTestAppWithPeople(t, stub)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000005", "+4530000005")

	for _, path := range []string{
		"/api/contacts/people/p-crew/photo",
		"/api/contacts/patrols/138/photo/p-freja",
	} {
		resp := getWithCookies(t, srv.URL+path, cookies)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if strings.Contains(string(body), guardianNumber) {
			t.Errorf("%s leaked a guardian number in its response body", path)
		}
	}
}
