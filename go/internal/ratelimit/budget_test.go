package ratelimit

import (
	"testing"
	"time"
)

func TestBudget_AllowsUpToLimit(t *testing.T) {
	now := time.Now()
	b := NewBudget(1000, time.Hour)
	b.now = func() time.Time { return now }

	if !b.Allow("member", 600) {
		t.Fatal("first charge should fit")
	}
	if !b.Allow("member", 400) {
		t.Fatal("second charge should fit exactly")
	}
	if b.Allow("member", 1) {
		t.Fatal("one byte over the budget was allowed")
	}
}

// A rejected upload has consumed no budget. Otherwise a member who tried a large video and was
// refused would find the refusal had eaten the room for the small photo they settled for.
func TestBudget_ARefusalCostsNothing(t *testing.T) {
	now := time.Now()
	b := NewBudget(1000, time.Hour)
	b.now = func() time.Time { return now }

	if b.Allow("member", 5000) {
		t.Fatal("a charge larger than the whole budget was allowed")
	}
	if !b.Allow("member", 1000) {
		t.Error("the refused charge consumed budget it should not have")
	}
}

// Clamping an oversized charge would let one through per window and read as the limit not working.
func TestBudget_RefusesAnOversizedChargeRatherThanClamping(t *testing.T) {
	b := NewBudget(100, time.Hour)
	if b.Allow("member", 101) {
		t.Error("an oversized charge was clamped instead of refused")
	}
	if b.Remaining("member") != 100 {
		t.Errorf("remaining = %d, want the full budget", b.Remaining("member"))
	}
}

func TestBudget_WindowSlides(t *testing.T) {
	now := time.Now()
	b := NewBudget(1000, time.Hour)
	b.now = func() time.Time { return now }

	if !b.Allow("member", 1000) {
		t.Fatal("first charge should fit")
	}
	if b.Allow("member", 1) {
		t.Fatal("budget should be spent")
	}

	// Just inside the window: still spent. This is the assertion that distinguishes a sliding
	// window from a fixed one — a fixed-window counter would already have reset, letting a client
	// spend two full budgets back to back by straddling the boundary.
	now = now.Add(59 * time.Minute)
	if b.Allow("member", 1) {
		t.Error("the window reset early")
	}

	now = now.Add(2 * time.Minute)
	if !b.Allow("member", 1000) {
		t.Error("the window did not slide")
	}
}

func TestBudget_IsPerKey(t *testing.T) {
	b := NewBudget(1000, time.Hour)
	if !b.Allow("a", 1000) {
		t.Fatal("a should fit")
	}
	// Participants share networks, so these are keyed by member: one patrol member exhausting
	// their budget must not throttle the rest of the patrol.
	if !b.Allow("b", 1000) {
		t.Error("one member's spending blocked another's")
	}
}

// The disabling value is the zero value, matching the retention windows: an unset environment
// variable must not accidentally impose a limit nobody chose.
func TestBudget_ZeroMeansUnlimited(t *testing.T) {
	for _, limit := range []int64{0, -1} {
		b := NewBudget(limit, time.Hour)
		for i := 0; i < 100; i++ {
			if !b.Allow("member", 1<<30) {
				t.Fatalf("limit %d should be unlimited", limit)
			}
		}
		if b.Remaining("member") != -1 {
			t.Errorf("limit %d: remaining = %d, want -1 for unlimited", limit, b.Remaining("member"))
		}
	}
}

// A nil Budget allows everything, so a handler can hold one unconditionally without a nil check —
// the same shape the existing limiters' callers rely on.
func TestBudget_NilAllowsEverything(t *testing.T) {
	var b *Budget
	if !b.Allow("member", 1<<40) {
		t.Error("a nil budget refused a charge")
	}
	if b.Remaining("member") != -1 {
		t.Error("a nil budget reported a finite remainder")
	}
}

func TestBudget_RemainingCountsDown(t *testing.T) {
	now := time.Now()
	b := NewBudget(1000, time.Hour)
	b.now = func() time.Time { return now }

	if got := b.Remaining("member"); got != 1000 {
		t.Errorf("remaining = %d, want 1000", got)
	}
	b.Allow("member", 250)
	if got := b.Remaining("member"); got != 750 {
		t.Errorf("remaining = %d, want 750", got)
	}
	b.Allow("member", 750)
	if got := b.Remaining("member"); got != 0 {
		t.Errorf("remaining = %d, want 0", got)
	}
}
