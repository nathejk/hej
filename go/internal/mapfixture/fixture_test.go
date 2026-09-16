package mapfixture

import (
	"testing"

	"github.com/nathejk/shared-go/types"
)

// This test is what makes the fixture world trustworthy rather than merely present. A fixture nobody checks
// drifts: a checkpoint moved into the wrong checkgroup, or a handout marked current, and the state it was
// built to demonstrate silently stops existing — while the dev map still looks plausible.

func revealedIDs(t *testing.T, started bool) map[string]bool {
	t.Helper()
	got, err := NewRule().Revealed("2026", PatrolID, started)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	ids := map[string]bool{}
	for _, c := range got.Checkpoints {
		ids[string(c.ID)] = true
	}
	return ids
}

// The guarantee PRD 016 exists to be safe about, in the fixture world: a positioned, real checkpoint that
// no rule reveals must not come back. It is absent because the reveal rule filters it — the sheet listing
// it was never handed to this patrol — not because the fake declined to mention it.
func TestSecretCheckpointIsNeverRevealed(t *testing.T) {
	ids := revealedIDs(t, true)

	if ids["cp-secret"] {
		t.Error("cp-secret is revealed by nothing and must never reach the client")
	}
	// The positive half, without which the test above would pass on an empty result.
	if len(ids) == 0 {
		t.Fatal("the fixture must reveal something, or this test is vacuous")
	}
}

// Each reveal trigger the fixture claims to demonstrate, asserted to actually fire.
func TestEveryRevealTriggerFires(t *testing.T) {
	ids := revealedIDs(t, true)

	for _, c := range []struct {
		id, why string
	}{
		{"cp-1", "drawn on kort-1, a QR-bound sheet the patrol holds"},
		{"cp-2", "same, plus its checkgroup has been scanned"},
		{"cp-3", "on kort-2, a skitse whose handout checkgroup (cg-2) has been reached"},
		{"cp-4", "on kort-3 — reassigned away, but revealing is monotonic (PRD 016 §11.4)"},
	} {
		if !ids[c.id] {
			t.Errorf("%s must be revealed: %s", c.id, c.why)
		}
	}

	// Revealed in principle, absent in practice: no position means nothing to draw and nothing to arrow at.
	if ids["cp-5"] {
		t.Error("cp-5 has no position and must not be returned")
	}
}

// The next line drives the edge arrows. With cg-1 and cg-2 scanned, the patrol is heading for cg-3.
func TestNextLineIsTheFirstUnreachedGroup(t *testing.T) {
	got, err := NewRule().Revealed("2026", PatrolID, true)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if got.NextCheckgroup != "cg-3" {
		t.Errorf("next checkgroup = %q, want cg-3 (cg-1 and cg-2 are scanned)", got.NextCheckgroup)
	}
}

// Another patrol shares the fixture year and must see none of it — the fakes are team-scoped, so this also
// guards against a fixture that ignores the patrol id and hands every caller the same map.
func TestAnotherPatrolSeesNothing(t *testing.T) {
	got, err := NewRule().Revealed("2026", "some-other-patrol", true)
	if err != nil {
		t.Fatalf("Revealed: %v", err)
	}
	if len(got.Checkpoints) != 0 {
		t.Errorf("a patrol with no handouts and no scans must see nothing, got %d", len(got.Checkpoints))
	}
}

// The three handout states the drawer has to render, plus the synthesised one that no binding event can
// ever produce.
func TestHandoutsCoverEveryState(t *testing.T) {
	got, err := NewRule().Handouts("2026", PatrolID)
	if err != nil {
		t.Fatalf("Handouts: %v", err)
	}

	var bound, reassigned, unknownSheet, synthesised int
	for _, h := range got {
		switch {
		case h.Synthesised:
			synthesised++
		case h.Sheet == "":
			unknownSheet++
		case !h.StillHeld:
			reassigned++
		default:
			bound++
		}
	}

	if bound == 0 {
		t.Error("want a QR-bound sheet the patrol still holds")
	}
	if reassigned == 0 {
		t.Error("want a reassigned sheet, to exercise the \"afleveret\" state")
	}
	if unknownSheet == 0 {
		t.Error("want a handout with an unknown sheet, to exercise \"Ukendt kort\"")
	}
	if synthesised == 0 {
		t.Error("want a synthesised skitse handout — the case a binding-only list loses")
	}
}

// A synthesised handout has no sticker number, which is the branch the drawer lays out without a gap.
func TestSynthesisedHandoutHasNoQr(t *testing.T) {
	got, err := NewRule().Handouts("2026", PatrolID)
	if err != nil {
		t.Fatalf("Handouts: %v", err)
	}
	for _, h := range got {
		if h.Synthesised && h.QrID != "" {
			t.Errorf("a skitse has no code, got QrID %q", h.QrID)
		}
	}
}

// Every scheme must appear across the fixture's checkgroups, or the verdict paths cannot all be looked at
// on a device: fixed, relative (with its anchor) and none.
func TestEverySchemeIsRepresented(t *testing.T) {
	seen := map[string]bool{}
	var relativeAnchored bool
	for _, g := range Checkgroups() {
		seen[string(g.Scheme)] = true
		if g.Scheme == "relative" && g.RelativeCheckgroupID != "" {
			relativeAnchored = true
		}
	}
	for _, want := range []string{"fixed", "relative", "none"} {
		if !seen[want] {
			t.Errorf("no checkgroup uses the %q scheme", want)
		}
	}
	if !relativeAnchored {
		t.Error("the relative group must name an anchoring checkgroup")
	}
}

// The scan fixtures must include one nobody can attribute: no checkpoint, no checkgroup, no scheme — and
// therefore no verdict. It is the state most likely to be missing from a hand-built fixture, because it
// looks like a mistake.
func TestScansIncludeAnUnattributableOne(t *testing.T) {
	var unattributed int
	for _, s := range Scans() {
		if s.CheckpointID == "" && s.CheckgroupID == "" {
			unattributed++
		}
	}
	if unattributed == 0 {
		t.Error("want a scan no shift could place, listed with no checkpoint and no verdict")
	}
}

// The fixture must stay inside the map's default view, or the dev map opens on empty sea and the markers
// are unreachable without panning. Central Jutland, matching scans.NewMockSource.
func TestPositionedCheckpointsAreInCentralJutland(t *testing.T) {
	for _, c := range Checkpoints() {
		if c.Lat == 0 && c.Lng == 0 {
			continue // deliberately unsited
		}
		if c.Lat < 56.0 || c.Lat > 56.3 || c.Lng < 9.1 || c.Lng > 9.6 {
			t.Errorf("%s at %v,%v is outside the map's default view", c.ID, c.Lat, c.Lng)
		}
	}
}

// The reveal rule reads coordinates from the checkpoint projection, so the fixture's own filter must behave
// like the real one: bounded reads, and no unsited posts. Asserted directly because the security boundary
// lives in this read (PRD 016 §6) — a fake that ignored its id filter would make every other test vacuous.
func TestCheckpointReadsAreBounded(t *testing.T) {
	f := fixtureCheckpoints{all: Checkpoints()}

	got, err := f.ByIDs("2026", nil)
	if err != nil {
		t.Fatalf("ByIDs: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("asking for no ids must return nothing, got %d", len(got))
	}

	got, err = f.ByIDs("2026", []types.CheckpointID{"cp-1"})
	if err != nil {
		t.Fatalf("ByIDs: %v", err)
	}
	if len(got) != 1 || got[0].ID != "cp-1" {
		t.Errorf("want only cp-1, got %+v", got)
	}
}
