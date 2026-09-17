# 311 — Rate limits and a storage ceiling for Glimt

**Status:** done
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:** agent
**Started:** 2026-09-17
**Completed:** 2026-09-17

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

- [x] Per-member glimt-per-hour and bytes-per-hour limits, configurable
- [x] A separate, looser read limiter — test asserts a burst of feed/media reads is not throttled at browse-like rates
- [x] Per-member and total storage ceilings, configurable, `0` meaning unlimited
- [x] Exceeding a ceiling returns 429 (or 507) with a message naming the limit — **both**, and the
      choice between them is deliberate: see the log
- [x] Tests for each limit boundary
- [x] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.

- 2026-09-17 — **The byte budget needed a new type.** `ratelimit.Limiter` answers "how many times",
  which is right for a PIN request and wrong for an upload: sixty thumbnails and sixty 12 MiB videos
  are the same number of events and nowhere near the same cost. Added
  `go/internal/ratelimit/budget.go` — a per-key sliding-window sum — rather than widening `Limiter`,
  whose one-argument `Allow(key)` has a dozen callers that are honestly counting events.

  Two decisions inside it worth keeping:

  - **A refusal costs nothing.** A rejected charge is not recorded, so a member who tries a large
    video and is refused can still post the small photo they settle for — rather than having the
    refusal itself eat the room for it.
  - **An oversized charge is refused, not clamped.** Clamping would let one over-budget item through
    per window and read as the limit not working.

  Sliding rather than fixed window, matching `Limiter`: a fixed-window counter lets a client spend
  two full budgets back-to-back by straddling the boundary, and there is a test that distinguishes
  the two.

- 2026-09-17 — **The read limiter is the point of the task, and the test that matters asserts that
  nothing is throttled.** `glimtReadLimiter` is 600/minute per member — two orders of magnitude
  looser than the writes *and* measured over a shorter window.

  Both halves are deliberate. PRD 019 §0a.3 describes the post-race browse as the load peak of the
  feature — a thousand people at the finish line, each pulling a grid of thumbnails per screen — and
  it is the use the feature was *built* for. A limiter anywhere near the upload numbers (60/hour)
  would refuse the fourth screen, and an *hourly* read budget would be spent by somebody scrolling
  for two minutes and then lock them out for fifty-eight.

  `TestGlimtReads_ABrowseBurstIsNotThrottled` drives 200 reads through the real handler with the
  **production numbers wired in**, including the write limiters at their production values — so if
  reads and writes ever share a limiter again, that is what fails. Paired with a hold-grid version,
  since the grid is what the browse actually hammers, and with one asserting reads are still bounded,
  because it is still a limiter.

  This is worth naming as a trap rather than an optimisation: **a throttled browse does not look like
  a bug from the server's side. It looks like a working rate limiter.** The moderation queue is
  deliberately *not* limited — three accounts, and the failure mode is a moderator throttled out of a
  takedown while a photograph somebody objected to stays up.

- 2026-09-17 — **Found a live bug while making the limits configurable.** `Limiter.Allow` refuses
  when `len(kept) >= l.limit`, so `ratelimit.New(0, w)` **blocks every request**. Every existing
  caller passes a positive constant, so this had never mattered — but the moment these became
  configurable with the documented "0 means unlimited" convention, `GLIMT_PER_HOUR=0` would have
  taken the whole feature down for everyone, silently, and looked like a working rate limit.

  Fixed with `limiterOrNil` in `glimt.go` rather than by changing `ratelimit.New`, whose "0 allows
  nothing" reading is defensible on its own terms and whose other callers are fine. Returning nil is
  also what the handlers already understand. `TestLimiterOrNil_ZeroDisablesRatherThanBlocks` records
  it.

- 2026-09-17 — **PRD §11 Q8 answered: reject, never evict.** Eviction would mean this feature's one
  irreversible operation firing with no human involved and nobody told — and it would delete one
  member's memories to make room for another's. Retention (task 310) is what frees space, on a
  schedule everybody was told about. PRD updated.

  Two ceilings with **two different status codes**, and the distinction is what decides what the
  client tells the member:

  - **Per member → 429.** Their quota, and they can act on it. The message says so: *"Slet et glimt
    for at gøre plads."* "Du har uploadet for meget" would invite a retry that fails identically.
  - **Total → 507** (`InsufficientStorageResponse`, new). The server is out of room; nothing the
    member did wrong and nothing they can do. A 429 there would be a lie, and "try again" invites a
    loop. Logged at error level, because unlike a 503 this is not a designed degraded state.

  **The check fails open** — the opposite of how the moderation check fails, and asserted. A ceiling
  is a safety margin, not an authorization: the cost of wrongly allowing an upload is some disk,
  which retention reclaims and an operator can see, while the cost of wrongly refusing one is a
  member standing in a forest losing a photograph they cannot retake.

- 2026-09-17 — **On measuring storage: the numbers overcount, on purpose.** `StoredBytes` /
  `TotalBytes` sum `glimt_media.bytes`, but content addressing means identical bytes are **one
  object** — two members posting the same screenshot occupy one blob between them. So the sum exceeds
  real disk use whenever media are shared.

  That is the right way round for both uses. As a **per-member quota** it is arguably *more* correct
  than the disk figure: a member's budget should reflect what they posted, not whether a stranger
  happened to post the same file first — otherwise the second person to share a popular image gets it
  free and their neighbour's quota depends on someone else's timing. As an input to the **total**
  ceiling it errs towards refusing early, which is the safe direction for a volume that cannot be
  replayed from the stream.

  Hidden glimt are counted (hiding does not delete, so the bytes are still there); deleted ones are
  not. `TestStoredBytesQueryShape` asserts all three properties off the query text, because none of
  them is observable from a stubbed result — including the `COALESCE`, without which a member who has
  posted nothing (the most common caller) would produce a scan error and the ceiling would fail open
  on every first upload.

  The charged size is the **uploaded** size, not the re-encoded one: it is the honest measure of what
  the member cost the link and the CPU, it is knowable at the point of checking, and it cannot be
  gamed by sending something that compresses well after resampling. The stored figure ends up
  smaller, so the ceiling refuses slightly early.

- 2026-09-17 — Config, all `0` for unlimited: `GLIMT_PER_HOUR` (20), `GLIMT_MEDIA_PER_HOUR` (60),
  `GLIMT_BYTES_PER_HOUR` (200 MiB), `GLIMT_READS_PER_MINUTE` (600),
  `GLIMT_MEMBER_STORAGE_BYTES` (500 MiB), `GLIMT_TOTAL_STORAGE_BYTES` (off). Added `envInt64`, plain
  digits only — no "500MB" suffix parsing, because a suffix a typo turns into a different magnitude
  is worse than a long number, and these are written once in a compose file.

  Not added to `docker-compose.yml`: the defaults are the intended values and listing them would be
  a second place to keep correct. **The byte figures are guesses**, like the retention defaults in
  §11 Q1 — 500 MiB per member is roughly 500 photographs, and the total is deliberately off until
  somebody knows the volume size. Both should be reviewed before the event.

  Also: the task 312 annotation guard immediately caught the new 507 branch as undocumented, which
  is exactly what it was built for.

  Full Go suite, `gofmt` and `go vet` clean. **None of this has been exercised under real load** —
  task 324 is the load test, and it is the thing that would tell us whether 600 reads/minute is the
  right number.
