package ratelimit

import (
	"sync"
	"time"
)

// Budget is a per-key sliding-window limit on an *amount* rather than on a count.
//
// # Why a second type instead of a bigger Limiter
//
// `Limiter` answers "how many times", which is the right question for a PIN request or a report. It
// is the wrong question for an upload: sixty thumbnails and sixty 12 MiB videos are the same number
// of events and nowhere near the same cost. PRD 019 §8 asks for both a per-hour count *and* a
// per-hour byte cap, because each one lets through exactly what the other is for.
//
// Kept separate rather than folded in, so `Limiter.Allow(key)` keeps its one-argument signature and
// its dozen existing callers stay honest about counting events.
//
// # The window is a sum, not a bucket
//
// It holds the individual charges and re-sums the ones still inside the window, matching `Limiter`'s
// approach for the same reason: a fixed-window counter resets on a boundary, which lets a client
// spend two full budgets back-to-back by straddling it. Memory is bounded by what a member can
// actually upload in a window, which the count limiter already caps.
type Budget struct {
	mu     sync.Mutex
	spends map[string][]spend
	limit  int64
	window time.Duration
	now    func() time.Time
}

type spend struct {
	at     time.Time
	amount int64
}

// NewBudget returns a Budget allowing `limit` units per `window` for each key.
//
// A limit of 0 or less means unlimited, which is what a zero-config run and the test harness get. The
// same convention as the retention windows: the disabling value is the zero value, so an unset
// environment variable cannot accidentally impose a limit nobody chose.
func NewBudget(limit int64, window time.Duration) *Budget {
	return &Budget{
		spends: make(map[string][]spend),
		limit:  limit,
		window: window,
		now:    time.Now,
	}
}

// Allow charges `amount` to key and reports whether it fits.
//
// **Nothing is charged when it does not fit.** A rejected upload has consumed no budget, so a member
// who tries a 12 MiB video and is refused can still post a 200 kB photo — rather than having the
// refusal itself eat the room for it. That asymmetry is the whole point of checking before the work
// rather than after.
//
// A single charge larger than the entire budget is refused rather than clamped. Clamping would let
// one oversized item through per window and read as the limit not working.
func (b *Budget) Allow(key string, amount int64) bool {
	if b == nil || b.limit <= 0 {
		return true
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.now()
	cutoff := now.Add(-b.window)

	kept := make([]spend, 0, len(b.spends[key]))
	var used int64
	for _, s := range b.spends[key] {
		if s.at.After(cutoff) {
			kept = append(kept, s)
			used += s.amount
		}
	}

	if used+amount > b.limit {
		b.spends[key] = kept
		return false
	}

	b.spends[key] = append(kept, spend{at: now, amount: amount})
	return true
}

// Remaining reports how much of the budget is left for key.
//
// Exists so a refusal can name a number a member can act on. "Du har uploadet for meget" invites a
// retry that will fail the same way; a figure tells them to wait or to post less.
func (b *Budget) Remaining(key string) int64 {
	if b == nil || b.limit <= 0 {
		return -1 // unlimited
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	cutoff := b.now().Add(-b.window)
	var used int64
	for _, s := range b.spends[key] {
		if s.at.After(cutoff) {
			used += s.amount
		}
	}
	if used >= b.limit {
		return 0
	}
	return b.limit - used
}
