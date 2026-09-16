package main

import (
	"testing"
	"time"

	"nathejk.dk/internal/reveal"
	"nathejk.dk/internal/scans"
	"nathejk.dk/internal/users"
	"nathejk.dk/nathejk/table/checkpoint"
)

// The two things every one of these versions must get right (task 269):
//   - it CHANGES when the underlying data changes — a stable-but-wrong version silently strands a device
//     on stale data;
//   - it is STABLE across repeated calls with unchanged data — an unstable version makes every device
//     refetch everything on every poll, which is the cost the version exists to avoid.
// Tested on the pure hash functions per dataset, then the caching (keyed by patrol, not user) on top.

func TestCheckpointsVersion_ChangesAndIsStable(t *testing.T) {
	base := []checkpoint.Checkpoint{
		{ID: "cp-1", Name: "Post 1", Checkgroup: "cg-1", SortOrder: 0, Lat: 56.1, Lng: 9.5, OpenFromUts: 1000, OpenUntilUts: 2000},
	}

	v := checkpointsVersion(base, "cg-1")
	if v == "" {
		t.Fatal("version must not be empty")
	}
	if again := checkpointsVersion(base, "cg-1"); again != v {
		t.Errorf("unchanged data must hash the same: %q vs %q", v, again)
	}

	// Each field the payload exposes must move the version, or a change to it would strand a device.
	cases := map[string]func(*checkpoint.Checkpoint){
		"name":     func(c *checkpoint.Checkpoint) { c.Name = "Post 1b" },
		"lat":      func(c *checkpoint.Checkpoint) { c.Lat = 56.2 },
		"sorter":   func(c *checkpoint.Checkpoint) { c.SortOrder = 1 },
		"openFrom": func(c *checkpoint.Checkpoint) { c.OpenFromUts = 1500 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			changed := []checkpoint.Checkpoint{base[0]}
			mutate(&changed[0])
			if checkpointsVersion(changed, "cg-1") == v {
				t.Errorf("a change to %s must change the version", name)
			}
		})
	}

	// The next line is part of it: an arrow moving to a new line is a change the client must notice.
	if checkpointsVersion(base, "cg-2") == v {
		t.Error("a different next_checkgroup must change the version")
	}
}

func TestHandoutsVersion_ChangesAndIsStable(t *testing.T) {
	base := []reveal.Handout{
		{Sheet: "kort-1", Name: "Etape 1", Format: "a4", QrID: "1042", HandedOutUts: 1000, StillHeld: true},
	}

	v := handoutsVersion(base)
	if again := handoutsVersion(base); again != v {
		t.Errorf("unchanged data must hash the same: %q vs %q", v, again)
	}

	// A sheet moving from held to afleveret changes nothing but StillHeld, and must still move the version.
	changed := []reveal.Handout{base[0]}
	changed[0].StillHeld = false
	if handoutsVersion(changed) == v {
		t.Error("a handout becoming afleveret must change the version")
	}

	// A new sheet handed out must change it too.
	added := append([]reveal.Handout{}, base...)
	added = append(added, reveal.Handout{Sheet: "kort-2", Name: "Etape 2", Format: "a4", QrID: "1043", HandedOutUts: 2000, StillHeld: true})
	if handoutsVersion(added) == v {
		t.Error("an additional handout must change the version")
	}
}

func TestScansVersion_ChangesAndIsStable(t *testing.T) {
	base := []scans.Scan{
		{ID: "s-1", Kind: scans.KindCheckpoint, Label: "Post 1", CheckpointID: "cp-1", ScannedAt: time.Unix(1000, 0)},
	}

	v := scansVersion(base)
	if again := scansVersion(base); again != v {
		t.Errorf("unchanged data must hash the same: %q vs %q", v, again)
	}

	// A scan gaining a verdict — a relative window whose anchor has now been scanned — has changed for the
	// drawer even though the scan row itself is untouched.
	withVerdict := []scans.Scan{base[0]}
	withVerdict[0].Verdict = &scans.Verdict{OnTime: true, DeltaSeconds: 0}
	gained := scansVersion(withVerdict)
	if gained == v {
		t.Error("a scan gaining a verdict must change the version")
	}

	// And the verdict's content matters: on time vs late must differ.
	late := []scans.Scan{base[0]}
	late[0].Verdict = &scans.Verdict{OnTime: false, DeltaSeconds: 600}
	if scansVersion(late) == gained {
		t.Error("a different verdict must change the version")
	}

	// A new scan must change it.
	added := append([]scans.Scan{}, base...)
	added = append(added, scans.Scan{ID: "s-2", Kind: scans.KindBandit, Label: "Bandit", ScannedAt: time.Unix(2000, 0)})
	if scansVersion(added) == v {
		t.Error("an additional scan must change the version")
	}
}

// The cache is keyed by patrol, not by user: two members of one patrol must share the computed answer, so
// a race's worth of devices does not turn the poll into continuous query load. Demonstrated on checkpoints
// (the most involved, since it also keys on started-state); handouts and scans use the same versionCache.
func TestCheckpointsVersionFor_CachedByPatrolNotUser(t *testing.T) {
	app := checkpointsApp(t, fakeMapReads{
		revealed: []checkpoint.Checkpoint{{ID: "cp-1", Name: "Post 1", Lat: 56.1, Lng: 9.5}},
	}, "2026")
	app.checkpointsVersions = newVersionCache(time.Hour)

	memberA := users.User{ID: "u1", PatrolID: "team-1"}
	memberB := users.User{ID: "u2", PatrolID: "team-1"}

	vA, err := app.checkpointsVersionFor(memberA)
	if err != nil {
		t.Fatalf("versionFor: %v", err)
	}

	// Change the underlying data. A second member of the same patrol must still get the cached answer —
	// proving the work is shared per patrol rather than recomputed per user.
	app.models.Maps = fakeMapReads{revealed: []checkpoint.Checkpoint{{ID: "cp-9", Name: "Different", Lat: 55, Lng: 8}}}
	vB, err := app.checkpointsVersionFor(memberB)
	if err != nil {
		t.Fatalf("versionFor: %v", err)
	}
	if vA != vB {
		t.Errorf("two members of one patrol must share a cached version: %q vs %q", vA, vB)
	}

	// A different patrol computes fresh, against the now-changed data.
	other := users.User{ID: "u3", PatrolID: "team-2"}
	vOther, err := app.checkpointsVersionFor(other)
	if err != nil {
		t.Fatalf("versionFor: %v", err)
	}
	if vOther == vA {
		t.Error("a different patrol must not receive another patrol's cached version")
	}
}

// With caching off, a data change really does propagate through the *For method — the belt to the pure
// test's braces, and the guarantee PRD 017's foreground check actually depends on.
func TestCheckpointsVersionFor_ChangesWhenDataChanges(t *testing.T) {
	app := checkpointsApp(t, fakeMapReads{
		revealed: []checkpoint.Checkpoint{{ID: "cp-1", Name: "Post 1", Lat: 56.1, Lng: 9.5}},
	}, "2026")
	app.checkpointsVersions = newVersionCache(0) // no caching: recompute every call

	viewer := users.User{ID: "u1", PatrolID: "team-1"}
	before, _ := app.checkpointsVersionFor(viewer)

	app.models.Maps = fakeMapReads{revealed: []checkpoint.Checkpoint{{ID: "cp-1", Name: "Post 1 renamed", Lat: 56.1, Lng: 9.5}}}
	after, _ := app.checkpointsVersionFor(viewer)

	if before == after {
		t.Error("a change in the revealed set must change the version")
	}
}

func TestHandoutsVersionFor_ChangesWhenDataChanges(t *testing.T) {
	app := checkpointsApp(t, fakeMapReads{
		handouts: []reveal.Handout{{Sheet: "kort-1", Name: "Etape 1", QrID: "1", HandedOutUts: 1000, StillHeld: true}},
	}, "2026")
	app.handoutsVersions = newVersionCache(0)

	viewer := users.User{ID: "u1", PatrolID: "team-1"}
	before, _ := app.handoutsVersionFor(viewer)

	app.models.Maps = fakeMapReads{handouts: []reveal.Handout{{Sheet: "kort-1", Name: "Etape 1", QrID: "1", HandedOutUts: 1000, StillHeld: false}}}
	after, _ := app.handoutsVersionFor(viewer)

	if before == after {
		t.Error("a handout becoming afleveret must change the version through the endpoint path")
	}
}

// stubScanSource is a settable scans.Source, so the scans version's caching (keyed by patrol) can be
// exercised through the *For method with data that changes between calls.
type stubScanSource struct{ byPatrol map[string][]scans.Scan }

func (s stubScanSource) ByPatrol(patrolID string) []scans.Scan { return s.byPatrol[patrolID] }

func TestScansVersionFor_CachedByPatrolAndChangesWithData(t *testing.T) {
	app := checkpointsApp(t, fakeMapReads{}, "2026")
	src := stubScanSource{byPatrol: map[string][]scans.Scan{
		"team-1": {{ID: "s-1", Kind: scans.KindCheckpoint, Label: "Post 1", ScannedAt: time.Unix(1000, 0)}},
	}}
	app.models.Scans = src
	app.scansVersions = newVersionCache(time.Hour)

	memberA := users.User{ID: "u1", PatrolID: "team-1"}
	memberB := users.User{ID: "u2", PatrolID: "team-1"}

	vA, err := app.scansVersionFor(memberA)
	if err != nil {
		t.Fatalf("scansVersionFor: %v", err)
	}

	// Change the underlying scans; a second member of the same patrol still gets the cached answer.
	src.byPatrol["team-1"] = append(src.byPatrol["team-1"], scans.Scan{ID: "s-2", Kind: scans.KindBandit, Label: "Bandit", ScannedAt: time.Unix(2000, 0)})
	if vB, _ := app.scansVersionFor(memberB); vB != vA {
		t.Errorf("one patrol's members must share a cached version: %q vs %q", vA, vB)
	}

	// With caching off, the new scan propagates.
	app.scansVersions = newVersionCache(0)
	if vChanged, _ := app.scansVersionFor(memberA); vChanged == vA {
		t.Error("an additional scan must change the version through the *For path")
	}
}
