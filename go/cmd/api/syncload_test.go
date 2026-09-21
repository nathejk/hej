package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jrgensen/cqrs/cqrstest"
	"github.com/nathejk/shared-go/types"

	"nathejk.dk/internal/commands"
	"nathejk.dk/internal/data"
	"nathejk.dk/internal/reveal"
	"nathejk.dk/internal/scans"
	"nathejk.dk/internal/session"
	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/checkpoint"
	"nathejk.dk/nathejk/table/kort"
	"nathejk.dk/nathejk/table/person"
)

// Load test for the freshness check (task 291, PRD 017 §8/§9).
//
// # What this measures, and what it deliberately does not
//
// PRD 017 §8 calls load "the whole risk": this endpoint is the app's only continuous during-race
// traffic besides position reporting, and it lands on the same BFF. So the question is whether a few
// hundred devices checking on every foreground plus an interval is measurable next to a position
// report.
//
// This runs **in process**, against an `httptest` server, with synthetic projections sized to an event.
// That means it measures the thing the design is actually uncertain about — derivation cost, cache
// behaviour under a herd, and lock contention across the five version caches — and it measures it
// reproducibly, on any machine, without needing a deployed environment or a populated database.
//
// It does **not** measure TLS, Traefik, the network, or MySQL. Those are real costs and this says
// nothing about them; a number from here is a floor, not a forecast. What makes it worth having anyway
// is that the endpoint's own work is the part nobody could otherwise size, and the part a bad version
// derivation would show up in.
//
// # Why it is env-gated rather than skipped by default
//
// The dev container re-runs `go test ./...` on every change, so a multi-second load test in the normal
// suite would tax every edit for a number that changes only when the derivations change. `SYNC_LOAD=1`
// turns it on:
//
//	cd go && SYNC_LOAD=1 go test ./cmd/api/ -run TestSyncLoad -v
//
// Knobs, all optional: SYNC_LOAD_DEVICES, SYNC_LOAD_PATROLS, SYNC_LOAD_ROUNDS, SYNC_LOAD_CONCURRENCY.

// loadDirectory is a synthetic users.Directory sized to an event: many users, spread across patrols.
//
// The spread is the point. Contacts caches by permitted role set (a handful of keys for a whole event),
// the map datasets by patrol (hundreds), and profile by user (thousands) — so a load test with a handful
// of users would sit almost entirely on cache hits and report a flattering number that says nothing
// about a real event.
type loadDirectory struct {
	users   []users.User
	byID    map[string]users.User
	patrols int
}

func newLoadDirectory(devices, patrols int) *loadDirectory {
	d := &loadDirectory{byID: make(map[string]users.User, devices), patrols: patrols}
	for i := range devices {
		// Every fourth device is personnel: no patrol, but a contacts pane. That mix matters, because the
		// two roles exercise different key spaces and a different number of datasets.
		patrolID := ""
		role := users.RoleCrew
		if i%4 != 0 {
			patrolID = fmt.Sprintf("team-%d", i%patrols)
			role = users.RoleSpejder
		}
		u := users.User{
			ID:         fmt.Sprintf("u-%d", i),
			Role:       role,
			Name:       fmt.Sprintf("Deltager %d", i),
			PatrolID:   patrolID,
			PatrolName: patrolID,
			Phone:      fmt.Sprintf("+45300%05d", i),
		}
		d.users = append(d.users, u)
		d.byID[u.ID] = u
	}
	return d
}

func (d *loadDirectory) LookupAll(string) []users.User    { return nil }
func (d *loadDirectory) Lookup(string) (users.User, bool) { return users.User{}, false }
func (d *loadDirectory) Get(id string) (users.User, bool) { u, ok := d.byID[id]; return u, ok }

// loadPeople answers the person projection for every synthetic user, plus a directory of realistic size.
//
// 150 listed rows, because that is what the contacts manifest holds for this event (PRD 017 §11) and the
// contacts version hashes every one of them on a miss — the most expensive of the six derivations.
type loadPeople struct {
	listed []person.Person
	rows   map[string]person.Person
}

func newLoadPeople(dir *loadDirectory) *loadPeople {
	p := &loadPeople{rows: make(map[string]person.Person, len(dir.users))}
	for _, u := range dir.users {
		p.rows[u.ID] = person.Person{
			PersonID: u.ID, Name: u.Name, Phone: u.Phone,
			TeamID: u.PatrolID, TeamName: u.PatrolName, PortraitRef: "ref-" + u.ID,
		}
	}
	for i := range 150 {
		p.listed = append(p.listed, person.Person{
			PersonID: fmt.Sprintf("p-%d", i), Name: fmt.Sprintf("Kontakt %d", i),
			Phone: fmt.Sprintf("+45200%05d", i), AppRole: string(users.RoleCrew),
			TeamID: "sektion", TeamName: "Sektion", PortraitRef: fmt.Sprintf("ref-p-%d", i),
		})
	}
	return p
}

func (p *loadPeople) Get(_, personID string) (person.Person, bool, error) {
	row, ok := p.rows[personID]
	return row, ok, nil
}
func (p *loadPeople) Lookup(string, string) ([]person.Person, error) { return nil, nil }
func (p *loadPeople) ListByAppRoles(string, []string) ([]person.Person, error) {
	return p.listed, nil
}
func (p *loadPeople) ListPatrolByNumber(string, string) ([]person.Person, error) { return nil, nil }

func (p *loadPeople) TrackMembers(string, string) ([]person.TrackMember, error) { return nil, nil }
func (p *loadPeople) ExpiredPortraits(string, time.Time, int) ([]person.ExpiredPortrait, error) {
	return nil, nil
}

// loadMaps answers the map projections with a plausible amount of data per patrol.
type loadMaps struct {
	revealed []checkpoint.Checkpoint
	handouts []reveal.Handout
}

func (m loadMaps) Revealed(string, string, bool) (reveal.RevealedMap, error) {
	return reveal.RevealedMap{Checkpoints: m.revealed, NextCheckgroup: types.CheckgroupID("cg-2")}, nil
}
func (m loadMaps) Handouts(string, string) ([]reveal.Handout, error) { return m.handouts, nil }

// loadScans answers the scan source per patrol.
type loadScans struct{ rows []scans.Scan }

func (s loadScans) ByPatrol(string) []scans.Scan { return s.rows }

func loadEnvInt(t *testing.T, key string, fallback int) int {
	t.Helper()
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		t.Fatalf("%s=%q is not a positive integer", key, raw)
	}
	return n
}

// loadApp assembles an app with synthetic projections at event scale.
func loadApp(t *testing.T, devices, patrols int) (*application, *loadDirectory) {
	t.Helper()

	dir := newLoadDirectory(devices, patrols)

	var revealed []checkpoint.Checkpoint
	for i := range 12 {
		revealed = append(revealed, checkpoint.Checkpoint{
			ID: types.CheckpointID(fmt.Sprintf("cp-%d", i)), Name: fmt.Sprintf("Post %d", i),
			Checkgroup: types.CheckgroupID(fmt.Sprintf("cg-%d", i/4)), SortOrder: i,
			Lat: 56.0 + float64(i)/100, Lng: 9.5 + float64(i)/100,
			OpenFromUts: 1000, OpenUntilUts: 9000,
		})
	}
	var handouts []reveal.Handout
	for i := range 4 {
		handouts = append(handouts, reveal.Handout{
			Sheet: kort.KortID(fmt.Sprintf("kort-%d", i)), Name: fmt.Sprintf("Etape %d", i),
			QrID: strconv.Itoa(i), HandedOutUts: int64(1000 + i), StillHeld: true,
		})
	}
	var scanRows []scans.Scan
	for i := range 8 {
		scanRows = append(scanRows, scans.Scan{
			ID: fmt.Sprintf("s-%d", i), Kind: scans.KindCheckpoint,
			Label: fmt.Sprintf("Post %d", i), ScannedAt: time.Unix(int64(1000+i), 0),
		})
	}

	app := newTestApp(t)
	app.config.eventYear = "2026"
	app.config.syncIntervalSeconds = 60
	app.config.syncDebounceSeconds = 5
	// A fake telemetry publisher, so position reports can be measured alongside the sync check rather than
	// answering 503. Same double the track tests use.
	publisherHolder := commands.NewPublisherHolder()
	publisherHolder.Set(&cqrstest.Publisher{})
	app.commands = commands.New(publisherHolder)
	app.models = data.NewModels(dir, loadScans{rows: scanRows},
		fakeRaceAreas{area: areaFixture(), ok: true}, newLoadPeople(dir), nil,
		data.WithMapReads(loadMaps{revealed: revealed, handouts: handouts}))

	// Production TTLs and a fresh metrics instance, so the numbers describe the shipped configuration
	// rather than a test-friendly one.
	metrics := newSyncMetrics()
	app.syncMetrics = metrics
	app.contactsVersions = newVersionCache(5*time.Second).observedAs("contacts", metrics)
	app.checkpointsVersions = newVersionCache(5*time.Second).observedAs("checkpoints", metrics)
	app.handoutsVersions = newVersionCache(5*time.Second).observedAs("handouts", metrics)
	app.scansVersions = newVersionCache(5*time.Second).observedAs("scans", metrics)
	app.profileVersions = newVersionCache(5*time.Second).observedAs("profile", metrics)
	app.raceAreaVersions = newVersionCache(5*time.Second).observedAs("race_area", metrics)

	return app, dir
}

// cookieFor mints a session cookie directly, bypassing the PIN flow.
//
// The alternative — running the real login for every simulated device — would spend the whole test on
// PIN issuance and SMS stubs, measuring the wrong endpoint entirely.
func cookieFor(app *application, u users.User) *http.Cookie {
	rec := httptest.NewRecorder()
	app.sessions.Issue(rec, u.ID, string(u.Role))
	for _, c := range rec.Result().Cookies() {
		if c.Name == session.CookieName {
			return c
		}
	}
	return nil
}

type latencies struct {
	mu  sync.Mutex
	all []time.Duration
}

func (l *latencies) add(d time.Duration) {
	l.mu.Lock()
	l.all = append(l.all, d)
	l.mu.Unlock()
}

func (l *latencies) percentile(p float64) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.all) == 0 {
		return 0
	}
	sorted := append([]time.Duration{}, l.all...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

// TestSyncLoad drives the endpoint at event scale and reports the numbers PRD 017 §9 asks for.
//
// Two runs, because they answer different questions:
//
//  1. **Steady state** — every device holds an ETag, so almost every check is a 304 and almost every
//     version comes from cache. This is what an event actually looks like between changes, and where
//     the ≥ 95 % unchanged target lives.
//  2. **Thundering herd** — every device checks at once with cold caches and no ETag. This is a race
//     start or a broadcast notification, and it is the case the 5 s TTL either absorbs or does not.
func TestSyncLoad(t *testing.T) {
	if os.Getenv("SYNC_LOAD") == "" {
		t.Skip("set SYNC_LOAD=1 to run the load test (task 291)")
	}

	devices := loadEnvInt(t, "SYNC_LOAD_DEVICES", 400)
	patrols := loadEnvInt(t, "SYNC_LOAD_PATROLS", 100)
	rounds := loadEnvInt(t, "SYNC_LOAD_ROUNDS", 5)
	concurrency := loadEnvInt(t, "SYNC_LOAD_CONCURRENCY", 32)

	app, dir := loadApp(t, devices, patrols)
	srv := httptest.NewServer(app.routes())
	defer srv.Close()

	cookies := make([]*http.Cookie, devices)
	for i, u := range dir.users {
		cookies[i] = cookieFor(app, u)
		if cookies[i] == nil {
			t.Fatalf("no session cookie for %s", u.ID)
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	etags := make([]string, devices)

	// One pass first, to give every device an ETag and warm the caches — the state a device is actually
	// in when it foregrounds during an event.
	for i := range devices {
		resp, _, etag := loadCheck(t, client, srv.URL, cookies[i], "")
		if resp != http.StatusOK {
			t.Fatalf("warm-up status = %d, want 200", resp)
		}
		etags[i] = etag
	}

	t.Run("steady", func(t *testing.T) {
		// Fresh metrics per run, or the herd's derivation counts would include the steady run's and the
		// numbers could not be attributed to either.
		app.syncMetrics = newSyncMetrics()
		observeCaches(app)

		lat := &latencies{}
		var notModified, ok atomic.Int64
		var bytes atomic.Int64

		start := time.Now()
		runConcurrently(devices*rounds, concurrency, func(n int) {
			i := n % devices
			began := time.Now()
			status, n2, _ := loadCheck(t, client, srv.URL, cookies[i], etags[i])
			lat.add(time.Since(began))
			bytes.Add(n2)
			switch status {
			case http.StatusNotModified:
				notModified.Add(1)
			case http.StatusOK:
				ok.Add(1)
			default:
				t.Errorf("unexpected status %d", status)
			}
		})
		elapsed := time.Since(start)

		total := notModified.Load() + ok.Load()
		report(t, "steady", elapsed, total, lat, app.syncMetrics.snapshot())
		t.Logf("steady: 304 %d  200 %d  (%.1f%% unchanged)  body bytes total %d",
			notModified.Load(), ok.Load(), 100*float64(notModified.Load())/float64(total), bytes.Load())

		// What the endpoint costs at the load it will actually see, rather than at saturation: every device
		// checks once per served interval, so expected traffic is devices/interval.
		expected := float64(devices) / float64(app.config.syncIntervalSeconds)
		achieved := float64(total) / elapsed.Seconds()
		t.Logf("steady: expected event load %.1f req/s; sustained %.0f req/s (%.0fx headroom)",
			expected, achieved, achieved/expected)

		// The endpoint's economy in one assertion. Below this, something is deriving a version wrongly
		// and every device is refetching payloads it already holds.
		if ratio := float64(notModified.Load()) / float64(total); ratio < 0.95 {
			t.Errorf("unchanged ratio %.3f < 0.95 (PRD 017 §9)", ratio)
		}
	})

	t.Run("herd", func(t *testing.T) {
		// Cold caches, so every one of the six derivations runs for every distinct key. A race start, or
		// a broadcast that brings a few hundred phones out of a pocket at once.
		app.syncMetrics = newSyncMetrics()
		observeCaches(app)

		lat := &latencies{}
		var bytes atomic.Int64
		start := time.Now()
		runConcurrently(devices, concurrency, func(n int) {
			began := time.Now()
			// No ETag: the worst case, where the full response is built and sent.
			status, size, _ := loadCheck(t, client, srv.URL, cookies[n], "")
			if status != http.StatusOK {
				t.Errorf("herd status = %d, want 200", status)
			}
			lat.add(time.Since(began))
			bytes.Add(size)
		})
		elapsed := time.Since(start)

		report(t, "herd", elapsed, int64(devices), lat, app.syncMetrics.snapshot())
		t.Logf("herd: mean response body %d bytes", bytes.Load()/int64(devices))
	})

	// The comparison PRD 017 §9 asks for: what one check costs next to one position report, which is the
	// app's other continuous during-race traffic and lands on the same BFF.
	t.Run("versus position report", func(t *testing.T) {
		app.syncMetrics = newSyncMetrics()
		observeCaches(app)

		// One device, sequentially, for both endpoints — a ratio is only meaningful if the two were measured
		// under the same conditions, and concurrency would let scheduling noise dominate.
		const n = 200
		syncLat := &latencies{}
		for i := range n {
			began := time.Now()
			loadCheck(t, client, srv.URL, cookies[i%devices], etags[i%devices])
			syncLat.add(time.Since(began))
		}

		// The rate limiter allows 20 batches per user per minute, so spread the reports across devices
		// rather than hammering one — a 429 would measure the limiter instead of the endpoint.
		trackLat := &latencies{}
		for i := range n {
			began := time.Now()
			status := loadTrack(t, client, srv.URL, cookies[i%devices])
			if status != http.StatusAccepted && status != http.StatusOK && status != http.StatusNoContent {
				t.Fatalf("unexpected /api/track status %d (%d of %d)", status, i, n)
			}
			trackLat.add(time.Since(began))
		}

		syncP50 := syncLat.percentile(0.50)
		trackP50 := trackLat.percentile(0.50)
		t.Logf("one sync check p50 %v; one position report p50 %v; ratio %.2fx",
			syncP50.Round(time.Microsecond), trackP50.Round(time.Microsecond),
			float64(syncP50)/float64(trackP50))
	})
}

// observeCaches re-attaches the caches to the app's current metrics, and clears them.
//
// Both halves matter: the metrics must be the ones being read, and the caches must be cold so a run's
// derivation counts describe that run.
func observeCaches(app *application) {
	m := app.syncMetrics
	app.contactsVersions = newVersionCache(5*time.Second).observedAs("contacts", m)
	app.checkpointsVersions = newVersionCache(5*time.Second).observedAs("checkpoints", m)
	app.handoutsVersions = newVersionCache(5*time.Second).observedAs("handouts", m)
	app.scansVersions = newVersionCache(5*time.Second).observedAs("scans", m)
	app.profileVersions = newVersionCache(5*time.Second).observedAs("profile", m)
	app.raceAreaVersions = newVersionCache(5*time.Second).observedAs("race_area", m)
}

// loadTrack posts a one-point position batch, the smallest realistic position report.
func loadTrack(t *testing.T, client *http.Client, base string, cookie *http.Cookie) int {
	t.Helper()
	body := fmt.Sprintf(`{"points":[{"ts":%d,"lat":56.1,"lng":9.5,"accuracy":12.5}]}`,
		time.Now().UnixMilli())
	req, err := http.NewRequest(http.MethodPost, base+"/api/track", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /api/track: %v", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

func loadCheck(t *testing.T, client *http.Client, base string, cookie *http.Cookie, etag string) (int, int64, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/sync", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.AddCookie(cookie)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /api/sync: %v", err)
	}
	defer resp.Body.Close()
	n, _ := io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, n, resp.Header.Get("ETag")
}

func runConcurrently(total, concurrency int, fn func(n int)) {
	var next atomic.Int64
	var wg sync.WaitGroup
	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				n := int(next.Add(1)) - 1
				if n >= total {
					return
				}
				fn(n)
			}
		}()
	}
	wg.Wait()
}

func report(t *testing.T, name string, elapsed time.Duration, calls int64, lat *latencies, snap syncMetricsSnapshot) {
	t.Helper()
	t.Logf("%s: %d calls in %v (%.0f req/s at concurrency)", name, calls, elapsed.Round(time.Millisecond),
		float64(calls)/elapsed.Seconds())
	// Client-side round trip, i.e. what a device waits for (minus the network).
	t.Logf("%s: round trip p50 %v  p95 %v  p99 %v", name,
		lat.percentile(0.50).Round(time.Microsecond),
		lat.percentile(0.95).Round(time.Microsecond),
		lat.percentile(0.99).Round(time.Microsecond))
	// Server-side handler only, from the endpoint's own instrumentation (task 293). Kept on its own line
	// because mixing the two in one row produced a "max" below the "p50" and read as a bug in the numbers.
	t.Logf("%s: handler mean %v  max %v", name,
		snap.MeanDuration().Round(time.Microsecond), snap.MaxDuration.Round(time.Microsecond))
	for _, dataset := range []string{"contacts", "profile", "scans", "handouts", "checkpoints", "race_area"} {
		stats := snap.Datasets[dataset]
		t.Logf("%s: %-12s computed %-6d churn %.3f", name, dataset, stats.Computed, stats.ChurnRatio())
	}
}
