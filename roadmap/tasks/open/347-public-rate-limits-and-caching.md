# 347 — Rate limits and cache headers on the public routes

**Status:** open
**Priority:** high
**Created:** 2026-09-19
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 011 §6, §8. Every new public route gets a by-IP read limit and a deliberate cache header.

**Why this is its own task and not an afterthought:** every other route in this repo is behind a login, which
is an extremely effective rate limiter. These are not. And the load profile is genuinely new — this page is
shared in family WhatsApp groups the morning after the event, so **a hundred simultaneous visitors is the
normal case**, not the stress case.

Follow `publicGlimtReadLimiter` in `go/cmd/api/glimtpublic.go`, and carry across the reasoning already
recorded there:

- **A separate limiter from the member-keyed ones.** An IP is a much worse key — a school or a workplace
  behind one NAT shares a single budget — so the ceiling has to be correspondingly generous, and mixing the
  two drags the member-keyed limit down with it.
- **Cache-Control: public, max-age=60** on the HTML, the same short window the glimt page chose: shareable,
  but a takedown lands quickly. That window is also what task 335 depends on for "promptly", so do not
  lengthen it without reading that task.

**The expensive route is the patrol map endpoint** (task 342), because it reads a merged track. Task 340
already requires that result to be cached or materialised per patrol — verify here that the public endpoint
actually reads the cache and cannot be made to walk the stream by a stranger with a loop.

## Acceptance Criteria

- [ ] Every new public route (`/offentligt*`, `/api/public/*`) is by-IP rate limited, using a limiter separate
      from the member-keyed ones.
- [ ] Limits are generous enough for a NAT'd school; the chosen ceilings and their reasoning recorded here.
- [ ] `Cache-Control` set deliberately on every public response, `max-age=60` on HTML unless a route argues
      otherwise in a comment.
- [ ] The patrol map endpoint demonstrably reads the cached/materialised track and cannot be driven to walk
      `TELEMETRY` per request.
- [ ] Media responses cache longer than HTML — content-addressed bytes do not change.
- [ ] A rate-limited response returns a Danish message a visitor can understand, matching the existing
      `RateLimitMessageResponse` usage rather than a bare 429.
- [ ] Load-checked against the morning-after scenario: a burst on one patrol page and on the frontpage.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §6 / §8 / §10 (throughout).
