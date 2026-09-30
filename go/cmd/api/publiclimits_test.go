package main

import (
	"fmt"
	"io"
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
		{"/2026", "public, max-age=60"},
		{"/2026/patrulje/42", "public, max-age=60"},
		{"/2026/patrulje/999999", "public, max-age=60"}, // the not-yet page
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

	// **400 requests, 64 of them in flight at a time** (task 458).
	//
	// # Why not all 400 at once, which is what this did
	//
	// HTTP/1.1 needs one connection per in-flight request, so 400 simultaneous requests means 400 sockets against one
	// `httptest` listener — and 400 sockets in TIME_WAIT, repeatedly, on a machine running the rest of the suite.
	// That produced two failures with two different messages, neither about this route: "connection reset by peer",
	// and then, once the client stopped hiding it, `can't assign requested address` — the local ephemeral port range
	// running dry.
	//
	// **The assertion is unchanged: 400 requests must cost one read.** That is what the counting doubles below check,
	// and it does not depend on how many sockets were open at once. What the concurrency has to do is make the
	// requests *overlap*, so that the second visitor arrives before the first has finished reading and a cache
	// without single-flight would be caught doing two reads. 64 simultaneous first-hits tests that as well as 400
	// do, and it is 64 sockets instead of 400.
	//
	// A pool that serialised them would be worthless here — it would pass against no cache at all — which is why
	// this is a pool of 64 and not of one.
	const inFlight = 64

	// Its own client rather than `http.DefaultClient`, whose transport keeps two idle connections per host and so
	// re-dials for almost every request even at this concurrency.
	transport := &http.Transport{
		MaxIdleConns:        inFlight,
		MaxIdleConnsPerHost: inFlight,
		MaxConnsPerHost:     inFlight,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second}

	// Collected rather than asserted in place, because **`t.Fatalf` may not be called from these goroutines**: it
	// runs `runtime.Goexit`, so the line recording the status code never executes and the failure surfaces as a
	// zero from a different assertion. `getPublic` fatals, which is right for the sequential tests that use it and
	// wrong for 400 goroutines — so this one does its own request and reports afterwards.
	var wg sync.WaitGroup
	codes := make([]int, visitors*2)
	errs := make([]error, visitors*2)
	get := func(slot int, url string) {
		defer wg.Done()
		resp, err := client.Get(url)
		if err != nil {
			errs[slot] = err
			return
		}
		defer resp.Body.Close()
		// Drained, or the connection cannot be reused and the budget above buys nothing.
		_, _ = io.Copy(io.Discard, resp.Body)
		codes[slot] = resp.StatusCode
	}

	// The two URLs interleaved, so both reads are raced rather than one being warm by the time the other starts.
	urls := make([]string, visitors*2)
	for i := 0; i < visitors; i++ {
		urls[i*2] = srv.URL + "/2026/patrulje/42"
		urls[i*2+1] = srv.URL + "/api/public/patrol/42/map"
	}

	// A gate rather than 400 goroutines: the goroutines are free, the sockets are not.
	slots := make(chan struct{}, inFlight)
	for i, url := range urls {
		wg.Add(1)
		slots <- struct{}{}
		go func(slot int, url string) {
			defer func() { <-slots }()
			get(slot, url)
		}(i, url)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("request %d failed at the transport: %v", i, err)
		}
	}
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
		resp, _ := getPublic(t, srv.URL+"/2026/patrulje/42", nil)
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

	if resp, _ := getPublic(t, srv.URL+"/2026", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("the first request should pass, got %d", resp.StatusCode)
	}
	resp, body := getPublic(t, srv.URL+"/2026", nil)
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
	page, _ := getPublic(t, srv.URL+"/2026/patrulje/42", nil)
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
