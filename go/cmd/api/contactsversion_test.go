package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nathejk.dk/nathejk/table/person"
)

func fetchVersion(t *testing.T, app *application, srv *httptest.Server, phone, normalized string) (*http.Response, string) {
	t.Helper()
	cookies := authedCookies(t, app, srv, phone, normalized)
	resp := getWithCookies(t, srv.URL+"/api/contacts/version", cookies)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	resp.Body.Close()

	var v contactsVersionResponse
	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(body, &v); err != nil {
			t.Fatalf("decoding version: %v\nbody: %s", err, body)
		}
	}
	return resp, v.Version
}

func TestContactsVersion_RequiresAuthAndRefusesSpejder(t *testing.T) {
	app, _ := contactsTestApp(t, []person.Person{banditRow()})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	anon, err := http.Get(srv.URL + "/api/contacts/version")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	io.Copy(io.Discard, anon.Body)
	anon.Body.Close()
	if anon.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous status = %d, want 401", anon.StatusCode)
	}

	spejder, _ := fetchVersion(t, app, srv, "30000001", "+4530000001")
	if spejder.StatusCode != http.StatusForbidden {
		t.Errorf("spejder status = %d, want 403", spejder.StatusCode)
	}
}

// The version endpoint must agree with the manifest's, or the client refetches forever (or
// never).
func TestContactsVersion_MatchesManifest(t *testing.T) {
	app, _ := contactsTestApp(t, []person.Person{banditRow(), crewRow()})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, manifest := fetchManifest(t, app, srv, "30000002", "+4530000002")
	_, version := fetchVersion(t, app, srv, "30000002", "+4530000002")

	if manifest.Version != version {
		t.Errorf("version endpoint says %q, manifest says %q", version, manifest.Version)
	}
}

func TestContactsVersion_ChangesWithData(t *testing.T) {
	app, stub := contactsTestApp(t, []person.Person{banditRow()})
	// No caching for this test: the point is that data changes propagate, not how long
	// they are allowed to lag.
	app.contactsVersions = newVersionCache(0)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	_, before := fetchVersion(t, app, srv, "30000002", "+4530000002")

	changed := banditRow()
	changed.Name = "Bo Bandit-Jensen"
	stub.listed = []person.Person{changed}

	_, after := fetchVersion(t, app, srv, "30000002", "+4530000002")

	if before == after {
		t.Error("the version did not change when a name did")
	}
}

// Every field the manifest can expose must be part of the version, or an edit to it would
// never reach devices. This is the failure mode that looks like "the app is fine" until
// somebody's corrected number never shows up.
func TestContactsVersion_CoversEveryExposedField(t *testing.T) {
	mutations := map[string]func(*person.Person){
		"name":        func(p *person.Person) { p.Name = "Someone Else" },
		"phone":       func(p *person.Person) { p.Phone = "+4599999999" },
		"status":      func(p *person.Person) { p.MemberStatus = "released" },
		"teamName":    func(p *person.Person) { p.TeamName = "Klan Anden" },
		"teamID":      func(p *person.Person) { p.TeamID = "klan-other" },
		"sectionSlug": func(p *person.Person) { p.SectionSlug = "goeglerledelse" },
		"sectionName": func(p *person.Person) { p.SectionName = "Andet" },
		"portrait":    func(p *person.Person) { p.PortraitThumbRef = "thumb-changed" },
	}

	for field, mutate := range mutations {
		t.Run(field, func(t *testing.T) {
			base := crewBanditRow()
			app, stub := contactsTestApp(t, []person.Person{base})
			app.contactsVersions = newVersionCache(0)
			srv := httptest.NewServer(app.routes())
			defer srv.Close()

			_, before := fetchVersion(t, app, srv, "30000005", "+4530000005")

			changed := base
			mutate(&changed)
			stub.listed = []person.Person{changed}

			_, after := fetchVersion(t, app, srv, "30000005", "+4530000005")
			if before == after {
				t.Errorf("changing %s did not change the version; that edit would never reach devices", field)
			}
		})
	}
}

// The cache is what makes a 60-second poll affordable. Without it, every device's poll is a
// query on the same BFF that takes position reports.
func TestContactsVersion_CachesAcrossRequests(t *testing.T) {
	app, stub := contactsTestApp(t, []person.Person{banditRow()})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000002", "+4530000002")
	for i := 0; i < 5; i++ {
		resp := getWithCookies(t, srv.URL+"/api/contacts/version", cookies)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	if len(stub.listedRoles) != 1 {
		t.Errorf("5 polls caused %d queries, want 1 — the version cache is not working", len(stub.listedRoles))
	}
}

// Two viewers with the same permitted set share a cache entry; different sets do not.
func TestContactsVersion_CacheIsKeyedByPermittedSet(t *testing.T) {
	app, stub := contactsTestApp(t, []person.Person{banditRow(), goeglerRow(), crewRow()})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	// A samarit and a plain crew member have identical permitted sets, so the second
	// poll must be free.
	fetchVersion(t, app, srv, "30000005", "+4530000005")
	afterFirstCrew := len(stub.listedRoles)
	fetchVersion(t, app, srv, "30000007", "+4530000007")
	if len(stub.listedRoles) != afterFirstCrew {
		t.Error("two crew viewers did not share a cache entry despite identical permitted sets")
	}

	// A bandit's set differs, so it must be computed separately.
	fetchVersion(t, app, srv, "30000002", "+4530000002")
	if len(stub.listedRoles) == afterFirstCrew {
		t.Error("a bandit reused the crew cache entry; permitted sets differ and so must versions")
	}
}

func TestContactsVersion_CacheExpires(t *testing.T) {
	cache := newVersionCache(5 * time.Second)
	now := time.Now()
	cache.now = func() time.Time { return now }

	cache.put("crew", "v1")
	if got, ok := cache.get("crew"); !ok || got != "v1" {
		t.Fatalf("get after put = %q, %v", got, ok)
	}

	now = now.Add(6 * time.Second)
	if _, ok := cache.get("crew"); ok {
		t.Error("the entry outlived its TTL; a stale version means changes never propagate")
	}
}

// The cache is keyed by user for profiles and by patrol for the map datasets (task 283), so its key
// space is no longer the handful of role combinations its comment once claimed. Expiry is not removal,
// so without a sweep the map grew one entry per caller for the life of the process — a slow leak on the
// endpoint every device calls on every foreground (task 295).
func TestVersionCache_SweepsExpiredEntries(t *testing.T) {
	cache := newVersionCache(5 * time.Second)
	now := time.Now()
	cache.now = func() time.Time { return now }

	// Well past the sweep threshold, each key expiring before the next arrives — the profile cache's
	// shape: many callers, none of them concurrent.
	for i := range versionCacheSweepAt * 3 {
		cache.put(fmt.Sprintf("user-%d", i), "v1")
		now = now.Add(6 * time.Second)
	}

	cache.mu.Lock()
	size := len(cache.entries)
	cache.mu.Unlock()

	if size > versionCacheSweepAt {
		t.Errorf("cache held %d entries; it must not grow without bound", size)
	}
}

// A sweep may only drop what has expired. If it could evict a live entry the cache would silently stop
// being a cache under load — every call recomputing, which is the cost it exists to avoid.
func TestVersionCache_SweepKeepsLiveEntries(t *testing.T) {
	cache := newVersionCache(time.Hour)
	now := time.Now()
	cache.now = func() time.Time { return now }

	cache.put("live", "keep-me")

	// Fill past the threshold with entries that are already expired, so the next put sweeps.
	cache.now = func() time.Time { return now.Add(-2 * time.Hour) }
	for i := range versionCacheSweepAt + 1 {
		cache.put(fmt.Sprintf("stale-%d", i), "v")
	}
	cache.now = func() time.Time { return now }
	cache.put("trigger", "v")

	if got, ok := cache.get("live"); !ok || got != "keep-me" {
		t.Errorf("a live entry was evicted by a sweep: %q, %v", got, ok)
	}
}

// And correctness does not depend on the threshold: many *live* callers at once are all still served,
// even well past the size at which a sweep runs. Memory is what the threshold bounds, not answers.
func TestVersionCache_ManyLiveEntriesAreAllServed(t *testing.T) {
	cache := newVersionCache(time.Hour)
	now := time.Now()
	cache.now = func() time.Time { return now }

	const n = versionCacheSweepAt + 100
	for i := range n {
		cache.put(fmt.Sprintf("user-%d", i), fmt.Sprintf("v-%d", i))
	}
	for i := range n {
		if got, ok := cache.get(fmt.Sprintf("user-%d", i)); !ok || got != fmt.Sprintf("v-%d", i) {
			t.Fatalf("live entry %d not served: %q, %v", i, got, ok)
		}
	}
}

func TestContactsVersion_NotModified(t *testing.T) {
	app, _ := contactsTestApp(t, []person.Person{banditRow()})
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := authedCookies(t, app, srv, "30000002", "+4530000002")
	first := getWithCookies(t, srv.URL+"/api/contacts/version", cookies)
	io.Copy(io.Discard, first.Body)
	first.Body.Close()

	etag := first.Header.Get("ETag")
	if etag == "" {
		t.Fatal("no ETag on the version response")
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/contacts/version", nil)
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
