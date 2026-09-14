package main

import (
	"strconv"
	"sync"
	"time"
)

// Per-login-session state for the contact-number check (PRD 015, tasks 227 and 228).
//
// # Why this exists at all
//
// A member who cannot recall the last two digits of their contact number must not be trapped in
// the check. After three failed attempts the step ends by itself and the outcome is recorded, the
// same as if they had tapped "spring over" (PRD 015 §6). Counting those attempts is the only piece
// of that which cannot live in the client: a `localStorage` counter is reset by a reload, and the
// point of the limit is that the member gets out of the loop, not that they are policed.
//
// # Why it is keyed by (user, session expiry) and not by user alone
//
// The maintainer's decision was three attempts **per login session**, reset by a new login: a
// member who comes back tomorrow having asked their mother should get to try again (PRD 015 §6).
// Sessions here are signed cookies with no server-side identity of their own — `session.Session`
// carries a user id, a role and an expiry — so the expiry is what distinguishes one login from the
// next. Two logins by the same member land on different keys because they were issued at different
// times.
//
// The one imprecision: two logins within the same second collide and share a budget. That is
// accepted; the cost is a member getting three attempts instead of six in a situation that does
// not arise in practice.
//
// # Why in memory, and what a restart costs
//
// A restart forgets every counter, so a member mid-check gets three fresh attempts. That is
// deliberate (PRD 015 §6 Non-Functional): the failure mode of forgetting is generous, while the
// failure mode of not counting server-side at all is that the limit does not exist. Nothing here
// is worth a table.
//
// Note what is NOT stored: the digits, the number, or anything about who the contact is. Just a
// count and a bit.
type contactCheck struct {
	mu     sync.Mutex
	states map[string]*contactCheckState
	ttl    time.Duration
	now    func() time.Time
}

type contactCheckState struct {
	failures int
	// closed means the check is over for this login session: the member exhausted their
	// attempts or gave up, and an outcome has been recorded. It is what makes a double submit
	// idempotent — a second "spring over" tap, or a retry after a dropped connection, must not
	// read as two members' worth of signal.
	closed bool
	seen   time.Time
}

// maxContactCheckAttempts is the number of wrong answers a member may give before the check ends
// by itself.
//
// Three, from PRD 015 §6. Not tunable at runtime on purpose: this is a product decision about how
// long to let a 13-year-old fail at recalling their parent's number, not an operational knob.
const maxContactCheckAttempts = 3

func newContactCheck(ttl time.Duration) *contactCheck {
	return &contactCheck{
		states: make(map[string]*contactCheckState),
		ttl:    ttl,
		now:    time.Now,
	}
}

// contactCheckKey identifies one login session.
//
// Built from the session rather than the request so that a member cannot get a fresh budget by
// clearing a cookie value they control — the expiry is inside the signed payload.
func contactCheckKey(userID string, expiresAt time.Time) string {
	return userID + "|" + strconv.FormatInt(expiresAt.Unix(), 10)
}

// Failed records one wrong answer and reports how many attempts remain and whether that failure
// ended the check.
//
// Returning both is what lets the endpoint answer in one shot: the client needs "you have two
// tries left" and "that was the last one" from the same response, and computing the second from
// the first at the call site is how the two drift.
func (c *contactCheck) Failed(key string) (remaining int, closed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pruneLocked()

	st := c.stateLocked(key)
	if st.closed {
		return 0, true
	}
	st.failures++
	if st.failures >= maxContactCheckAttempts {
		st.closed = true
		return 0, true
	}
	return maxContactCheckAttempts - st.failures, false
}

// Close ends the check for this login session and reports whether it was already over.
//
// The boolean is the idempotency guard: the caller publishes an outcome only the first time.
func (c *contactCheck) Close(key string) (alreadyClosed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pruneLocked()

	st := c.stateLocked(key)
	if st.closed {
		return true
	}
	st.closed = true
	return false
}

// Closed reports whether the check is over for this login session, without changing anything.
func (c *contactCheck) Closed(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	st, ok := c.states[key]
	return ok && st.closed
}

func (c *contactCheck) stateLocked(key string) *contactCheckState {
	st, ok := c.states[key]
	if !ok {
		st = &contactCheckState{}
		c.states[key] = st
	}
	st.seen = c.now()
	return st
}

// pruneLocked drops entries nobody has touched for a while.
//
// Without it this map grows for the life of the process, once per login, forever. The TTL is not a
// security property — a stale entry only ever costs a member attempts they were not using — so it
// is swept opportunistically on write rather than by a goroutine nobody would remember to stop.
func (c *contactCheck) pruneLocked() {
	if c.ttl <= 0 {
		return
	}
	cutoff := c.now().Add(-c.ttl)
	for key, st := range c.states {
		if st.seen.Before(cutoff) {
			delete(c.states, key)
		}
	}
}
