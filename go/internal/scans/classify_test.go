package scans

import (
	"errors"
	"testing"
)

// Classifying a bandit catch (task 271).
//
// The asymmetry these tests exist to protect: missing a catch understates what happened, but *inventing*
// one tells a patrol they were caught when they were not — and they would act on it. Nothing upstream
// distinguishes the two acts, so every uncertain case must resolve to KindCheckpoint.

type stubBanditter struct {
	ids  map[string]bool
	err  error
	year string
	// calls counts the reads, so the "one query, not one per scan" property is testable.
	calls int
}

func (s *stubBanditter) BanditIDs(year string) (map[string]bool, error) {
	s.calls++
	s.year = year
	if s.err != nil {
		return nil, s.err
	}
	return s.ids, nil
}

// A scan by someone on a post shift is a checkpoint visit.
func TestPostPersonnelScanIsACheckpointScan(t *testing.T) {
	p := &stubProjection{rows: []ProjectedScan{
		{QrID: "qr-1", Uts: 1750000500, ScannerID: "user-post", CheckpointID: "cp-1", CheckpointName: "Post 1"},
	}}
	b := &stubBanditter{ids: map[string]bool{"user-bandit": true}}

	got := NewProjectionSource(p, "2026", b, nil).ByPatrol("team-9")

	if len(got) != 1 {
		t.Fatalf("want 1 registration, got %d", len(got))
	}
	if got[0].Kind != KindCheckpoint {
		t.Errorf("kind = %q, want checkpoint", got[0].Kind)
	}
	if got[0].Label != "Post 1" {
		t.Errorf("label = %q, want the post name", got[0].Label)
	}
}

// A scan by a known bandit is a bandit catch.
func TestBanditScanIsABanditCatch(t *testing.T) {
	p := &stubProjection{rows: []ProjectedScan{
		{QrID: "qr-2", Uts: 1750001000, ScannerID: "user-bandit"},
	}}
	b := &stubBanditter{ids: map[string]bool{"user-bandit": true}}

	got := NewProjectionSource(p, "2026", b, nil).ByPatrol("team-9")

	if len(got) != 1 {
		t.Fatalf("want 1 registration, got %d", len(got))
	}
	if got[0].Kind != KindBandit {
		t.Errorf("kind = %q, want bandit", got[0].Kind)
	}
	// More use than "Registrering" beside the drawer's skull, and it names no individual: we know the
	// scanner was a bandit, not which one, and who caught them is not this app's business.
	if got[0].Label != "Bandit" {
		t.Errorf("label = %q, want \"Bandit\"", got[0].Label)
	}
}

// The case that must fail safe: a scanner nobody can classify stays a checkpoint scan.
func TestUnknownScannerStaysACheckpointScan(t *testing.T) {
	p := &stubProjection{rows: []ProjectedScan{
		{QrID: "qr-3", Uts: 1750002000, ScannerID: "user-who"},
		// And a scan with no scanner at all.
		{QrID: "qr-4", Uts: 1750002100},
	}}
	b := &stubBanditter{ids: map[string]bool{"user-bandit": true}}

	got := NewProjectionSource(p, "2026", b, nil).ByPatrol("team-9")

	if len(got) != 2 {
		t.Fatalf("want 2 registrations, got %d", len(got))
	}
	for _, s := range got {
		if s.Kind != KindBandit {
			continue
		}
		t.Errorf("an unclassifiable scanner must not produce a bandit catch: %+v", s)
	}
}

// No classifier at all (no person projection — a database-less run) leaves everything a checkpoint scan
// rather than panicking on a nil interface or guessing.
func TestNoClassifierLeavesEverythingACheckpointScan(t *testing.T) {
	p := &stubProjection{rows: []ProjectedScan{
		{QrID: "qr-5", Uts: 1750003000, ScannerID: "user-bandit"},
	}}

	got := NewProjectionSource(p, "2026", nil, nil).ByPatrol("team-9")

	if len(got) != 1 {
		t.Fatalf("want 1 registration, got %d", len(got))
	}
	if got[0].Kind != KindCheckpoint {
		t.Errorf("kind = %q, want checkpoint without a classifier", got[0].Kind)
	}
}

// A failed classification must not fail the list, and must not invent a catch. The registrations are the
// point; the kind is a label on them.
func TestClassifierFailureIsReportedAndDoesNotInventACatch(t *testing.T) {
	p := &stubProjection{rows: []ProjectedScan{
		{QrID: "qr-6", Uts: 1750004000, ScannerID: "user-bandit"},
	}}
	b := &stubBanditter{err: errors.New("database is down")}

	var reported error
	got := NewProjectionSource(p, "2026", b, func(err error) { reported = err }).ByPatrol("team-9")

	if len(got) != 1 {
		t.Fatalf("a classification failure must not lose the registrations, got %d", len(got))
	}
	if got[0].Kind != KindCheckpoint {
		t.Errorf("kind = %q, want checkpoint when classification failed", got[0].Kind)
	}
	if reported == nil {
		t.Error("a classification failure must be reported, not swallowed")
	}
}

// One read per request, not one per registration — the reason Banditter returns a set.
func TestClassificationReadsTheSetOnce(t *testing.T) {
	p := &stubProjection{rows: []ProjectedScan{
		{QrID: "qr-a", Uts: 1750005000, ScannerID: "user-bandit"},
		{QrID: "qr-b", Uts: 1750005100, ScannerID: "user-post"},
		{QrID: "qr-c", Uts: 1750005200, ScannerID: "user-bandit"},
	}}
	b := &stubBanditter{ids: map[string]bool{"user-bandit": true}}

	NewProjectionSource(p, "2026", b, nil).ByPatrol("team-9")

	if b.calls != 1 {
		t.Errorf("want a single classification read, got %d", b.calls)
	}
	if b.year != "2026" {
		t.Errorf("classification must be year-scoped, asked %q", b.year)
	}
}

// An empty patrol short-circuits before any read, including the classification one.
func TestNoPatrolDoesNotClassify(t *testing.T) {
	b := &stubBanditter{}

	NewProjectionSource(&stubProjection{}, "2026", b, nil).ByPatrol("")

	if b.calls != 0 {
		t.Errorf("want no classification read for an empty patrol, got %d", b.calls)
	}
}
