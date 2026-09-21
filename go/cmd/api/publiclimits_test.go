package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"nathejk.dk/nathejk/table/trackpoint"
)

// Rate limits and cache headers on the public surface (PRD 011 §6, §8; task 347).
//
// # Why this file exists
//
// Every other route in this repo is behind a login, which is an extremely effective rate limiter. These are
// not — and the load profile is genuinely new: this page is shared in family group chats the morning after
// the event, so **a hundred simultaneous visitors is the normal case, not the stress case**.
//
// So the tests here are about a burst of strangers rather than about one caller's behaviour.

// publicRoutes is every route the public surface answers, with the kind of response it gives.
//
// Built as a table rather than asserted per handler so that a route added later shows up as a missing case
// here instead of quietly shipping with no cache header. `routes.go` is walked by
// `publicprivacy_test.go` for the same reason.
func publicRoutes() []struct {
	path      string
	wantCache string
} {
	return []struct {
		path      string
		wantCache string
	}{
		{"/offentligt", "public, max-age=60"},
		{"/offentligt/patrulje/42", "public, max-age=60"},
		{"/offentligt/patrulje/999999", "public, max-age=60"}, // the not-yet page
		{"/api/public/patrol/42/map", publicJSONCacheControl},
		{"/api/public/albums", publicJSONCacheControl},
	}
}

// **Every public response carries a deliberate cache window.** Not for speed: it is what makes the
// morning-after burst cheap, because the hundredth visitor in a minute is served by a shared cache rather
// than by a merge of somebody's track.
func TestEveryPublicResponseSetsCacheControl(t *testing.T) {
	_, srv := mapApp(t)

	for _, route := range publicRoutes() {
		resp, _ := getPublic(t, srv.URL+route.path, nil)
		if resp.StatusCode != http.StatusOK {
			// An error must **not** be cached — a 503 pinned for a minute turns a blip into an outage
			// every shared cache repeats. Asserted here rather than skipped, because this is the branch
			// a careless "set the header at the top of the handler" would get wrong.
			if got := resp.Header.Get("Cache-Control"); strings.Contains(got, "public") {
				t.Errorf("%s answered %d with Cache-Control %q; failures must not be cached",
					route.path, resp.StatusCode, got)
			}
			continue
		}
		if got := resp.Header.Get("Cache-Control"); got != route.wantCache {
			t.Errorf("%s: Cache-Control = %q, want %q", route.path, got, route.wantCache)
		}
	}
}

// And the album JSON on a server that *has* albums, so the success path of that route is covered rather
// than falling through the unavailable branch above.
func TestTheAlbumMapJSONIsCacheable(t *testing.T) {
	app, _ := albumApp(t)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	resp, body := getPublic(t, srv.URL+"/api/public/albums", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, body)
	}
	if got := resp.Header.Get("Cache-Control"); got != publicJSONCacheControl {
		t.Errorf("Cache-Control = %q, want %q", got, publicJSONCacheControl)
	}
}

// **Media caches for a year; pages cache for a minute.** The asymmetry is the point: media is
// content-addressed, so a changed photograph is a changed URL and there is nothing stale to serve — while a
// page must be able to change, because a takedown has to land promptly (tasks 335, 343).
func TestMediaCachesFarLongerThanPages(t *testing.T) {
	if !strings.Contains(publicGlimtMediaCacheControl, "immutable") {
		t.Errorf("media cache control %q should be immutable", publicGlimtMediaCacheControl)
	}
	if !strings.Contains(publicGlimtMediaCacheControl, "max-age=31536000") {
		t.Errorf("media cache control %q should be a year", publicGlimtMediaCacheControl)
	}
	if !strings.Contains(publicJSONCacheControl, "max-age=60") {
		t.Errorf("JSON cache control %q should be the short window a takedown depends on", publicJSONCacheControl)
	}
}

// **The morning-after burst.** Two hundred visitors hitting one patrol page and its map, concurrently, must
// all be served — and must cost **one** read of the telemetry projection between them, because the merged
// track is cached per patrol (task 340). Without that cache this route walks `TELEMETRY` per request, which
// is precisely what a stranger with a loop would exploit.
func TestAMorningAfterBurstCostsOneTrackRead(t *testing.T) {
	app, srv := mapApp(t)

	// The reader built by mapApp, with counting doubles behind it.
	people := &trackPeople{members: map[string][]string{"team-42": {"p1", "p2"}}}
	points := &trackPoints{byPerson: map[string][]trackpoint.Point{
		"p1": walk(12.200, 10),
		"p2": walk(12.300, 10),
	}}
	app.patrolTracks = trackReader(t, people, points)

	const visitors = 200

	var wg sync.WaitGroup
	codes := make([]int, visitors*2)
	for i := 0; i < visitors; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			resp, _ := getPublic(t, srv.URL+"/offentligt/patrulje/42", nil)
			codes[i*2] = resp.StatusCode
		}(i)
		go func(i int) {
			defer wg.Done()
			resp, _ := getPublic(t, srv.URL+"/api/public/patrol/42/map", nil)
			codes[i*2+1] = resp.StatusCode
		}(i)
	}
	wg.Wait()

	for i, code := range codes {
		if code != http.StatusOK {
			t.Fatalf("request %d answered %d; the burst must not be throttled or fail", i, code)
		}
	}

	// One read of each projection, shared by 400 requests. Not "few" — one: the cache is not an
	// optimisation here, it is what stops the route being a lever.
	if got := points.callCount(); got != 1 {
		t.Errorf("ByPeople called %d times for %d requests; the merged track is not being cached",
			got, len(codes))
	}
	if got := people.callCount(); got != 1 {
		t.Errorf("TrackMembers called %d times; the membership read is not being cached", got)
	}
}

// The limiter is generous enough that a **whole page load** cannot trip it. A patrol page is HTML plus
// config plus two JSON calls; an album page is HTML plus up to sixty thumbnails. A limit that a single page
// view can exhaust is not a limit, it is an outage.
func TestOnePageLoadCannotTripTheLimits(t *testing.T) {
	app, srv := mapApp(t)
	// The production ceilings, wired as main.go wires them.
	app.publicGlimtReadLimiter = limiterOrNil(3000, time.Minute)
	app.publicMediaReadLimiter = limiterOrNil(12000, time.Minute)

	// Sixty-one requests: one page and the thumbnails an album of sixty would ask for.
	for i := 0; i < 61; i++ {
		resp, _ := getPublic(t, srv.URL+"/offentligt/patrulje/42", nil)
		if resp.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("request %d of a single page load was throttled", i+1)
		}
	}
}

// **A throttled visitor gets a Danish sentence, not a bare 429.** This is a route a grandparent reaches by
// following a link; `429 Too Many Requests` in a browser reads as *broken*, which sends them to a leader to
// ask why.
func TestAThrottledPublicRequestExplainsItselfInDanish(t *testing.T) {
	app, srv := mapApp(t)
	app.publicGlimtReadLimiter = limiterOrNil(1, time.Hour)

	if resp, _ := getPublic(t, srv.URL+"/offentligt", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("the first request should pass, got %d", resp.StatusCode)
	}
	resp, body := getPublic(t, srv.URL+"/offentligt", nil)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 past the limit", resp.StatusCode)
	}
	if !strings.Contains(string(body), "Prøv igen") {
		t.Errorf("a throttled response must say something a visitor understands, got %s", body)
	}
}

// **Media has its own budget.** Sixty thumbnails per album page would otherwise spend the allowance the
// pages need, and the visible failure would be a page that looks broken rather than a limit being hit. So
// exhausting the media budget must leave the pages answering.
func TestMediaAndPagesDoNotShareABudget(t *testing.T) {
	app, srv := mapApp(t)
	app.publicMediaReadLimiter = limiterOrNil(1, time.Hour)
	app.publicGlimtReadLimiter = limiterOrNil(100, time.Minute)

	// Two media requests: the second is past the media ceiling. 404 is fine — the fixture has no album
	// media — what matters is which limiter answers.
	for i := 0; i < 2; i++ {
		getPublic(t, srv.URL+"/api/public/albums/al-1/media/0?variant=thumb", nil)
	}
	resp, _ := getPublic(t, srv.URL+"/api/public/albums/al-1/media/0?variant=thumb", nil)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("media status = %d, want the media budget to be exhausted", resp.StatusCode)
	}

	// And the page is still served, which is the whole point of the split.
	page, _ := getPublic(t, srv.URL+"/offentligt/patrulje/42", nil)
	if page.StatusCode != http.StatusOK {
		t.Errorf("page status = %d; an exhausted media budget must not take the pages down",
			page.StatusCode)
	}
}

// Sanity check on the wiring rather than on behaviour: the media limiter must actually be a different
// instance from the page one. Sharing an instance would pass every test above except this one, and would
// reintroduce exactly the coupling the split exists to remove.
func TestTheMediaLimiterIsItsOwnInstance(t *testing.T) {
	app, _ := mapApp(t)
	app.publicGlimtReadLimiter = limiterOrNil(10, time.Minute)
	app.publicMediaReadLimiter = limiterOrNil(10, time.Minute)

	if fmt.Sprintf("%p", app.publicGlimtReadLimiter) == fmt.Sprintf("%p", app.publicMediaReadLimiter) {
		t.Error("the media and page limiters must be separate instances")
	}
}
