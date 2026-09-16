package main

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// What this instrumentation is for, and therefore what these tests defend:
//
//   - the aggregate unchanged ratio must be measured from the 304, because that is the only signal the
//     device actually sends about what it holds;
//   - per-dataset churn must be attributable, because an overall figure hides one wrongly-unstable
//     version behind five well-behaved ones (PRD 017 §9);
//   - a cache hit must never be counted as "unchanged", or the measurement degenerates into a
//     restatement of the TTL and flatters every dataset by the hit rate.

func TestSyncMetrics_UnchangedRatioFromNotModified(t *testing.T) {
	m := newSyncMetrics()

	m.recordCall(true, 2*time.Millisecond)
	m.recordCall(true, 2*time.Millisecond)
	m.recordCall(true, 2*time.Millisecond)
	m.recordCall(false, 6*time.Millisecond)

	snap := m.snapshot()
	if got := snap.UnchangedRatio(); got != 0.75 {
		t.Errorf("unchanged ratio = %v, want 0.75", got)
	}
	if got := snap.MeanDuration(); got != 3*time.Millisecond {
		t.Errorf("mean = %v, want 3ms", got)
	}
	if got := snap.MaxDuration; got != 6*time.Millisecond {
		t.Errorf("max = %v, want 6ms", got)
	}
}

// Zero calls must not read as "every check found something", which is the alarming answer. A reviewer
// glancing at 0.0 would draw exactly the wrong conclusion from a server nobody has used yet.
func TestSyncMetrics_NoCallsIsNotZeroRatio(t *testing.T) {
	if got := newSyncMetrics().snapshot().UnchangedRatio(); got != -1 {
		t.Errorf("ratio with no calls = %v, want -1 (unmeasured)", got)
	}
	var stats datasetStats
	if got := stats.ChurnRatio(); got != -1 {
		t.Errorf("churn with nothing witnessed = %v, want -1 (unmeasured)", got)
	}
}

func TestSyncMetrics_ChurnIsPerDataset(t *testing.T) {
	m := newSyncMetrics()

	// A stable dataset: same version every time, for the same key.
	for range 4 {
		m.recordDerivation("handouts", "team-1", "stable")
	}
	// A dataset whose version moves on every single derivation — the wrongly-unstable signature, and
	// the fault an overall ratio would hide.
	for i := range 4 {
		m.recordDerivation("scans", "team-1", fmt.Sprintf("v%d", i))
	}

	snap := m.snapshot()

	if got := snap.Datasets["handouts"].ChurnRatio(); got != 0 {
		t.Errorf("handouts churn = %v, want 0", got)
	}
	// First sighting joins the sample without a comparison, so three of the four are witnessed and all
	// three changed.
	if got := snap.Datasets["scans"].ChurnRatio(); got != 1 {
		t.Errorf("scans churn = %v, want 1", got)
	}
	if snap.Datasets["scans"].Computed != 4 {
		t.Errorf("computed = %d, want 4", snap.Datasets["scans"].Computed)
	}
}

// A first sighting has nothing to compare against. Counting it as unchanged would understate churn on a
// fresh process — i.e. right after a deploy, which is exactly when someone is looking.
func TestSyncMetrics_FirstSightingIsNotCountedUnchanged(t *testing.T) {
	m := newSyncMetrics()
	m.recordDerivation("profile", "u-1", "v1")

	stats := m.snapshot().Datasets["profile"]
	if stats.Witnessed != 0 {
		t.Errorf("witnessed = %d, want 0 for a first sighting", stats.Witnessed)
	}
	if stats.Computed != 1 {
		t.Errorf("computed = %d, want 1", stats.Computed)
	}
}

// The witness sample is bounded, so the instrumentation cannot become the leak task 295 just fixed —
// profile keys by user, so unbounded witnessing would grow one entry per user forever.
func TestSyncMetrics_WitnessSampleIsBounded(t *testing.T) {
	m := newSyncMetrics()

	for i := range witnessLimit * 10 {
		m.recordDerivation("profile", fmt.Sprintf("u-%d", i), "v1")
	}

	m.mu.Lock()
	size := len(m.witnesses["profile"])
	m.mu.Unlock()

	if size > witnessLimit {
		t.Errorf("witnessed %d keys; the sample must stay bounded", size)
	}
	// Every derivation is still counted, sampled or not — the sample bounds memory, not the call count.
	if got := m.snapshot().Datasets["profile"].Computed; got != int64(witnessLimit*10) {
		t.Errorf("computed = %d, want %d", got, witnessLimit*10)
	}
}

// Keys outside the sample must not skew the rate in either direction: they are counted as computed and
// nothing else. Otherwise the churn ratio would depend on how many users happened to call.
func TestSyncMetrics_UnsampledKeysDoNotSkewChurn(t *testing.T) {
	m := newSyncMetrics()

	// Fill the sample with one stable key each, then witness a change on the first.
	for i := range witnessLimit {
		m.recordDerivation("profile", fmt.Sprintf("u-%d", i), "v1")
	}
	m.recordDerivation("profile", "u-0", "v2")

	// Now hammer keys that cannot enter the sample.
	for i := range 100 {
		m.recordDerivation("profile", fmt.Sprintf("outsider-%d", i), "whatever")
	}

	stats := m.snapshot().Datasets["profile"]
	if stats.Witnessed != 1 || stats.Changed != 1 {
		t.Errorf("witnessed/changed = %d/%d, want 1/1", stats.Witnessed, stats.Changed)
	}
	if got := stats.ChurnRatio(); got != 1 {
		t.Errorf("churn = %v, want 1 — unsampled keys must not dilute it", got)
	}
}

func TestSyncMetrics_LastChangeMovesOnlyOnAChange(t *testing.T) {
	m := newSyncMetrics()
	now := time.Now()
	m.now = func() time.Time { return now }

	m.recordDerivation("checkpoints", "team-1", "v1")
	m.recordDerivation("checkpoints", "team-1", "v1")
	if !m.snapshot().Datasets["checkpoints"].LastChange.IsZero() {
		t.Error("lastChange must stay zero while nothing changes")
	}

	now = now.Add(time.Hour)
	m.recordDerivation("checkpoints", "team-1", "v2")
	if got := m.snapshot().Datasets["checkpoints"].LastChange; !got.Equal(now) {
		t.Errorf("lastChange = %v, want %v", got, now)
	}
}

func TestSyncMetrics_UnavailableIsCounted(t *testing.T) {
	m := newSyncMetrics()
	m.recordUnavailable("race_area")
	if got := m.snapshot().Datasets["race_area"].Unavailable; got != 1 {
		t.Errorf("unavailable = %d, want 1", got)
	}
}

// A nil metrics pointer is safe, so tests and any future caller need not wire it to exercise the
// endpoint. Cheap to guarantee, and the alternative is a nil panic on the app's busiest read.
func TestSyncMetrics_NilIsSafe(t *testing.T) {
	var m *syncMetrics
	m.recordCall(true, time.Millisecond)
	m.recordDerivation("scans", "team-1", "v1")
	m.recordUnavailable("scans")
	if _, due := m.dueForSummary(); due {
		t.Error("a nil metrics must never be due for a summary")
	}
	if got := m.snapshot().UnchangedRatio(); got != -1 {
		t.Errorf("nil snapshot ratio = %v, want -1", got)
	}
}

func TestSyncMetrics_SummaryIsPeriodicNotPerCall(t *testing.T) {
	m := newSyncMetrics()
	now := time.Now()
	m.now = func() time.Time { return now }

	// The very first check must not emit: a ratio over one call is noise that reads like a signal.
	m.recordCall(true, time.Millisecond)
	if _, due := m.dueForSummary(); due {
		t.Fatal("the first call must not trigger a summary")
	}

	// Nothing within the window either — this is the endpoint every device calls on every foreground.
	now = now.Add(summaryInterval - time.Second)
	if _, due := m.dueForSummary(); due {
		t.Error("a summary must not be emitted inside the interval")
	}

	now = now.Add(2 * time.Second)
	if _, due := m.dueForSummary(); !due {
		t.Error("a summary is due once the interval has passed")
	}
	// And the window resets, so it stays one line per interval however many calls arrive.
	if _, due := m.dueForSummary(); due {
		t.Error("the interval must reset after a summary")
	}
}

// End to end: the endpoint's own numbers must reflect real traffic through it, including that a 304 is
// what counts as unchanged.
func TestSync_MetricsReflectTraffic(t *testing.T) {
	app := syncApp(t)
	app.syncMetrics = newSyncMetrics()

	resp, _ := getSync(t, app, "+4530000001", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	etag := resp.Header.Get("ETag")

	again, _ := getSync(t, app, "+4530000001", etag)
	if again.StatusCode != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", again.StatusCode)
	}

	snap := app.syncMetrics.snapshot()
	if snap.Calls != 2 {
		t.Errorf("calls = %d, want 2", snap.Calls)
	}
	if snap.NotModified != 1 {
		t.Errorf("notModified = %d, want 1", snap.NotModified)
	}
}

// The cache is what observes derivations, so a *hit* must record nothing. If hits counted, the churn
// rate would silently become a report on the TTL rather than on the version.
func TestSync_CacheHitsAreNotCountedAsDerivations(t *testing.T) {
	m := newSyncMetrics()
	cache := newVersionCache(time.Hour).observedAs("contacts", m)

	cache.put("crew", "v1")
	for range 5 {
		if _, ok := cache.get("crew"); !ok {
			t.Fatal("expected a cache hit")
		}
	}

	if got := m.snapshot().Datasets["contacts"].Computed; got != 1 {
		t.Errorf("computed = %d, want 1 — only derivations count, not hits", got)
	}
}
