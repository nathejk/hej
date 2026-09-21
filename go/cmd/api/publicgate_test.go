package main

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"nathejk.dk/internal/publicgate"
	"nathejk.dk/nathejk/table/checkgroup"
	"nathejk.dk/nathejk/table/scan"
)

// The wiring half of task 330. The rule itself is tested in internal/publicgate; what is tested here is
// what the HTTP layer does with it — in particular the two ways it must refuse to leak.

type gateCheckgroups struct {
	groups []checkgroup.Checkgroup
	err    error
}

func (g gateCheckgroups) ByYear(string) ([]checkgroup.Checkgroup, error) { return g.groups, g.err }

type gateScans struct{ byTeam map[string][]scan.Scan }

func (g gateScans) ByTeam(_, teamID string) ([]scan.Scan, error) { return g.byTeam[teamID], nil }

type gateClosing struct {
	uts int64
	ok  bool
}

func (g gateClosing) LastCloses(string) (int64, bool, error) { return g.uts, g.ok, nil }

func gateTestApp(t *testing.T, gate *publicgate.Gate) *application {
	t.Helper()
	app := newTestApp(t)
	app.config.eventYear = "2025"
	app.publicGate = gate
	return app
}

// A nil gate must fail closed rather than answering "unavailable".
//
// This is the one place the public surface deliberately diverges from the rest of the service, which
// answers 503 when a projection is missing. A distinguishable "exists but unavailable" on an
// unauthenticated route confirms that a patrol number is real.
func TestPatrolGateNilFailsClosed(t *testing.T) {
	app := gateTestApp(t, nil)

	v, open := app.patrolGateFor("team-42")
	if open || v.Open() {
		t.Fatalf("a nil gate must fail closed, got reason %q open %v", v.Reason, open)
	}
}

// An error inside the gate must also fail closed, and must not be distinguishable by the caller from a
// patrol that has simply not finished.
func TestPatrolGateErrorFailsClosed(t *testing.T) {
	gate := publicgate.New(
		gateCheckgroups{err: errors.New("database is down")},
		gateScans{}, gateClosing{},
	)
	app := gateTestApp(t, gate)

	v, open := app.patrolGateFor("team-42")
	if open {
		t.Fatalf("a gate error must fail closed, got reason %q", v.Reason)
	}
	if v.Reason != publicgate.Closed {
		t.Fatalf("a gate error must be indistinguishable from a closed gate, got %q", v.Reason)
	}
}

func TestPatrolGateOpensForAFinishedPatrol(t *testing.T) {
	gate := publicgate.New(
		gateCheckgroups{groups: []checkgroup.Checkgroup{{ID: "cg-1", SortOrder: 10}, {ID: "cg-2", SortOrder: 20}}},
		gateScans{byTeam: map[string][]scan.Scan{"team-42": {{CheckgroupID: "cg-2"}}}},
		gateClosing{},
	)
	app := gateTestApp(t, gate)

	v, open := app.patrolGateFor("team-42")
	if !open || v.Reason != publicgate.Finished {
		t.Fatalf("a patrol scanned at the finish must be open as Finished, got %q open %v", v.Reason, open)
	}
}

func TestPatrolGateStaysClosedForAPatrolStillWalking(t *testing.T) {
	gate := publicgate.New(
		gateCheckgroups{groups: []checkgroup.Checkgroup{{ID: "cg-1", SortOrder: 10}, {ID: "cg-2", SortOrder: 20}}},
		gateScans{byTeam: map[string][]scan.Scan{"team-42": {{CheckgroupID: "cg-1"}}}},
		gateClosing{},
	)
	app := gateTestApp(t, gate)

	if v, open := app.patrolGateFor("team-42"); open {
		t.Fatalf("a patrol still walking must stay closed, got %q", v.Reason)
	}
}

func TestPatrolGateBackstopOpensEveryone(t *testing.T) {
	gate := publicgate.New(
		gateCheckgroups{groups: []checkgroup.Checkgroup{{ID: "cg-1", SortOrder: 10}}},
		gateScans{},
		gateClosing{uts: time.Now().Add(-time.Hour).Unix(), ok: true},
	)
	app := gateTestApp(t, gate)

	v, open := app.patrolGateFor("team-99")
	if !open || v.Reason != publicgate.RaceOver {
		t.Fatalf("after the last checkpoint closes every patrol must be open, got %q open %v", v.Reason, open)
	}
}

// Unset configuration must not produce a set containing the empty string, which would make the
// non-existent patrol of every personnel user "overridden".
func TestPublicGateOverrideEmptyConfigOpensNothing(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	for name, ids := range map[string][]string{
		"nil":   nil,
		"empty": {},
		// What splitCSV yields for an unset variable, asserted through the same path config uses.
		"unset variable": splitCSV(""),
		"only commas":    splitCSV(",, ,"),
	} {
		overridden := publicGateOverride(ids, logger)
		if overridden("") {
			t.Fatalf("%s: the empty patrol id must never be overridden", name)
		}
		if overridden("team-42") {
			t.Fatalf("%s: no patrol should be overridden by empty configuration", name)
		}
	}
}

func TestPublicGateOverrideOpensOnlyTheNamedPatrols(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	overridden := publicGateOverride(splitCSV(" team-42 , team-44 "), logger)

	for _, id := range []string{"team-42", "team-44"} {
		if !overridden(id) {
			t.Fatalf("%s should be overridden", id)
		}
	}
	for _, id := range []string{"team-43", "", "team-4", "team-420"} {
		if overridden(id) {
			t.Fatalf("%q must not be overridden", id)
		}
	}
}

// End to end through the app: an override opens a patrol that has finished nothing, and only that one.
func TestPatrolGateOverrideOpensOnePatrol(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gate := publicgate.New(
		gateCheckgroups{groups: []checkgroup.Checkgroup{{ID: "cg-1", SortOrder: 10}}},
		gateScans{}, gateClosing{},
		publicgate.WithOverride(publicGateOverride([]string{"team-42"}, logger)),
	)
	app := gateTestApp(t, gate)

	if v, open := app.patrolGateFor("team-42"); !open || v.Reason != publicgate.Override {
		t.Fatalf("the overridden patrol must be open as Override, got %q open %v", v.Reason, open)
	}
	if v, open := app.patrolGateFor("team-43"); open {
		t.Fatalf("the override must not open any other patrol, got %q", v.Reason)
	}
}
