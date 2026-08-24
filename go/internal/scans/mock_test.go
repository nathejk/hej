package scans_test

import (
	"testing"

	"nathejk.dk/internal/scans"
	"nathejk.dk/internal/users"
)

func TestMockSource_ByPatrolNewestFirst(t *testing.T) {
	got := scans.NewMockSource().ByPatrol(users.MockSpejderPatrolID)
	if len(got) == 0 {
		t.Fatal("expected seeded registrations for the spejder patrol")
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].ScannedAt.Before(got[i].ScannedAt) {
			t.Fatalf("not newest first at index %d", i)
		}
	}
}

func TestMockSource_NoPatrolYieldsNothing(t *testing.T) {
	src := scans.NewMockSource()
	if got := src.ByPatrol(""); len(got) != 0 {
		t.Errorf("empty patrol id: got %d scans, want 0", len(got))
	}
	if got := src.ByPatrol("mock-patrol-does-not-exist"); len(got) != 0 {
		t.Errorf("unknown patrol id: got %d scans, want 0", len(got))
	}
}
