package main

import (
	"fmt"
	"sync"
	"time"

	"nathejk.dk/internal/patroltrack"
	"nathejk.dk/nathejk/table/person"
	"nathejk.dk/nathejk/table/trackpoint"
)

// The patrol's drawable route, composed and cached (PRD 011 §6, §8; task 340).
//
// # Three reads, one answer
//
// Who is in the patrol (`person.TrackMembers`), what those people recorded (`trackpoint.ByPeople`), and the
// merge that turns it into unattributed segments (`internal/patroltrack`). The composition lives here
// rather than in a projection for the reason `internal/reveal` records: a `nathejk/table/*` package may not
// read another's tables, and this needs two.
//
// # A member who left the race stops counting (task 349)
//
// PRD 011 §0b.6: there are no transfer sections, and a person who is no longer active in the race may be
// sitting in a car with their phone still recording. Their later points are not the patrol walking, so they
// are cut off at the moment their status changed — and where that moment is unknown, all of their points
// are excluded rather than guessed at.
//
// **This is the one filter that cannot live in `internal/patroltrack`**, and the reason is a property worth
// keeping: `trackpoint.ByPeople` returns groups with the person id thrown away, precisely so the merge
// cannot attribute a segment to anybody. So a per-person rule has to be applied *before* the points reach
// the merge, which is here.
//
// # The cache is not an optimisation, it is the requirement
//
// PRD 011 §8.4's actual concern was never storage \u2014 it was that **a public page must not do bulk work per
// request**. A patrol of six at 30 s sampling is ~8,600 points; merging and simplifying that is cheap once
// and wasteful a hundred times, and this route is unauthenticated, so "a hundred times" is somebody's
// afternoon rather than a load test.
//
// So the merged result is cached per patrol. The cache is deliberately simple \u2014 a map with a TTL, no
// eviction policy, no size bound \u2014 and that is safe because of what it holds: one entry per patrol per
// event, a few hundred entries of a few hundred points. A patrol's route also does not change after the
// race, which is what makes a long TTL correct rather than merely tolerable.

// patrolTrackTTL is how long a merged route is kept.
//
// Long, because the input is immutable in practice: a patrol's recorded points stop arriving when the race
// ends, and this page only opens once the patrol has finished (task 330). The one thing that *can* change is
// a late-arriving batch from a phone that was offline for hours \u2014 which is exactly why this is not
// `forever`. An hour means such a batch appears on the page within an hour of reaching us, and costs one
// merge per patrol per hour in the worst case.
const patrolTrackTTL = time.Hour

// patrolTrackReader composes and caches the merged route.
type patrolTrackReader struct {
	people person.Queries
	points trackpoint.Queries
	year   string

	mu    sync.Mutex
	cache map[string]patrolTrackEntry
	// inflight holds the merge currently running for a team, so concurrent callers wait for it rather than
	// starting their own. See Track — this is the difference between a cache and a lever.
	inflight map[string]*inflightTrack
	now      func() time.Time
}

type patrolTrackEntry struct {
	track patroltrack.Track
	at    time.Time
}

// inflightTrack is one merge in progress, and the answer it produced.
type inflightTrack struct {
	done  chan struct{}
	track patroltrack.Track
	err   error
}

// newPatrolTrackReader builds the reader, or nil when either projection is missing.
//
// Nil rather than a partially-working reader, following the reveal rule's precedent: a reader that could
// find the members but not their points would answer "this patrol recorded nothing", which is a legitimate
// state and therefore indistinguishable from the outage. The page must be able to tell those apart.
func newPatrolTrackReader(people person.Queries, points trackpoint.Queries, year string) *patrolTrackReader {
	if people == nil || points == nil {
		return nil
	}
	return &patrolTrackReader{
		people:   people,
		points:   points,
		year:     year,
		cache:    map[string]patrolTrackEntry{},
		inflight: map[string]*inflightTrack{},
		now:      time.Now,
	}
}

// Track returns the patrol's merged, unattributed route.
//
// An empty track is a normal answer and not an error: task 082 measured 2% coverage, and plenty of members
// never grant location at all. The page must be worth opening with an empty map — see PRD 011 §5.
//
// # Concurrent callers share one merge, and that is a security property
//
// A plain read-through cache is not enough on an unauthenticated route. Two hundred visitors arriving in the
// same second — the morning-after case, and the normal one — all miss a cold cache, so all two hundred walk
// the telemetry projection and merge it. Measured under `-race` in task 347's burst test before this was
// added: 400 requests produced two merges when the scheduler interleaved them, and nothing bounds that
// number except timing. A stranger with a concurrent loop has a lever.
//
// So a miss is claimed: the first caller computes and the rest **wait for its answer**. That makes the cost
// one merge per patrol per TTL regardless of how many people ask, which is what task 340 required and what a
// read-through cache alone does not deliver.
func (r *patrolTrackReader) Track(teamID string) (patroltrack.Track, error) {
	if teamID == "" {
		return patroltrack.Track{}, nil
	}

	if cached, ok := r.cached(teamID); ok {
		return cached, nil
	}

	flight, mine := r.claim(teamID)
	if !mine {
		// Somebody else is already merging this patrol. Waiting costs a blocked goroutine and saves a
		// duplicate walk of the projection — and the answer is identical, because the inputs are.
		<-flight.done
		return flight.track, flight.err
	}

	track, err := r.merge(teamID)

	r.mu.Lock()
	delete(r.inflight, teamID)
	if err == nil {
		r.cache[teamID] = patrolTrackEntry{track: track, at: r.now()}
	}
	r.mu.Unlock()

	// Published *after* the map is tidied, and before the waiters are released, so nobody reads a
	// half-written result.
	flight.track, flight.err = track, err
	close(flight.done)

	return track, err
}

// claim returns the in-flight merge for a team, and whether the caller owns it.
//
// The cache is re-checked under the lock: between the miss in Track and this call another goroutine may have
// finished and stored an answer, and starting a second merge because of that window would defeat the point.
func (r *patrolTrackReader) claim(teamID string) (*inflightTrack, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if entry, ok := r.cache[teamID]; ok && r.now().Sub(entry.at) <= patrolTrackTTL {
		// Already answered while we were getting here. Hand back a closed flight carrying it rather than
		// a second code path for the caller to get wrong.
		done := make(chan struct{})
		close(done)
		return &inflightTrack{done: done, track: entry.track}, false
	}
	if flight, ok := r.inflight[teamID]; ok {
		return flight, false
	}
	flight := &inflightTrack{done: make(chan struct{})}
	r.inflight[teamID] = flight
	return flight, true
}

// merge does the actual work: who is in the patrol, what they recorded, and the unattributed merge.
//
// Storing the result is Track's job, not this function's — it has to happen together with releasing the
// in-flight claim, or a waiter could be released before the answer is cached and immediately start another
// merge.
func (r *patrolTrackReader) merge(teamID string) (patroltrack.Track, error) {
	members, err := r.people.TrackMembers(r.year, teamID)
	if err != nil {
		return patroltrack.Track{}, fmt.Errorf("reading patrol members: %w", err)
	}
	if len(members) == 0 {
		// No members is not an error — a personnel "team", or a patrol whose records have gone. Cached
		// like any other answer, so a page for a nonexistent patrol does not re-query on every refresh.
		return patroltrack.Track{}, nil
	}

	groups, err := r.groupsFor(members)
	if err != nil {
		return patroltrack.Track{}, err
	}

	return patroltrack.Merge(groups), nil
}

// groupsFor reads each member's points, applying the race-status cutoff per member.
//
// # Why the reads are split
//
// `ByPeople` is one query for the whole patrol and returns groups with **no person id on them** — by
// design, since that is what makes the merged track unattributable. That means a per-member cutoff cannot
// be applied to its result: there is no way to tell whose group is whose.
//
// So the members who need no cutoff (the usual case: everybody) are read in one query as before, and each
// member who withdrew mid-race is read on their own so their points can be trimmed. A patrol has at most
// eight members and withdrawals are rare, so this is one query plus a small number — and the whole result
// is cached per patrol for an hour.
func (r *patrolTrackReader) groupsFor(members []person.TrackMember) ([][]trackpoint.Point, error) {
	var whole []string
	var trimmed []person.TrackMember

	for _, m := range members {
		if m.PersonID == "" {
			continue
		}
		if stillInRace(m.MemberStatus) {
			whole = append(whole, m.PersonID)
			continue
		}
		if m.StatusAt == nil {
			// **Left the race, and we do not know when.** Excluded entirely. That loses the kilometres
			// they did walk, which is a real cost — but the alternative is counting a car, and the label
			// says *mindst*: the figure is allowed to be low and is not allowed to be high.
			continue
		}
		trimmed = append(trimmed, m)
	}

	var groups [][]trackpoint.Point
	if len(whole) > 0 {
		read, err := r.points.ByPeople(r.year, whole)
		if err != nil {
			return nil, fmt.Errorf("reading track points: %w", err)
		}
		groups = append(groups, read...)
	}

	for _, m := range trimmed {
		read, err := r.points.ByPeople(r.year, []string{m.PersonID})
		if err != nil {
			return nil, fmt.Errorf("reading track points: %w", err)
		}
		for _, group := range read {
			if kept := untilStatusChange(group, *m.StatusAt); len(kept) > 0 {
				groups = append(groups, kept)
			}
		}
	}
	return groups, nil
}

// untilStatusChange keeps the points recorded before a member left the race.
//
// Strictly before: a point stamped at the same millisecond as the withdrawal is the moment they stopped
// walking, not a step they took. The points arrive in time order, so this is a prefix rather than a filter —
// but it is written as a filter anyway, because relying on the ordering of somebody else's query result is
// how a reasonable change to that query becomes a silent data bug here.
func untilStatusChange(points []trackpoint.Point, leftAt time.Time) []trackpoint.Point {
	cutoff := leftAt.UTC().UnixMilli()
	kept := make([]trackpoint.Point, 0, len(points))
	for _, p := range points {
		if p.TS < cutoff {
			kept = append(kept, p)
		}
	}
	return kept
}

func (r *patrolTrackReader) cached(teamID string) (patroltrack.Track, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.cache[teamID]
	if !ok || r.now().Sub(entry.at) > patrolTrackTTL {
		return patroltrack.Track{}, false
	}
	return entry.track, true
}

// trackPointQueriesOrNil narrows the projection to its read API, or nil.
//
// The same typed-nil guard as the other projections: a nil `*trackpoint.Table` in an interface field is not
// `== nil`, so `newPatrolTrackReader`'s check would pass and the first read would panic inside a request.
func trackPointQueriesOrNil(t *trackpoint.Table) trackpoint.Queries {
	if t == nil {
		return nil
	}
	return t
}
