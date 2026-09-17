// Command glimtload seeds a realistic Glimt corpus and load-tests the post-race browse.
//
// PRD 019 §0a.3 names the finish line as the load peak of the whole feature: a thousand people on
// the same congested network at the same time, each paging through thousands of items and pulling
// media. §9 makes p95 on the hold collection during the hour after the race a headline metric.
// Task 324 exists because **that cannot be fixed on the night**, so it gets measured first.
//
// # Two modes
//
//	glimtload -seed   -holds 120 -glimt 2000 -blobs 400
//	glimtload -load   -scenario hold -concurrency 50 -requests 2000
//
// Run inside the api container: it needs the database and the blob volume, neither of which is
// reachable from the host (the db publishes no port and /blobs is a container volume).
//
//	docker compose exec api /usr/local/go/bin/go run ./cmd/glimtload -seed
//
// # Why seeding writes SQL directly instead of publishing events
//
// The projection is normally built by replaying the event stream, and the dev fixture
// (/api/dev/glimt-fixture) works that way deliberately. This does not, for two reasons:
//
//  1. **The broker is shared and append-only.** `nathejk-jetstream-1` is a container shared with the
//     other Nathejk apps, and a published glimt cannot be un-published — only purged. Putting two
//     thousand synthetic events on it to measure a SELECT is antisocial and irreversible.
//  2. **A load test measures the read model**, and the read model is a table. Inserting rows tests
//     exactly the queries and indexes under test, with none of the event-plumbing cost in the way.
//
// The consequence is worth stating plainly rather than discovering: **a projection rebuild wipes
// this corpus**, because a replay of the stream will not contain it. That is correct behaviour and
// mildly useful — the test data cleans itself up on the next boot that replays from scratch.
//
// # Why the blobs are distinct
//
// Media rows could all point at one object: content addressing means identical bytes are one blob,
// so a thousand rows sharing a ref is legal. It would also make the media measurements a lie, since
// every read after the first would come out of the page cache. So `-blobs` distinct objects are
// generated and rows are spread across them.
package main

import (
	"bytes"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log"
	"math"
	mrand "math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"nathejk.dk/internal/blob"
	"nathejk.dk/internal/session"
)

func main() {
	var (
		doSeed  = flag.Bool("seed", false, "insert a corpus")
		doLoad  = flag.Bool("load", false, "drive load against the API")
		doClean = flag.Bool("clean", false, "remove the corpus and the objects it wrote")

		dsn       = flag.String("dsn", os.Getenv("DB_DSN"), "database DSN")
		blobPath  = flag.String("blob-path", envOr("BLOB_PATH", "/blobs"), "blob store root")
		year      = flag.String("year", envOr("EVENT_YEAR", "2026"), "event year")
		holds     = flag.Int("holds", 120, "how many holds post")
		total     = flag.Int("glimt", 2000, "how many glimt in total")
		blobCount = flag.Int("blobs", 400, "how many distinct media objects to generate")

		base        = flag.String("base", "http://localhost:4000", "API base URL")
		scenario    = flag.String("scenario", "hold", "hold | feed | thumb | thumb-cached | media")
		concurrency = flag.Int("concurrency", 50, "concurrent clients")
		requests    = flag.Int("requests", 2000, "total requests")
		users       = flag.Int("users", 50, "distinct directory members to act as")
		secret      = flag.String("session-secret", os.Getenv("SESSION_SECRET"), "to mint sessions")
	)
	flag.Parse()

	modes := 0
	for _, on := range []bool{*doSeed, *doLoad, *doClean} {
		if on {
			modes++
		}
	}
	if modes != 1 {
		log.Fatal("choose exactly one of -seed, -load or -clean")
	}

	db, err := sql.Open("mysql", *dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("ping db: %v", err)
	}

	if *doSeed {
		if err := seed(db, *blobPath, *year, *holds, *total, *blobCount); err != nil {
			log.Fatalf("seed: %v", err)
		}
		return
	}
	if *doClean {
		if err := clean(db, *blobPath, *year); err != nil {
			log.Fatalf("clean: %v", err)
		}
		return
	}
	if err := load(db, *base, *year, *scenario, *concurrency, *requests, *users, *secret); err != nil {
		log.Fatalf("load: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// manifestName is where the seeder records the objects it wrote, so -clean can remove exactly those.
//
// A manifest rather than "delete every orphan": the blob store also holds **portraits**, and an
// orphan sweep that got its reachability query wrong would delete photographs of minors that cannot
// be re-derived (PRD 008 §8 — the blob store is the one thing here that is not rebuildable). Deleting
// only what this tool created is the version that cannot go wrong.
const manifestName = ".glimtload-manifest"

// clean removes the seeded rows and the objects the seeder wrote.
func clean(db *sql.DB, blobPath, year string) error {
	res, err := db.Exec(`DELETE FROM glimt_media WHERE year = ? AND glimtId LIKE 'loadtest-%'`, year)
	if err != nil {
		return fmt.Errorf("delete media rows: %w", err)
	}
	mediaRows, _ := res.RowsAffected()

	res, err = db.Exec(`DELETE FROM glimt WHERE year = ? AND glimtId LIKE 'loadtest-%'`, year)
	if err != nil {
		return fmt.Errorf("delete glimt rows: %w", err)
	}
	glimtRows, _ := res.RowsAffected()

	manifestPath := filepath.Join(blobPath, manifestName)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("removed %d glimt and %d media rows; no blob manifest, so no objects deleted",
				glimtRows, mediaRows)
			return nil
		}
		return err
	}

	store, err := blob.NewFileStore(blobPath)
	if err != nil {
		return err
	}
	ctx := context.Background()
	deleted := 0
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		ref := blob.Ref(strings.TrimSpace(line))
		if ref == "" {
			continue
		}
		if err := store.Delete(ctx, ref); err == nil {
			deleted++
		}
	}
	if err := os.Remove(manifestPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	log.Printf("removed %d glimt rows, %d media rows and %d objects", glimtRows, mediaRows, deleted)
	return nil
}

// ---------------------------------------------------------------------------
// Seeding

// seed writes a corpus shaped like a real event rather than a uniform grid.
func seed(db *sql.DB, blobPath, year string, holds, total, blobCount int) error {
	store, err := blob.NewFileStore(blobPath)
	if err != nil {
		return fmt.Errorf("blob store: %w", err)
	}

	log.Printf("generating %d distinct media objects…", blobCount)
	refs, thumbs, bytesEach, err := generateBlobs(store, blobCount)
	if err != nil {
		return err
	}
	log.Printf("wrote %d full + %d thumb objects (~%d kB each full)", len(refs), len(thumbs), bytesEach/1024)

	// Recorded before any row is written, so an interrupted seed still leaves a manifest that
	// -clean can act on. An orphaned object is cheap; an unremovable one is not.
	if err := writeManifest(blobPath, refs, thumbs); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	// A long tail, not a uniform spread. Most holds post a handful; a few post a lot. That
	// distribution is the point: the hold collection is read per hold, so a p99 taken against a
	// uniform corpus would miss the enthusiastic patrulje whose page is ten times the median.
	perHold := longTail(holds, total)

	rnd := mrand.New(mrand.NewSource(42)) // fixed seed: a rerun measures the same corpus
	groups := []string{"spejder", "spejder", "spejder", "bandit", "crew"}

	inserted, mediaRows := 0, 0
	start := time.Now()

	for h := 0; h < holds; h++ {
		group := groups[h%len(groups)]
		teamNumber := fmt.Sprintf("%d", 100+h)
		teamName := fmt.Sprintf("LOADTEST Hold %d", 100+h)
		if group == "crew" {
			teamNumber = ""
			teamName = fmt.Sprintf("LOADTEST Sektion %d", 100+h)
		}
		authorID := fmt.Sprintf("loadtest-author-%d", h)

		for g := 0; g < perHold[h]; g++ {
			id := fmt.Sprintf("loadtest-%d-%d", h, g)
			// Spread over the race weekend, so ORDER BY createdAt has real work to do and the
			// public-retention cutoff has something on both sides of it.
			createdAt := time.Now().UTC().Add(-time.Duration(rnd.Intn(72)) * time.Hour)
			audience := pickAudience(rnd)

			if _, err := db.Exec(`
				INSERT INTO glimt (glimtId, year, authorPersonId, authorGroup, teamNumber,
					teamName, audience, caption, createdAt, mediaCount, reportCount, deleted)
				VALUES (?,?,?,?,?,?,?,?,?,?,0,0)
				ON DUPLICATE KEY UPDATE createdAt = VALUES(createdAt)`,
				id, year, authorID, group, teamNumber, teamName, audience,
				"LOADTEST fixture", createdAt, 0); err != nil {
				return fmt.Errorf("insert glimt: %w", err)
			}

			// 1–4 items, weighted low: most glimt are one or two photographs.
			items := 1 + rnd.Intn(4)
			if items > 2 && rnd.Intn(2) == 0 {
				items = 2
			}
			for o := 0; o < items; o++ {
				n := rnd.Intn(len(refs))
				if _, err := db.Exec(`
					INSERT INTO glimt_media (glimtId, year, ordinal, blobRef, thumbRef, kind,
						contentType, bytes, width, height, durationMs)
					VALUES (?,?,?,?,?,'image','image/jpeg',?,1600,1200,0)
					ON DUPLICATE KEY UPDATE blobRef = VALUES(blobRef)`,
					id, year, o, string(refs[n]), string(thumbs[n]), bytesEach); err != nil {
					return fmt.Errorf("insert media: %w", err)
				}
				mediaRows++
			}
			if _, err := db.Exec(`UPDATE glimt SET mediaCount = ? WHERE glimtId = ? AND year = ?`,
				items, id, year); err != nil {
				return fmt.Errorf("update mediaCount: %w", err)
			}
			inserted++
			if inserted%250 == 0 {
				log.Printf("  %d glimt…", inserted)
			}
		}
	}

	log.Printf("seeded %d glimt across %d holds, %d media rows, in %s",
		inserted, holds, mediaRows, time.Since(start).Round(time.Millisecond))
	log.Printf("NOTE: a projection rebuild will wipe this corpus — it is not on the event stream.")
	return nil
}

// longTail splits `total` glimt across `holds` so a few holds have many and most have few.
//
// Shaped by hand rather than drawn from a distribution, because the useful property is simply that
// the spread is wide: a p99 measured against a uniform corpus would never see the biggest page.
func longTail(holds, total int) []int {
	out := make([]int, holds)
	weights := make([]float64, holds)
	var sum float64
	for i := range weights {
		// 1/(rank) — a Zipf-ish curve. The busiest hold ends up with roughly ten times the median.
		weights[i] = 1 / float64(i+1)
		sum += weights[i]
	}
	assigned := 0
	for i := range out {
		out[i] = int(float64(total) * weights[i] / sum)
		if out[i] < 1 {
			out[i] = 1
		}
		assigned += out[i]
	}
	// Push the remainder onto the busiest hold, so the worst case is a little worse.
	if assigned < total {
		out[0] += total - assigned
	}
	return out
}

// writeManifest records every object the seeder created, appending so repeated seeds accumulate.
func writeManifest(blobPath string, sets ...[]blob.Ref) error {
	f, err := os.OpenFile(filepath.Join(blobPath, manifestName),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, set := range sets {
		for _, ref := range set {
			if _, err := fmt.Fprintln(f, ref); err != nil {
				return err
			}
		}
	}
	return nil
}

func pickAudience(rnd *mrand.Rand) string {
	switch n := rnd.Intn(100); {
	case n < 55:
		return "group"
	case n < 85:
		return "nathejk"
	default:
		return "public"
	}
}

// generateBlobs writes `n` distinct full images and thumbnails, returning their refs.
//
// Distinct by construction: each carries a different noise field, so no two hash alike and the page
// cache cannot flatter the media measurements.
func generateBlobs(store *blob.FileStore, n int) (full, thumb []blob.Ref, bytesEach int, err error) {
	ctx := context.Background()
	for i := 0; i < n; i++ {
		fullBytes, err := noiseJPEG(1600, 1200, i)
		if err != nil {
			return nil, nil, 0, err
		}
		thumbBytes, err := noiseJPEG(320, 240, i)
		if err != nil {
			return nil, nil, 0, err
		}
		fr, err := store.Put(ctx, fullBytes)
		if err != nil {
			return nil, nil, 0, err
		}
		tr, err := store.Put(ctx, thumbBytes)
		if err != nil {
			return nil, nil, 0, err
		}
		full = append(full, fr)
		thumb = append(thumb, tr)
		bytesEach = len(fullBytes)
	}
	return full, thumb, bytesEach, nil
}

// noiseJPEG builds an image that compresses to a realistic size.
//
// # Photo-like, not random
//
// The first version filled every pixel with independent random values. That is *distinct* — which is
// what matters for defeating the page cache — but it is also incompressible: 1600×1200 of pure noise
// at Q80 came out at **1.25 MB**, roughly six times what the upload path actually stores. Every media
// measurement taken against it would have been pessimistic by that factor, and a p95 that says "we
// are fine" against 1.25 MB objects is not a useful reassurance about 200 kB ones — but one that says
// "we are too slow" would have sent somebody optimising a problem that does not exist.
//
// So: smooth low-frequency gradients, which is what a photograph mostly is, plus a little
// high-frequency detail so it is not trivially compressible either. The `salt` shifts the phase, so
// no two images hash alike. Lands around 150–250 kB at 1600×1200, matching what task 303's pipeline
// produces.
func noiseJPEG(w, h, salt int) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	rnd := mrand.New(mrand.NewSource(int64(salt) * 7919))
	// Phase offsets, so each image differs in content rather than only in noise.
	px, py := float64(salt%97)*0.31, float64(salt%53)*0.47

	for y := 0; y < h; y++ {
		fy := float64(y) / float64(h)
		for x := 0; x < w; x++ {
			fx := float64(x) / float64(w)
			// Three octaves of smooth variation: the broad shapes a camera sees.
			v := 0.5 +
				0.25*math.Sin((fx*3+px)*math.Pi) +
				0.15*math.Sin((fy*5+py)*math.Pi) +
				0.10*math.Sin((fx*11+fy*7+px)*math.Pi)
			// A little grain. Small enough that the JPEG still compresses, large enough that flat
			// regions are not perfectly flat.
			grain := (rnd.Float64() - 0.5) * 0.08
			img.Set(x, y, color.RGBA{
				R: clamp8((v + grain) * 255),
				G: clamp8((v*0.9 + grain) * 255),
				B: clamp8((v*0.75 + grain) * 255),
				A: 255,
			})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func clamp8(f float64) uint8 {
	if f < 0 {
		return 0
	}
	if f > 255 {
		return 255
	}
	return uint8(f)
}

// ---------------------------------------------------------------------------
// Load

type result struct {
	dur    time.Duration
	status int
}

func load(db *sql.DB, base, year, scenario string, concurrency, requests, users int, secret string) error {
	if secret == "" {
		return fmt.Errorf("no session secret: pass -session-secret or set SESSION_SECRET")
	}

	cookies, role, err := mintCookies(db, year, secret, users)
	if err != nil {
		return err
	}

	targets, err := buildTargets(db, base, year, scenario)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("scenario %q produced no targets — is the corpus seeded?", scenario)
	}

	// A fresh client per worker with keep-alive, which is what a browser does. One shared client
	// with a small pool would measure connection contention rather than the endpoint.
	log.Printf("scenario=%s targets=%d concurrency=%d requests=%d users=%d (%s)",
		scenario, len(targets), concurrency, requests, len(cookies), role)

	results := make([]result, 0, requests)
	var mu sync.Mutex
	var wg sync.WaitGroup
	work := make(chan string, concurrency*2)

	start := time.Now()
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			client := &http.Client{
				Timeout:   30 * time.Second,
				Transport: &http.Transport{MaxIdleConnsPerHost: 4},
			}
			// One member per worker. The read limiter is keyed by member (deliberately —
			// participants share networks), so a single-user run measures the limiter rather than
			// the endpoint: the first version of this tool sent 2000 requests as one member and
			// got 70% 429s. Spreading across members is also what the scenario actually is — a
			// thousand people at the finish line, not one person with a script.
			cookie := cookies[worker%len(cookies)]
			for url := range work {
				req, _ := http.NewRequest(http.MethodGet, url, nil)
				req.AddCookie(cookie)
				if scenario == "thumb-cached" {
					// Drive the conditional-GET path: the ETag is the content hash, and the whole
					// point of `immutable` is that the second visit costs a 304 rather than a
					// transfer. Measuring only cold reads would miss the mitigation.
					req.Header.Set("If-None-Match", `"`+etagFor(url)+`"`)
				}
				t0 := time.Now()
				resp, err := client.Do(req)
				d := time.Since(t0)
				status := 0
				if err == nil {
					status = resp.StatusCode
					// Drained and closed, or keep-alive cannot reuse the connection and the
					// measurement becomes one of TCP setup.
					_, _ = io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}
				mu.Lock()
				results = append(results, result{dur: d, status: status})
				mu.Unlock()
			}
		}(i)
	}

	for i := 0; i < requests; i++ {
		work <- targets[i%len(targets)]
	}
	close(work)
	wg.Wait()
	elapsed := time.Since(start)

	report(scenario, results, elapsed)
	return nil
}

// etagFor is filled in by buildTargets for the cached scenario; see targetETags.
var targetETags = map[string]string{}

func etagFor(url string) string { return targetETags[url] }

// mintCookies signs sessions for real directory members.
//
// # Why not log in properly
//
// The login path needs a PIN delivered to a phone number, and every account that can log in to this
// dev stack belongs to a real 2026 member — in the spejder case, a minor. Driving the SMS flow would
// mean handling those numbers to measure a SELECT.
//
// So cookies are signed directly with the dev `SESSION_SECRET`, for person ids read from the
// projection. **The ids are never logged**: the feed handler needs a caller that resolves in the
// directory, and nothing here needs to know who they are.
//
// # Why more than one
//
// The read limiter is keyed by member (PRD 019, task 311), so a run as a single member measures the
// limiter rather than the endpoint — which is exactly what the first run of this tool did. It is also
// the more faithful scenario: §0a.3 is a thousand people at the finish line.
// # Which rows are eligible
//
// `deleted = 0` is not decoration: `person.Get` filters on it, so a tombstoned row produces a session
// whose caller does not resolve, and every request then answers **404** from the "caller has no
// directory record" branch. The first multi-user run of this tool picked those up and reported a 23%
// error rate that was entirely its own fault — and 404s are fast, so they were also flattering the
// percentiles. If a run reports 404s, this query is the first place to look.
func mintCookies(db *sql.DB, year, secret string, n int) ([]*http.Cookie, string, error) {
	if n < 1 {
		n = 1
	}
	rows, err := db.Query(`
		SELECT personId, appRole FROM person
		WHERE year = ? AND appRole <> '' AND deleted = 0 AND personId NOT LIKE 'loadtest-%'
		ORDER BY personId LIMIT ?`, year, n)
	if err != nil {
		return nil, "", fmt.Errorf("read directory: %w", err)
	}
	defer rows.Close()

	mgr := session.NewManager([]byte(secret), 7*24*time.Hour, false)
	var out []*http.Cookie
	roles := map[string]int{}
	for rows.Next() {
		var personID, role string
		if err := rows.Scan(&personID, &role); err != nil {
			return nil, "", err
		}
		rec := httptest.NewRecorder()
		mgr.Issue(rec, personID, role)
		for _, c := range rec.Result().Cookies() {
			if c.Name == session.CookieName {
				out = append(out, c)
				roles[role]++
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	if len(out) == 0 {
		return nil, "", fmt.Errorf("no directory members to act as for year %s", year)
	}

	// Roles only, never ids — and the counts matter, because visibility differs by role and a run
	// made entirely of one role is measuring one filter.
	summary := make([]string, 0, len(roles))
	for role, count := range roles {
		summary = append(summary, fmt.Sprintf("%s×%d", role, count))
	}
	sort.Strings(summary)
	return out, strings.Join(summary, " "), nil
}

// buildTargets picks the URLs to hammer, from the seeded corpus.
func buildTargets(db *sql.DB, base, year, scenario string) ([]string, error) {
	switch scenario {
	case "feed":
		// Paging, which is what a browse actually does — including deep offsets, where a
		// LIMIT/OFFSET scan costs the most.
		var out []string
		for offset := 0; offset <= 900; offset += 20 {
			out = append(out, fmt.Sprintf("%s/api/glimt/feed?limit=20&offset=%d", base, offset))
		}
		return out, nil

	case "hold":
		rows, err := db.Query(`
			SELECT DISTINCT teamNumber FROM glimt
			WHERE year = ? AND teamNumber <> '' AND deleted = 0`, year)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				return nil, err
			}
			out = append(out, fmt.Sprintf("%s/api/glimt/hold/%s", base, n))
		}
		return out, rows.Err()

	case "thumb", "thumb-cached", "media":
		variant := "?variant=thumb"
		refCol := "m.thumbRef"
		if scenario == "media" {
			variant = ""
			refCol = "m.blobRef"
		}
		// Only `nathejk` and `public` media, so every target is visible to every acting member.
		//
		// Without this the run reported ~18% 403s — which is `users.MaySeeGlimt` working exactly as
		// intended, refusing one group's photographs to another group's member. But a 403 is decided
		// **before** the blob is opened, so it is much cheaper than a real serve, and a fifth of the
		// sample coming back as cheap refusals silently flatters every percentile. The visibility
		// check has its own tests; this scenario is here to measure the serving path.
		rows, err := db.Query(`
			SELECT m.glimtId, m.ordinal, `+refCol+`
			FROM glimt_media m JOIN glimt g ON g.glimtId = m.glimtId AND g.year = m.year
			WHERE m.year = ? AND g.deleted = 0 AND g.hiddenAt IS NULL
			  AND g.audience IN ('nathejk','public')
			LIMIT 3000`, year)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var id, ref string
			var ordinal int
			if err := rows.Scan(&id, &ordinal, &ref); err != nil {
				return nil, err
			}
			u := fmt.Sprintf("%s/api/glimt/items/%s/media/%d%s", base, id, ordinal, variant)
			out = append(out, u)
			targetETags[u] = ref
		}
		return out, rows.Err()

	default:
		return nil, fmt.Errorf("unknown scenario %q", scenario)
	}
}

func report(scenario string, results []result, elapsed time.Duration) {
	if len(results) == 0 {
		log.Println("no results")
		return
	}
	durs := make([]time.Duration, 0, len(results))
	codes := map[int]int{}
	for _, r := range results {
		durs = append(durs, r.dur)
		codes[r.status]++
	}
	sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })

	ok := codes[200] + codes[304]
	errRate := float64(len(results)-ok) / float64(len(results)) * 100

	fmt.Printf("\n=== %s ===\n", scenario)
	fmt.Printf("requests    %d in %s (%.0f req/s)\n", len(results), elapsed.Round(time.Millisecond),
		float64(len(results))/elapsed.Seconds())
	fmt.Printf("p50         %s\n", pct(durs, 50))
	fmt.Printf("p95         %s\n", pct(durs, 95))
	fmt.Printf("p99         %s\n", pct(durs, 99))
	fmt.Printf("max         %s\n", durs[len(durs)-1].Round(time.Microsecond))
	fmt.Printf("error rate  %.2f%%\n", errRate)
	fmt.Printf("statuses    %v\n", codes)
}

func pct(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	i := p * len(sorted) / 100
	if i >= len(sorted) {
		i = len(sorted) - 1
	}
	return sorted[i].Round(time.Microsecond)
}
