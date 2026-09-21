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
// Who is in the patrol (`person.MemberIDs`), what those people recorded (`trackpoint.ByPeople`), and the
// merge that turns it into unattributed segments (`internal/patroltrack`). The composition lives here
// rather than in a projection for the reason `internal/reveal` records: a `nathejk/table/*` package may not
// read another's tables, and this needs two.
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
	now   func() time.Time
}

type patrolTrackEntry struct {
	track patroltrack.Track
	at    time.Time
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
		people: people,
		points: points,
		year:   year,
		cache:  map[string]patrolTrackEntry{},
		now:    time.Now,
	}
}

// Track returns the patrol's merged, unattributed route.
//
// An empty track is a normal answer and not an error: task 082 measured 2% coverage, and plenty of members
// never grant location at all. The page must be worth opening with an empty map \u2014 see PRD 011 §5.
func (r *patrolTrackReader) Track(teamID string) (patroltrack.Track, error) {
	if teamID == "" {
		return patroltrack.Track{}, nil
	}

	if cached, ok := r.cached(teamID); ok {
		return cached, nil
	}

	memberIDs, err := r.people.MemberIDs(r.year, teamID)
	if err != nil {
		return patroltrack.Track{}, fmt.Errorf("reading patrol members: %w", err)
	}
	if len(memberIDs) == 0 {
		// No members is not an error \u2014 a personnel "team", or a patrol whose records have gone. Cached
		// like any other answer, so a page for a nonexistent patrol does not re-query on every refresh.
		r.store(teamID, patroltrack.Track{})
		return patroltrack.Track{}, nil
	}

	groups, err := r.points.ByPeople(r.year, memberIDs)
	if err != nil {
		return patroltrack.Track{}, fmt.Errorf("reading track points: %w", err)
	}

	track := patroltrack.Merge(groups)
	r.store(teamID, track)
	return track, nil
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

func (r *patrolTrackReader) store(teamID string, track patroltrack.Track) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache[teamID] = patrolTrackEntry{track: track, at: r.now()}
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
