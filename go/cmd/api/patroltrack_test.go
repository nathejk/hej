package main

import (
	"errors"
	"testing"
	"time"

	"nathejk.dk/nathejk/table/person"
	"nathejk.dk/nathejk/table/trackpoint"
)

// The composed, cached patrol route (task 340).

// trackPeople answers TrackMembers and counts the calls.
type trackPeople struct {
	members map[string][]string
	err     error
	calls   int

	// status and statusAt override a member's lifecycle, keyed by person id (task 349). Absent means
	// "still racing", which is what most of these tests want.
	status   map[string]string
	statusAt map[string]time.Time
}

func (p *trackPeople) TrackMembers(_ string, teamID string) ([]person.TrackMember, error) {
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	out := make([]person.TrackMember, 0, len(p.members[teamID]))
	for _, id := range p.members[teamID] {
		m := person.TrackMember{PersonID: id, MemberStatus: p.status[id]}
		if at, ok := p.statusAt[id]; ok {
			t := at
			m.StatusAt = &t
		}
		out = append(out, m)
	}
	return out, nil
}

// The rest of person.Queries, unused here.
func (p *trackPeople) Lookup(string, string) ([]person.Person, error) { return nil, nil }
func (p *trackPeople) Get(string, string) (person.Person, bool, error) {
	return person.Person{}, false, nil
}
func (p *trackPeople) ListByAppRoles(string, []string) ([]person.Person, error)   { return nil, nil }
func (p *trackPeople) ListPatrolByNumber(string, string) ([]person.Person, error) { return nil, nil }
func (p *trackPeople) ExpiredPortraits(string, time.Time, int) ([]person.ExpiredPortrait, error) {
	return nil, nil
}

// trackPoints answers ByPeople and records who it was asked about.
type trackPoints struct {
	byPerson map[string][]trackpoint.Point
	err      error
	calls    int
	asked    [][]string
}

func (p *trackPoints) ByPeople(_ string, ids []string) ([][]trackpoint.Point, error) {
	p.calls++
	p.asked = append(p.asked, ids)
	if p.err != nil {
		return nil, p.err
	}
	var out [][]trackpoint.Point
	for _, id := range ids {
		if pts := p.byPerson[id]; len(pts) > 0 {
			out = append(out, pts)
		}
	}
	return out, nil
}

var trackBase = time.Date(2026, 9, 19, 21, 0, 0, 0, time.UTC).UnixMilli()

func tp(lat, lng float64, sec int) trackpoint.Point {
	return trackpoint.Point{TS: trackBase + int64(sec)*1000, Lat: lat, Lng: lng}
}

func walk(lng float64, n int) []trackpoint.Point {
	out := make([]trackpoint.Point, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, tp(55.700+float64(i)*0.0009, lng, i*30))
	}
	return out
}

func trackReader(t *testing.T, people *trackPeople, points *trackPoints) *patrolTrackReader {
	t.Helper()
	r := newPatrolTrackReader(people, points, "2026")
	if r == nil {
		t.Fatal("want a reader")
	}
	return r
}

func TestPatrolTrackMergesEveryMembersPoints(t *testing.T) {
	people := &trackPeople{members: map[string][]string{"team-42": {"p1", "p2"}}}
	points := &trackPoints{byPerson: map[string][]trackpoint.Point{
		"p1": walk(12.200, 10),
		"p2": walk(12.300, 10),
	}}
	r := trackReader(t, people, points)

	track, err := r.Track("team-42")
	if err != nil {
		t.Fatalf("Track: %v", err)
	}
	if len(track.Segments) != 2 {
		t.Errorf("want one segment per member, got %d", len(track.Segments))
	}
	if track.Recorders != 2 {
		t.Errorf("want 2 recorders, got %d", track.Recorders)
	}
	// Asked about exactly the patrol's members, and nobody else.
	if len(points.asked) != 1 || len(points.asked[0]) != 2 {
		t.Fatalf("want one bounded read for two members, got %v", points.asked)
	}
}

// **An empty track is a normal answer.** Task 082 measured 2% coverage, and plenty of members never grant
// location at all — so this is the common case, not a failure.
func TestPatrolTrackEmptyIsNotAnError(t *testing.T) {
	people := &trackPeople{members: map[string][]string{"team-42": {"p1", "p2"}}}
	points := &trackPoints{}
	r := trackReader(t, people, points)

	track, err := r.Track("team-42")
	if err != nil {
		t.Fatalf("an empty track must not be an error: %v", err)
	}
	if !track.IsEmpty() {
		t.Error("want an empty track")
	}
	if track.Recorders != 0 {
		t.Errorf("want 0 recorders, got %d", track.Recorders)
	}
}

// A patrol with no members — a personnel "team", or records that have gone — short-circuits without asking
// the telemetry projection anything.
func TestPatrolTrackWithNoMembersAsksForNoPoints(t *testing.T) {
	people := &trackPeople{members: map[string][]string{}}
	points := &trackPoints{}
	r := trackReader(t, people, points)

	track, err := r.Track("team-unknown")
	if err != nil {
		t.Fatalf("Track: %v", err)
	}
	if !track.IsEmpty() {
		t.Error("want an empty track")
	}
	if points.calls != 0 {
		t.Errorf("no members means no points to ask for, but ByPeople was called %d times", points.calls)
	}
}

func TestPatrolTrackEmptyTeamIDShortCircuits(t *testing.T) {
	people := &trackPeople{}
	points := &trackPoints{}
	r := trackReader(t, people, points)

	if _, err := r.Track(""); err != nil {
		t.Fatalf("Track(\"\"): %v", err)
	}
	if people.calls != 0 || points.calls != 0 {
		t.Errorf("an empty team id must not query anything, got %d/%d calls", people.calls, points.calls)
	}
}

// **The cache is the requirement, not an optimisation.** This route is unauthenticated, so "merged a
// hundred times" is somebody's afternoon rather than a load test.
func TestPatrolTrackIsCachedPerPatrol(t *testing.T) {
	people := &trackPeople{members: map[string][]string{
		"team-42": {"p1"},
		"team-43": {"p2"},
	}}
	points := &trackPoints{byPerson: map[string][]trackpoint.Point{
		"p1": walk(12.200, 10),
		"p2": walk(12.300, 10),
	}}
	r := trackReader(t, people, points)

	for i := 0; i < 5; i++ {
		if _, err := r.Track("team-42"); err != nil {
			t.Fatalf("Track: %v", err)
		}
	}
	if points.calls != 1 {
		t.Errorf("five reads of one patrol should merge once, got %d reads", points.calls)
	}

	// A different patrol is a different entry.
	if _, err := r.Track("team-43"); err != nil {
		t.Fatalf("Track: %v", err)
	}
	if points.calls != 2 {
		t.Errorf("a second patrol should be read separately, got %d reads", points.calls)
	}
}

// An empty answer is cached too, so a page for a patrol that recorded nothing does not re-query on every
// refresh — which is precisely the patrol whose page gets reloaded hopefully.
func TestPatrolTrackCachesTheEmptyAnswer(t *testing.T) {
	people := &trackPeople{members: map[string][]string{"team-42": {"p1"}}}
	points := &trackPoints{}
	r := trackReader(t, people, points)

	for i := 0; i < 3; i++ {
		if _, err := r.Track("team-42"); err != nil {
			t.Fatalf("Track: %v", err)
		}
	}
	if points.calls != 1 {
		t.Errorf("the empty answer should be cached, got %d reads", points.calls)
	}
}

// The TTL exists for a late-arriving batch from a phone that was offline for hours. An hour means such a
// batch appears within an hour rather than never.
func TestPatrolTrackCacheExpires(t *testing.T) {
	people := &trackPeople{members: map[string][]string{"team-42": {"p1"}}}
	points := &trackPoints{byPerson: map[string][]trackpoint.Point{"p1": walk(12.200, 10)}}
	r := trackReader(t, people, points)

	now := time.Now()
	r.now = func() time.Time { return now }

	if _, err := r.Track("team-42"); err != nil {
		t.Fatalf("Track: %v", err)
	}
	// Just inside the window.
	now = now.Add(patrolTrackTTL - time.Minute)
	if _, err := r.Track("team-42"); err != nil {
		t.Fatalf("Track: %v", err)
	}
	if points.calls != 1 {
		t.Errorf("still inside the TTL, got %d reads", points.calls)
	}

	// Past it.
	now = now.Add(2 * time.Minute)
	if _, err := r.Track("team-42"); err != nil {
		t.Fatalf("Track: %v", err)
	}
	if points.calls != 2 {
		t.Errorf("past the TTL the track should be re-merged, got %d reads", points.calls)
	}
}

// A failure must surface rather than being cached as "no track". An empty track is a legitimate state the
// page renders happily, so caching an outage as one would hide it for an hour.
func TestPatrolTrackErrorsAreNotCachedAsEmpty(t *testing.T) {
	people := &trackPeople{members: map[string][]string{"team-42": {"p1"}}}
	points := &trackPoints{err: errors.New("database is down")}
	r := trackReader(t, people, points)

	if _, err := r.Track("team-42"); err == nil {
		t.Fatal("want the error surfaced")
	}
	// And the next read must try again rather than serving a cached empty.
	if _, err := r.Track("team-42"); err == nil {
		t.Fatal("the failure was cached; a recovered database would not be noticed for an hour")
	}
	if points.calls != 2 {
		t.Errorf("want a retry, got %d reads", points.calls)
	}
}

func TestPatrolTrackMemberReadFailureSurfaces(t *testing.T) {
	people := &trackPeople{err: errors.New("database is down")}
	points := &trackPoints{}
	r := trackReader(t, people, points)

	if _, err := r.Track("team-42"); err == nil {
		t.Fatal("want the error surfaced")
	}
}

// Nil rather than a partially-working reader: one that could find the members but not their points would
// answer "this patrol recorded nothing", which is indistinguishable from the truth.
func TestNewPatrolTrackReaderRefusesPartialConstruction(t *testing.T) {
	if r := newPatrolTrackReader(nil, &trackPoints{}, "2026"); r != nil {
		t.Error("want nil without a person projection")
	}
	if r := newPatrolTrackReader(&trackPeople{}, nil, "2026"); r != nil {
		t.Error("want nil without a telemetry projection")
	}
}

func TestTrackPointQueriesOrNilIsAnHonestNil(t *testing.T) {
	if q := trackPointQueriesOrNil(nil); q != nil {
		t.Fatal("a nil table must convert to a nil interface, or the availability check passes and panics")
	}
}

// The race-status cutoff (PRD 011 §0b.6, task 349).
//
// A person who left the race may be sitting in a car with their phone still recording, so their later
// points are not the patrol walking. These tests pin the three cases: cut off at the moment they left,
// excluded entirely when that moment is unknown, and — the one that would be easy to get wrong —
// **`finished` is not a withdrawal**.

func statusTime(sec int) time.Time {
	return time.UnixMilli(trackBase + int64(sec)*1000).UTC()
}

func TestAWithdrawnMemberStopsCountingWhenTheyLeft(t *testing.T) {
	people := &trackPeople{
		members:  map[string][]string{"team-42": {"p1"}},
		status:   map[string]string{"p1": "reunited"},
		statusAt: map[string]time.Time{"p1": statusTime(150)},
	}
	// Twenty points at 30 s intervals: the first five fall before the withdrawal, the rest after.
	points := &trackPoints{byPerson: map[string][]trackpoint.Point{"p1": walk(12.200, 20)}}
	r := trackReader(t, people, points)

	track, err := r.Track("team-42")
	if err != nil {
		t.Fatalf("Track: %v", err)
	}
	if track.SourcePoints != 5 {
		t.Errorf("SourcePoints = %d, want the 5 recorded before they left", track.SourcePoints)
	}
	// And what remains is still drawable: the point of a cutoff is to keep the walk they did.
	if track.IsEmpty() {
		t.Error("the kilometres walked before withdrawing must survive the cutoff")
	}
}

// **Left the race, and we do not know when: excluded entirely.** That loses ground they really covered,
// which is a real cost — but the label says *mindst*, so the figure may be low and may not be high.
func TestAWithdrawnMemberWithNoTimeIsExcludedEntirely(t *testing.T) {
	people := &trackPeople{
		members: map[string][]string{"team-42": {"p1", "p2"}},
		status:  map[string]string{"p1": "released"},
	}
	points := &trackPoints{byPerson: map[string][]trackpoint.Point{
		"p1": walk(12.200, 10),
		"p2": walk(12.300, 10),
	}}
	r := trackReader(t, people, points)

	track, err := r.Track("team-42")
	if err != nil {
		t.Fatalf("Track: %v", err)
	}
	if track.Recorders != 1 {
		t.Errorf("Recorders = %d, want only the member still in the race", track.Recorders)
	}
	// p1 must not even be read: an excluded member's points are not fetched, so there is no window in
	// which they could be merged by mistake.
	for _, ids := range points.asked {
		for _, id := range ids {
			if id == "p1" {
				t.Errorf("read points for an excluded member: %v", points.asked)
			}
		}
	}
}

// **`finished` is not a withdrawal.** Finishing means walking the route to the end, and a rule that treated
// it as leaving the race would zero out the distance of exactly the patrols this page celebrates.
func TestAFinishedMemberStillCounts(t *testing.T) {
	people := &trackPeople{
		members:  map[string][]string{"team-42": {"p1"}},
		status:   map[string]string{"p1": "finished"},
		statusAt: map[string]time.Time{"p1": statusTime(150)},
	}
	points := &trackPoints{byPerson: map[string][]trackpoint.Point{"p1": walk(12.200, 20)}}
	r := trackReader(t, people, points)

	track, err := r.Track("team-42")
	if err != nil {
		t.Fatalf("Track: %v", err)
	}
	if track.SourcePoints != 20 {
		t.Errorf("SourcePoints = %d, want all 20 — a finisher walked the whole route", track.SourcePoints)
	}
}

// The reads stay bounded: one batched query for everybody who needs no cutoff, plus one per member who
// does. The batch is what keeps the common case (nobody withdrew) to a single query.
func TestTheCutoffCostsOneExtraReadPerWithdrawnMember(t *testing.T) {
	people := &trackPeople{
		members:  map[string][]string{"team-42": {"p1", "p2", "p3"}},
		status:   map[string]string{"p3": "reunited"},
		statusAt: map[string]time.Time{"p3": statusTime(150)},
	}
	points := &trackPoints{byPerson: map[string][]trackpoint.Point{
		"p1": walk(12.200, 10),
		"p2": walk(12.300, 10),
		"p3": walk(12.400, 10),
	}}
	r := trackReader(t, people, points)

	if _, err := r.Track("team-42"); err != nil {
		t.Fatalf("Track: %v", err)
	}
	if points.calls != 2 {
		t.Errorf("ByPeople calls = %d, want 2 (one batch + one trimmed member): %v", points.calls, points.asked)
	}
	if len(points.asked) != 2 || len(points.asked[0]) != 2 || len(points.asked[1]) != 1 {
		t.Errorf("want a batch of two then a single read, got %v", points.asked)
	}
}

// A patrol where *everybody* withdrew with no known time reads as "no route" rather than as an error, and
// costs no telemetry read at all.
func TestAPatrolThatWhollyWithdrewHasNoTrack(t *testing.T) {
	people := &trackPeople{
		members: map[string][]string{"team-42": {"p1"}},
		status:  map[string]string{"p1": "released"},
	}
	points := &trackPoints{byPerson: map[string][]trackpoint.Point{"p1": walk(12.200, 10)}}
	r := trackReader(t, people, points)

	track, err := r.Track("team-42")
	if err != nil {
		t.Fatalf("Track: %v", err)
	}
	if !track.IsEmpty() {
		t.Errorf("want an empty track, got %d segments", len(track.Segments))
	}
	if points.calls != 0 {
		t.Errorf("ByPeople calls = %d, want none — there was nobody to read", points.calls)
	}
}
