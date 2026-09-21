// Package publicgate decides when a patrol's public page may exist (PRD 011 §0b.3).
//
// # The rule
//
// A patrol's page opens on the first of two triggers:
//
//  1. **the patrol has been scanned at the last checkgroup** — that is, it has finished; or
//  2. **the last checkpoint has closed** — that is, the race is over, for everybody.
//
// Trigger 2 is a backstop rather than a replacement. Trigger 1 opens a finishing patrol's page while
// the race is still running, which is the point: a patrol wants its page when it finishes, not when
// the slowest patrol does. Trigger 2 then catches everyone trigger 1 missed — the patrols that
// retired, and the patrols whose finish scan could not be attributed to a post.
//
// # Why publishing on trigger 1 is safe, which is not visible from here
//
// This package decides to publish a patrol's positions to the open web, so the reasoning had better be
// written down. It rests on four facts, none of which is recoverable from the schema:
//
//  1. **The last checkgroup is the finish line.** `checkgroup` knows route *order*; nothing in it says
//     that the last position in that order is where the race ends. Given that it is, trigger 1 and
//     "has finished" are the same predicate — which is why one gate can honestly serve as both, and
//     why a finished patrol's track describes a route it will not walk again.
//  2. **It discloses nothing useful to the patrols still out.** The finish line is not a secret: every
//     patrol knows where it is and has simply not reached it. So publishing it confers no advantage,
//     and there is no advantage to withhold.
//  3. **One route sequence serves every patrol** — nothing upstream carries a per-team route
//     (PRD 016 §11.9), so "the last checkgroup" is one group for the whole event rather than a
//     per-patrol question.
//  4. **The last checkpoint always carries absolute opening hours** (maintainer, 2026-09-19), so
//     trigger 2 always has an instant to fire on and no patrol is left without a page.
//
// Taken together, PRD 016's possession-grounded reveal model is *satisfied* rather than bypassed: the
// earliest anything becomes public is after the patrol that generated it has finished, and the
// positions themselves are ones every patrol is already walking towards.
//
// **A future event that gives patrols different routes, adds a post after the finish, or runs its
// final post on a `relative` window invalidates this gate without changing a line of this code.** That
// is the whole reason this comment is longer than the logic.
//
// # Failing closed
//
// Every uncertainty resolves to "shut". No last checkgroup, an unreadable projection, a closing instant
// that cannot be computed — all yield a closed gate. The asymmetry is deliberate and is not a matter of
// taste: a page that opens early publishes a patrol's positions while it is still racing, and a page
// that opens late disappoints somebody until it opens. Those are not comparable costs.
package publicgate

import (
	"time"

	"github.com/nathejk/shared-go/types"

	"nathejk.dk/nathejk/table/checkgroup"
	"nathejk.dk/nathejk/table/scan"
)

// Verdict is the gate's answer: whether the page may be served, and the facts that decided it.
//
// # Why the finish time comes from here
//
// Finding it is the same work as opening the gate — both are "the patrol's scan at the last checkgroup" —
// so returning it costs nothing and having the page re-derive it would mean **two definitions of
// "finished"** that can disagree. PRD 011 §0b.3 is explicit that the gate, the page and the diploma must
// share one, and this is how: there is only one place that looks.
type Verdict struct {
	// Reason is why the page is open, or Closed.
	Reason Reason

	// FinishedAt is when the patrol was scanned at the last checkgroup, or nil.
	//
	// **Nil is not the same as "not open".** A page opened by the backstop has no finish time, because the
	// patrol never reached the finish — that is precisely the state task 346 renders without a diploma.
	// So the two fields answer different questions and neither implies the other.
	FinishedAt *time.Time
}

// Open reports whether the page exists publicly.
func (v Verdict) Open() bool { return v.Reason.Open() }

// Finished reports whether this patrol crossed the line.
//
// Reads the timestamp rather than comparing the reason, because an override may have opened the page for a
// patrol that *did* finish but whose scan could not be attributed — in which case there is no finish time
// and the page must not claim one.
func (v Verdict) Finished() bool { return v.FinishedAt != nil }

// Reason is why a patrol's page is open, or that it is not.
//
// **A reason rather than a boolean**, because the two open states are not interchangeable: a page
// opened by a finish carries a diploma and a page opened by the backstop does not (PRD 011 §0b.3,
// task 346). Collapsing them here would mean recovering the distinction later from a nullable finish
// time, which is the kind of inference that goes wrong once and then looks deliberate.
type Reason string

const (
	// Closed means the page does not exist publicly yet.
	Closed Reason = ""
	// Finished means the patrol has been scanned at the last checkgroup — it crossed the line.
	Finished Reason = "finished"
	// RaceOver means the last checkpoint has closed, so everything is public.
	//
	// The backstop. A patrol open for this reason may or may not have finished; what is known is that
	// the race has ended, which is enough to publish.
	RaceOver Reason = "race_over"
	// Override means løbsledelsen opened this patrol's page by hand.
	Override Reason = "override"
)

// Open reports whether the page exists publicly.
func (r Reason) Open() bool { return r != Closed }

// Checkgroups reads the year's checkgroups in route order.
type Checkgroups interface {
	ByYear(year string) ([]checkgroup.Checkgroup, error)
}

// Scans reads a patrol's scans, each attributed to a checkgroup where the rota allows it.
type Scans interface {
	ByTeam(year, teamID string) ([]scan.Scan, error)
}

// Closing reads when the last checkpoint shuts.
type Closing interface {
	LastCloses(year string) (uts int64, ok bool, err error)
}

// Gate evaluates the rule for a patrol.
//
// Narrow local interfaces for the three inputs, following internal/reveal: the rule is the interesting
// part and it is worth testing without a database.
type Gate struct {
	checkgroups Checkgroups
	scans       Scans
	closing     Closing

	// now is injectable so the backstop is testable without waiting for an event to end.
	now func() time.Time

	// overridden reports whether løbsledelsen has opened this patrol by hand. Never nil after New.
	overridden func(patrolID string) bool
}

// Option configures a Gate.
type Option func(*Gate)

// WithClock replaces the clock. For tests.
func WithClock(now func() time.Time) Option {
	return func(g *Gate) {
		if now != nil {
			g.now = now
		}
	}
}

// WithOverride supplies the manual override: patrols løbsledelsen has chosen to open early.
//
// **A convenience, not a safety net.** Because trigger 2 is guaranteed to fire (see the package doc's
// fact 4), no patrol can end up with no page at all; the worst case without an override is a patrol
// whose finish scan went unattributed waiting until closing time instead of getting its page at the
// line. So this exists to fix that by hand, and nothing depends on it existing.
//
// It can only ever *open* a page, never close one. An override that could withhold a page would be a
// second, quieter access-control mechanism, and the one thing worse than a page opening late is two
// places to look when it does not open at all.
func WithOverride(overridden func(patrolID string) bool) Option {
	return func(g *Gate) {
		if overridden != nil {
			g.overridden = overridden
		}
	}
}

// New builds the gate.
func New(checkgroups Checkgroups, scans Scans, closing Closing, opts ...Option) *Gate {
	g := &Gate{
		checkgroups: checkgroups,
		scans:       scans,
		closing:     closing,
		now:         time.Now,
		overridden:  func(string) bool { return false },
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// For reports why this patrol's public page is open, or Closed.
//
// An empty patrol id is Closed rather than an error: personnel roles have no patrol, and asking for the
// scans of team "" can only ever return nothing.
//
// Finished is reported in preference to RaceOver when both hold: "this patrol finished" is the stronger
// statement, and it is the one that decides whether a diploma appears.
func (g *Gate) For(year, patrolID string) (Verdict, error) {
	if patrolID == "" {
		return Verdict{}, nil
	}

	// The finish scan is looked for first even when an override is in play, so the page can show a finish
	// time for a patrol that finished *and* was opened by hand. An override is about visibility, not about
	// whether the patrol reached the line.
	finishedAt, err := g.finishedAt(year, patrolID)
	if err != nil {
		return Verdict{}, err
	}

	if g.overridden(patrolID) {
		return Verdict{Reason: Override, FinishedAt: finishedAt}, nil
	}
	if finishedAt != nil {
		return Verdict{Reason: Finished, FinishedAt: finishedAt}, nil
	}

	over, err := g.raceOver(year)
	if err != nil {
		return Verdict{}, err
	}
	if over {
		return Verdict{Reason: RaceOver}, nil
	}
	return Verdict{}, nil
}

// finishedAt returns when the patrol was scanned at the last checkgroup, or nil.
//
// Note both halves fail closed by returning nil rather than an error where the data is merely absent: no
// checkgroups yet is the normal state of the projection for most of the year, and it is not a fault.
//
// The **earliest** matching scan is taken, not the latest. A patrol scanned twice at the finish — a
// re-scan, a second post in the same group — finished when it first arrived; reporting the later time
// would quietly add the minutes it stood there to its night.
func (g *Gate) finishedAt(year, patrolID string) (*time.Time, error) {
	last, ok, err := g.lastCheckgroup(year)
	if err != nil || !ok {
		return nil, err
	}

	patrolScans, err := g.scans.ByTeam(year, patrolID)
	if err != nil {
		return nil, err
	}

	var earliest *time.Time
	for _, s := range patrolScans {
		// An unattributed scan has no CheckgroupID and cannot satisfy this. That is a normal outcome
		// rather than an error (see scan.Scan's doc): the checkpoint is recovered from the scanner's
		// shift, and the rota is fed from outside this repo. Such a patrol waits for the backstop.
		if s.CheckgroupID == "" || types.CheckgroupID(s.CheckgroupID) != last {
			continue
		}
		at := time.Unix(s.Uts, 0).UTC()
		if earliest == nil || at.Before(*earliest) {
			earliest = &at
		}
	}
	return earliest, nil
}

// lastCheckgroup returns the final group in route order — the finish line.
//
// `ByYear` already orders by sortOrder with the id as a stable tiebreak, so this is the last element. It
// is taken rather than re-sorted here precisely so there is one definition of route order in the
// codebase: a second sort would be a second chance to disagree with the map's arrows.
//
// ok is false when the year has no checkgroups. Normal before the route is entered, and closed.
func (g *Gate) lastCheckgroup(year string) (types.CheckgroupID, bool, error) {
	groups, err := g.checkgroups.ByYear(year)
	if err != nil {
		return "", false, err
	}
	if len(groups) == 0 {
		return "", false, nil
	}
	return groups[len(groups)-1].ID, true, nil
}

// raceOver reports whether the last checkpoint has closed.
//
// **The zero guard is the one that matters**, and it is worth being explicit about what it prevents:
// `openUntilUts` is `0` for a `relative` or `none` window and zero means **not set**, not midnight
// 1970. Treating an absent instant as a real one would put it decades in the past and open every
// patrol's page in the event at once — the worst available failure of this package.
//
// It is checked twice, here and in the querier, and the duplication is deliberate: `Closing` is an
// interface, so the instant arrives from whatever a caller wired in. A guard that lives only in one
// implementation of a dependency is not a guard on this rule. The cost is one comparison; the cost of
// the alternative is the entire event's positions published at once by a fake in a test or a second
// implementation that reports `ok` more loosely.
//
// The maintainer has confirmed the last checkpoint always carries absolute hours (2026-09-19), so this
// asserts an invariant rather than handling an expected path.
func (g *Gate) raceOver(year string) (bool, error) {
	closes, ok, err := g.closing.LastCloses(year)
	if err != nil || !ok || closes <= 0 {
		return false, err
	}
	return !g.now().Before(time.Unix(closes, 0)), nil
}
