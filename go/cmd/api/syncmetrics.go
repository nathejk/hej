package main

import (
	"sync"
	"time"
)

// Instrumentation for the freshness check (task 293, PRD 017 §8/§9).
//
// # What this is defending against
//
// The endpoint's whole economy rests on one assumption: almost every check finds nothing. If that is
// false, some version is being derived wrongly, and PRD 017 §8 is explicit that an event is the wrong
// time to discover it. There are two ways to be wrong and they fail in opposite directions:
//
//   - **A wrongly-unstable version** changes when its data did not, so every device refetches every
//     payload on every interval. Expensive, and visible in the numbers here.
//   - **A wrongly-stable version** never changes though its data did, so a device quietly never
//     updates. Silent, and the numbers here can only make it *suspicious*, never prove it.
//
// # What the server can honestly measure, and what it cannot
//
// The obvious instrumentation — "which datasets were unchanged for this device?" — is **not something
// this server knows**. Whether a dataset changed is a comparison between the version we computed and
// the version *the device holds*, and the device does not tell us: it sends its aggregate `ETag` and
// nothing more. Asking it to send six versions on every foreground would add request bytes to the
// cheapest endpoint in the API to satisfy a metric, which is the wrong trade (PRD 017 §6: "the response
// must stay tiny", and the same logic applies to the request).
//
// So this measures the two things that *are* knowable, and they turn out to be enough:
//
//  1. **The aggregate unchanged ratio, from the `ETag`.** A `304` means every dataset this caller holds
//     is unchanged — the device told us so by sending a matching `If-None-Match`. That is PRD 017 §9's
//     ≥ 95 % target, measured exactly, with no extra bytes.
//
//  2. **Per-dataset version churn, from a witness sample.** For a bounded sample of cache keys per
//     dataset, remember the last version computed and count how often the next computation differs.
//     A wrongly-unstable version churns for *every* key, so a sample of a few dozen catches it as
//     decisively as full coverage would — and unlike full coverage, it cannot grow into the leak task
//     295 just fixed.
//
// Churn is also the per-dataset signal PRD 017 §9 asks for, from the other side: it attributes cost to a
// dataset without needing any device to report anything.

// datasetStats is what we know about one dataset's version derivation.
type datasetStats struct {
	// Computed counts derivations, i.e. cache misses. Not calls: a cache hit tells us nothing new.
	Computed int64
	// Changed counts derivations where a witnessed key's version differed from the last one we saw
	// for that key. Only witnessed keys contribute, so this is a rate over `Witnessed`.
	Changed int64
	// Witnessed counts derivations for keys in the sample, i.e. the denominator for `Changed`.
	Witnessed int64
	// Unavailable counts derivations that failed. Any non-zero value is a server fault.
	Unavailable int64
	// LastChange is when a witnessed version last moved. A dataset whose data plainly changed during an
	// event but whose LastChange is hours old is the wrongly-stable suspect.
	LastChange time.Time
}

// syncMetrics accumulates the endpoint's own numbers, in memory.
//
// In memory and unexported deliberately: this is a diagnostic for reading during and after an event, not
// a metrics pipeline. If it ever needs to survive a restart or be scraped, that is a decision with its
// own trade-offs (a Prometheus endpoint on a participant-facing BFF is not free), and it should be made
// then rather than assumed now.
type syncMetrics struct {
	mu sync.Mutex

	calls       int64
	notModified int64

	// Timing, as count/total/max rather than a histogram. Enough to answer "is this endpoint cheap?"
	// during an event, and deliberately not enough to answer p95 — that belongs to the load test (task
	// 291), which drives the traffic and can measure from outside. A histogram here would be a
	// percentile estimate nobody could check.
	totalDuration time.Duration
	maxDuration   time.Duration

	datasets map[string]*datasetStats

	// witnesses samples cache keys per dataset: dataset -> key -> last version computed.
	//
	// Bounded by witnessLimit per dataset, first-come. First-come rather than random or LRU because a
	// wrongly-unstable version is wrongly unstable for every key, so *which* keys are watched does not
	// matter — only that some are, cheaply and forever.
	witnesses map[string]map[string]string

	// lastSummary is when the periodic summary was last emitted. Zero until the first one.
	lastSummary time.Time

	now func() time.Time
}

// witnessLimit is how many keys per dataset are watched for churn.
//
// A few dozen is plenty: the failure this detects affects every key, so the sample size affects how fast
// it is noticed, not whether. Small enough that the whole structure is a rounding error next to the
// caches it observes.
const witnessLimit = 32

func newSyncMetrics() *syncMetrics {
	return &syncMetrics{
		datasets:  map[string]*datasetStats{},
		witnesses: map[string]map[string]string{},
		now:       time.Now,
	}
}

// recordCall counts one answered check, whether it was a 304, and what it cost.
func (m *syncMetrics) recordCall(notModified bool, took time.Duration) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if notModified {
		m.notModified++
	}
	m.totalDuration += took
	if took > m.maxDuration {
		m.maxDuration = took
	}
}

// recordDerivation notes that `dataset` computed `version` for `key`, and whether that is a change.
//
// Called only on a cache miss. A cache hit is not evidence about the version's stability — it is
// evidence about the cache — and counting hits as "unchanged" would flatter every dataset by the hit
// rate, turning this measurement into a restatement of the TTL.
func (m *syncMetrics) recordDerivation(dataset, key, version string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	stats := m.statsLocked(dataset)
	stats.Computed++

	witnesses, ok := m.witnesses[dataset]
	if !ok {
		witnesses = map[string]string{}
		m.witnesses[dataset] = witnesses
	}

	previous, watched := witnesses[key]
	if !watched {
		if len(witnesses) >= witnessLimit {
			// Not in the sample and the sample is full. Counted as computed, not as witnessed, so it
			// cannot skew the churn rate in either direction.
			return
		}
		// First sighting: nothing to compare against yet, so it joins the sample without being counted
		// as unchanged. Recording it as unchanged would understate churn on a fresh process.
		witnesses[key] = version
		return
	}

	stats.Witnessed++
	if previous != version {
		stats.Changed++
		stats.LastChange = m.now()
		witnesses[key] = version
	}
}

// recordUnavailable notes a derivation that failed. Any non-zero count is a server fault, not an event
// state — see the handler, which also logs it at error level.
func (m *syncMetrics) recordUnavailable(dataset string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statsLocked(dataset).Unavailable++
}

func (m *syncMetrics) statsLocked(dataset string) *datasetStats {
	stats, ok := m.datasets[dataset]
	if !ok {
		stats = &datasetStats{}
		m.datasets[dataset] = stats
	}
	return stats
}

// syncMetricsSnapshot is a copy for reading, so a reader never holds the lock or races a writer.
type syncMetricsSnapshot struct {
	Calls         int64
	NotModified   int64
	TotalDuration time.Duration
	MaxDuration   time.Duration
	Datasets      map[string]datasetStats
}

func (m *syncMetrics) snapshot() syncMetricsSnapshot {
	if m == nil {
		return syncMetricsSnapshot{Datasets: map[string]datasetStats{}}
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	out := syncMetricsSnapshot{
		Calls:         m.calls,
		NotModified:   m.notModified,
		TotalDuration: m.totalDuration,
		MaxDuration:   m.maxDuration,
		Datasets:      make(map[string]datasetStats, len(m.datasets)),
	}
	for name, stats := range m.datasets {
		out.Datasets[name] = *stats
	}
	return out
}

// MeanDuration is the average time to answer a check. Zero when nothing has been measured.
func (s syncMetricsSnapshot) MeanDuration() time.Duration {
	if s.Calls == 0 {
		return 0
	}
	return s.TotalDuration / time.Duration(s.Calls)
}

// UnchangedRatio is the share of checks that found nothing, i.e. PRD 017 §9's ≥ 95 % target.
//
// Returns -1 rather than 0 for "no calls yet", because a zero here would read as "every check found
// something" — the alarming answer — when the truth is that nothing has been measured.
func (s syncMetricsSnapshot) UnchangedRatio() float64 {
	if s.Calls == 0 {
		return -1
	}
	return float64(s.NotModified) / float64(s.Calls)
}

// ChurnRatio is how often a witnessed key's version moved when recomputed.
//
// The number to read per dataset. Near 1 means the version changes on essentially every derivation,
// which for a dataset that is not changing that often means it is **wrongly unstable** — every device
// refetching every payload every interval. Near 0 during a period when the data demonstrably changed
// makes it a **wrongly-stable** suspect, which needs the cross-check by hand: this cannot tell the
// difference between "nothing changed" and "we failed to notice".
//
// -1 for "nothing witnessed yet", for the same reason as above.
func (d datasetStats) ChurnRatio() float64 {
	if d.Witnessed == 0 {
		return -1
	}
	return float64(d.Changed) / float64(d.Witnessed)
}

// summaryInterval is how often the endpoint logs its own numbers.
//
// Frequent enough to watch during an event, rare enough that the log stays readable. Deliberately not
// per call: this is the endpoint every device hits on every foreground, so a line per call would bury
// everything else in the log and tell a reader less — a ratio is the unit of meaning here, not a call.
const summaryInterval = 5 * time.Minute

// dueForSummary reports whether it is time to log the numbers, and hands back a snapshot if so.
//
// Driven by calls rather than by a goroutine and ticker: a background timer would need a lifecycle and
// a shutdown path, and it would keep logging identical numbers on an idle server all night. This only
// speaks when something has happened.
func (m *syncMetrics) dueForSummary() (syncMetricsSnapshot, bool) {
	if m == nil {
		return syncMetricsSnapshot{}, false
	}

	m.mu.Lock()
	now := m.now()
	if m.lastSummary.IsZero() {
		// Not due immediately: the first window would otherwise report a ratio over one call, which is
		// noise that reads like a signal.
		m.lastSummary = now
		m.mu.Unlock()
		return syncMetricsSnapshot{}, false
	}
	if now.Sub(m.lastSummary) < summaryInterval {
		m.mu.Unlock()
		return syncMetricsSnapshot{}, false
	}
	m.lastSummary = now
	m.mu.Unlock()

	return m.snapshot(), true
}

// logSummary writes the numbers a reviewer actually needs, per dataset.
//
// Per-dataset rather than one overall figure, and PRD 017 §9 is explicit about why: an overall ratio
// hides a single wrongly-unstable version behind five well-behaved ones, which is exactly the fault this
// instrumentation exists to attribute.
func (s syncMetricsSnapshot) logSummary(log interface {
	Info(msg string, args ...any)
}) {
	log.Info("sync check summary",
		"calls", s.Calls,
		"unchanged", s.NotModified,
		"unchangedRatio", s.UnchangedRatio(),
		"meanMs", s.MeanDuration().Milliseconds(),
		"maxMs", s.MaxDuration.Milliseconds(),
	)
	for name, stats := range s.Datasets {
		log.Info("sync dataset summary",
			"dataset", name,
			"computed", stats.Computed,
			"witnessed", stats.Witnessed,
			"changed", stats.Changed,
			"churnRatio", stats.ChurnRatio(),
			"unavailable", stats.Unavailable,
			"lastChange", stats.LastChange,
		)
	}
}
