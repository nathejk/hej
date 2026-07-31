// Package ratelimit provides a small in-memory, per-key fixed-window rate
// limiter used to throttle sensitive endpoints (e.g. auth PIN requests) per IP
// and per phone.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter allows up to `limit` events per `window` for each key.
type Limiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
	now    func() time.Time
}

// New returns a Limiter allowing `limit` events per `window`.
func New(limit int, window time.Duration) *Limiter {
	return &Limiter{
		hits:   make(map[string][]time.Time),
		limit:  limit,
		window: window,
		now:    time.Now,
	}
}

// Allow records an event for key and reports whether it is within the limit.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	cutoff := now.Add(-l.window)

	kept := make([]time.Time, 0, len(l.hits[key]))
	for _, t := range l.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}

	if len(kept) >= l.limit {
		l.hits[key] = kept
		return false
	}

	l.hits[key] = append(kept, now)
	return true
}
