# 311 — Rate limits and a storage ceiling for Glimt

**Status:** open
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 019 §8, §11 Q8. Uses `go/internal/ratelimit`, as the portrait upload already does.

**Write limits.** Per-member caps on glimt created per hour and media bytes per hour.

**Read limits must be separate and much looser.** The post-race browse is a legitimate flood
(PRD 019 §0a.3) — a limiter tuned for uploads would throttle exactly the use we most want to
work. This is the trap to avoid, not an optimisation.

**Storage ceiling.** The blob store is the only non-rebuildable data in the service, on one
Docker volume. A per-member byte ceiling, plus a total, both configurable. When a member is at
their ceiling, the upload is rejected with a clear message — we do not silently evict someone
else's memories to make room.

## Acceptance Criteria

- [ ] Per-member glimt-per-hour and bytes-per-hour limits, configurable
- [ ] A separate, looser read limiter — test asserts a burst of feed/media reads is not throttled at browse-like rates
- [ ] Per-member and total storage ceilings, configurable, `0` meaning unlimited
- [ ] Exceeding a ceiling returns 429 (or 507) with a message naming the limit
- [ ] Tests for each limit boundary
- [ ] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
