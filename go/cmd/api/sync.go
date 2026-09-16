package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"sort"
	"time"

	"nathejk.dk/internal/users"
)

// One multiplexed freshness check for every dataset the caller holds (PRD 017, task 284).
//
// # Why one endpoint and not six
//
// `useFreshnessLoop.ts` established the convention — a separate, cheap, opaque version endpoint per
// dataset — and it was right about *what* a version endpoint is. It was written when there was one
// dataset. At six, honouring its intent means multiplexing: six endpoints would be six requests per
// foreground, from a few hundred devices, on a mobile link where the round-trip count dominates the
// payload size by a wide margin. Everything else the convention argued for is preserved here —
// small, opaque, per-caller, answered from a projection read, ETag-able, version in the body.
//
// # This endpoint composes. It must never build a payload
//
// Every version here comes from an existing `*VersionFor` (contacts.go, mapversion.go,
// syncversion.go), each of which is a projection read or a short-lived cache hit. The moment
// something in this file starts assembling a manifest to hash it, the endpoint has lost its reason to
// exist: it would then cost what it was built to avoid.
//
// # Absence and unavailability are different answers, and the difference is load-bearing
//
// A dataset the caller may not hold is simply **not a key**. That is how a spejder's device learns
// not to ask for a directory — better than the arrangement it replaces, where the client's own role
// table decided and any disagreement with the BFF produced a 403 per foreground.
//
// But every derivation can also fail, and a failure must not be reported as absence. Absence means
// "you may not hold this", which a client cannot recover from: it would stop asking, and nothing
// would ever tell it otherwise. So a failed derivation is **named in `unavailable`**, which the
// client reads as "unchanged, ask again". One failing projection read therefore costs its own dataset
// a cycle of freshness and costs the other five nothing.

// syncResponse is the whole answer: a handful of short strings.
//
// If this grows, something in it belongs in a payload instead. In particular it deliberately carries
// no event state — race started, an active SOS, a new update post — however tempting that is, because
// that is precisely how a small endpoint becomes a large one (PRD 017 §11).
type syncResponse struct {
	// Versions maps dataset name to an opaque version, for the datasets this caller may hold.
	//
	// Opaque is a contract, not an implementation detail: the client may only compare these for
	// equality. Nothing may parse one, order two, or read a timestamp out of one.
	Versions map[string]string `json:"versions"`

	// Unavailable names the datasets whose version could not be derived this time.
	//
	// Normally empty, and when it is not, that is a server fault worth alerting on rather than a
	// state of the event. Never null in JSON — a client should not have to handle two spellings of
	// "nothing here".
	Unavailable []string `json:"unavailable"`

	// IntervalSeconds is how long the client should wait between checks while the app is open.
	//
	// Served here, and not only in `/api/config`, for one reason: the 02:00 lever. An operator
	// shedding load during an event needs a change to take effect on the next check, on every
	// device, without waiting for anyone to refetch config. Zero disables the **interval only** —
	// foreground, reconnect and manual checks keep running, which is the distinction an operator
	// would get wrong and therefore the one with its own test.
	IntervalSeconds int `json:"interval_seconds"`

	// DebounceSeconds is the minimum gap between checks, ignored for a user-requested refresh.
	//
	// Served for the same reason as the interval: it is the other half of the traffic this endpoint
	// generates, and tuning one without the other would leave a foregrounding-heavy crowd
	// (unlock, glance at the map, lock, unlock) untouched by a widened interval.
	DebounceSeconds int `json:"debounce_seconds"`
}

// syncDataset is one entry in the table below: what it is called on the wire, whether this caller
// holds it, and how to derive its version.
type syncDataset struct {
	name string
	// holds reports whether this caller may hold the dataset at all. False means the key is absent,
	// which tells the client not to ask for the payload either.
	holds func(users.User) bool
	// version derives the caller's version. Errors are collected into `unavailable`, never returned
	// to the client as a failure of the whole check.
	version func(users.User) (string, error)
}

// syncDatasets is the authoritative answer to "what may this caller hold".
//
// A table rather than a sequence of ifs, because the risk this endpoint carries is a dataset being
// *forgotten* — added to the app, never added to the check, and silently frozen on every device
// until the next cold start. A list is something a reviewer can compare against the app's panes.
func (app *application) syncDatasets() []syncDataset {
	hasPatrol := func(u users.User) bool { return u.PatrolID != "" }
	always := func(users.User) bool { return true }

	return []syncDataset{
		// The directory. Spejdere do not get the pane, so they do not get the key — the same rule
		// `contactsViewer` enforces with a 403, applied here by omission instead.
		{name: "contacts", holds: func(u users.User) bool { return users.MayUseContacts(u.Role) }, version: app.contactsVersionFor},
		// The caller's own record. Everyone has one, including personnel with no patrol.
		{name: "profile", holds: always, version: app.profileVersionFor},
		// The three patrol-scoped map datasets. Absent for personnel, exactly as their endpoints
		// already return empty for them — so the device never learns a version for something it has
		// no business asking about.
		{name: "scans", holds: hasPatrol, version: app.scansVersionFor},
		{name: "handouts", holds: hasPatrol, version: app.handoutsVersionFor},
		{name: "checkpoints", holds: hasPatrol, version: app.checkpointsVersionFor},
		// Shared by the whole event, and the cheapest of the six: one cache entry serves every
		// device.
		{name: "race_area", holds: always, version: app.raceAreaVersionFor},
	}
}

// @Summary      Freshness check for every dataset the caller holds
// @Description  Returns one small opaque version per dataset, so a client can ask "did anything I hold change?" in a single request rather than one per dataset. Compare each version for equality with the one held and refetch only the datasets that differ; the versions carry no parseable meaning. A dataset the caller may not hold is ABSENT from `versions` — that is how a client learns not to request it. A dataset whose version could not be derived is listed in `unavailable` instead, which means "unchanged, ask again": absence and unavailability are different answers, and a client must not treat a transient failure as a permission decision. `interval_seconds` and `debounce_seconds` are served here rather than only in /config so that load can be shed during an event without waiting for a config refetch; zero interval disables the periodic check only, leaving foreground, reconnect and manual checks running.
// @Tags         sync
// @Produce      json
// @Param        If-None-Match  header    string  false  "versions held by the client"
// @Success      200  {object}  syncResponse
// @Success      304  "nothing changed"
// @Failure      401  {object}  map[string]string
// @Router       /sync [get]
func (app *application) syncHandler(w http.ResponseWriter, r *http.Request) {
	// Measured around the whole handler, including the derivations, because that is what a device waits
	// for (PRD 017 §8 asks for "the endpoint's own timing").
	started := time.Now()

	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}
	viewer, found := app.models.Users.Get(s.UserID)
	if !found {
		// A session whose user no longer resolves. 404 rather than an empty-but-successful check,
		// which would look to the client like "you hold nothing" and quietly stop all refreshing.
		app.NotFoundResponse(w, r)
		return
	}

	out := syncResponse{
		Versions:        map[string]string{},
		Unavailable:     []string{},
		IntervalSeconds: app.config.syncIntervalSeconds,
		DebounceSeconds: app.config.syncDebounceSeconds,
	}

	for _, ds := range app.syncDatasets() {
		if !ds.holds(viewer) {
			continue
		}
		version, err := ds.version(viewer)
		if err != nil {
			// Logged, not returned. The other five datasets are still answerable, and a check that
			// failed wholesale because one projection hiccuped would strand every dataset on the
			// device rather than one.
			//
			// Error level and counted separately (task 293): a non-empty `unavailable` is a server
			// fault rather than a state of the event, and it is the one thing in this response worth
			// alerting on instead of reviewing afterwards.
			app.Logger.Error("deriving sync version", "err", err, "dataset", ds.name, "userId", viewer.ID)
			app.syncMetrics.recordUnavailable(ds.name)
			out.Unavailable = append(out.Unavailable, ds.name)
			continue
		}
		out.Versions[ds.name] = version
	}

	etag := `"` + syncETag(out.Versions, out.Unavailable) + `"`
	w.Header().Set("ETag", etag)
	// A second line of defence against the check's cost, copied from the contacts version endpoint:
	// if a client misbehaves and checks far more often than the served debounce, the browser's own
	// cache absorbs it before the request reaches us.
	w.Header().Set("Cache-Control", "private, max-age=5")

	// The 304 is also the measurement (task 293). It means every dataset this caller holds is unchanged
	// — the device said so by sending a matching `If-None-Match` — which is exactly PRD 017 §9's ≥ 95 %
	// target, measured without asking the client to send anything extra.
	unchanged := r.Header.Get("If-None-Match") == etag
	app.syncMetrics.recordCall(unchanged, time.Since(started))
	if unchanged {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	if err := app.WriteJSON(w, http.StatusOK, out, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}

	// Last, and after the response: a summary is a diagnostic, and nothing about it should sit between a
	// device and its answer on the endpoint every device calls on every foreground.
	if snapshot, due := app.syncMetrics.dueForSummary(); due {
		snapshot.logSummary(app.Logger)
	}
}

// syncETag hashes the answer so the browser can make its own conditional requests.
//
// Over the sorted keys, because Go's map iteration order is deliberately random and an ETag that
// changed per request would be worse than none: it would defeat the browser cache *and* claim the
// data had changed. The unavailable set is part of it, so recovering from a failure is visible to a
// conditional request rather than being masked by a 304.
//
// Not over the intervals. They are policy rather than data, and an operator widening the interval
// during an event must not make every device believe all six datasets changed.
func syncETag(versions map[string]string, unavailable []string) string {
	names := make([]string, 0, len(versions))
	for name := range versions {
		names = append(names, name)
	}
	sort.Strings(names)

	h := sha256.New()
	for _, name := range names {
		io.WriteString(h, name)
		io.WriteString(h, "=")
		io.WriteString(h, versions[name])
		io.WriteString(h, "\x1e")
	}
	io.WriteString(h, "\x1d")
	sorted := append([]string{}, unavailable...)
	sort.Strings(sorted)
	for _, name := range sorted {
		io.WriteString(h, name)
		io.WriteString(h, "\x1e")
	}
	return hex.EncodeToString(h.Sum(nil)[:16])
}
