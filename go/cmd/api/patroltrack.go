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

	members, err := r.people.TrackMembers(r.year, teamID)
	if err != nil {
		return patroltrack.Track{}, fmt.Errorf("reading patrol members: %w", err)
	}
	if len(members) == 0 {
		// No members is not an error — a personnel "team", or a patrol whose records have gone. Cached
		// like any other answer, so a page for a nonexistent patrol does not re-query on every refresh.
		r.store(teamID, patroltrack.Track{})
		return patroltrack.Track{}, nil
	}

	groups, err := r.groupsFor(members)
	if err != nil {
		return patroltrack.Track{}, err
	}

	track := patroltrack.Merge(groups)
	r.store(teamID, track)
	return track, nil
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
