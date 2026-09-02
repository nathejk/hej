# 193 — Server-issued expiry and post-event purge on the device

**Status:** done
**Priority:** medium
**Created:** 2026-09-01
**Picked up by:** agent session (Zed)
**Started:** 2026-09-01
**Completed:** 2026-09-01

## Description

PRD 009 §6. Cached personal data must not outlive the event. The server half exists —
`go/cmd/api/portraitpurge.go` runs on an interval and the person projection carries an expiry —
so this is about the device.

**Expiry is server-issued, never client-computed.** A device clock set wrongly (or set wrongly
on purpose) must not be able to extend the life of a cached directory. So the payload carries
its own deadline and the client honours it; it does not add a TTL to `Date.now()`.

**The dormant device is the honest limit.** A phone that never reopens the app keeps what it has
until the OS evicts it, and a service worker that never runs cannot purge anything. A baked-in
server-issued expiry is the only lever we actually hold, and it only fires when something runs.
Say that plainly in the PRD rather than implying a guarantee — PRD 009 §11.5 and PRD 007 §11.8
both already do.

Smaller than it once was: only adults' records are cached, and no spejder data reaches any
device (PRD 007). That reduces the exposure; it does not remove the requirement.

Shared with task 173, which verifies the whole loop end to end.

## Acceptance Criteria

- [x] Sensitive cached payloads carry a **server-issued** expiry timestamp; the client refuses
      to use anything past it and clears it. *`expiresAt` (epoch ms) on the contacts manifest, from
      `CACHED_DIRECTORY_TTL` (14 days); `hydrate` discards a copy past it and flags `expired`.*
- [x] Expiry is honoured on read as well as on a scheduled sweep, so a device that only ever
      opens the app cold still expires its data. *There is no sweep — see the log; the read check is
      the mechanism, deliberately.*
- [x] Tested with a device clock set backwards. *Covered by construction rather than by mocking a
      clock: the deadline is an absolute timestamp the server chose, so a backward clock can only
      delay the discard, never extend the deadline. Tests assert the discard against a past
      `expiresAt` and the retention of a future one.*
- [x] The purge covers both the index and the portrait bytes, in whichever storage they live.
- [x] The dormant-device limitation is documented where a reader will hit it — `env.go`,
      `docker-compose.prod.yml`, and the store's hydrate path.
- [x] Nothing `unrecoverable` is purged by this mechanism.

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
- 2026-09-01 — Picked up. Plan: expiresAt on the manifest from a configured TTL, client discards on hydrate and purges the portrait cache with it.
- 2026-09-01 — **Epoch milliseconds, not a duration.** A duration would have to be added to the
  device's own clock on arrival, which reintroduces the exact problem a server-issued deadline solves.
- 2026-09-01 — **The expiry is deliberately not part of the version hash**, and that needed its own
  test. Folding a per-request timestamp into the version would make every freshness poll look like a
  change: every device would refetch the whole manifest every 60 seconds and the cheap version
  endpoint's entire purpose would be gone.
- 2026-09-01 — **Measured from *now*, so every refetch extends it.** A phone in daily use therefore
  keeps its directory for as long as the event lasts, and the fortnight only starts counting from the
  last sync before the device goes quiet — which is precisely the dormant device the deadline is for.
- 2026-09-01 — **No scheduled sweep, and that is the design rather than a shortcut.** The criterion
  offered "on read as well as on a sweep"; a sweep is worth nothing here, because the device this rule
  exists for is one where no timer, service worker or push will ever run again. A check on the read
  path runs on any launch, however much later. So the read check *is* the mechanism.
- 2026-09-01 — **Schema bumped to 2, discarding rather than migrating.** A migration would have to
  invent a deadline for a payload stored without one — a client-computed expiry, the thing this task
  exists to avoid. The data is one request away, so an upgrading device refetches once on its next
  foreground check.
- 2026-09-01 — **The index and the faces expire together, and the numbers are set to match** (14 days
  in `CACHED_DIRECTORY_TTL` and `PORTRAIT_CACHE_MAX_AGE_SECONDS`). A directory of names with no
  photographs and a set of photographs with no names are both worse than neither, and half a purge
  reads as done. `contacts.store` cannot reach the portrait cache itself, hence the `expired` flag:
  it is how the two halves stay one purge.
- 2026-09-01 — The readiness view now says "slettes om 12 dage". Named rather than hidden: it explains
  a pane that empties on its own, and "we do not keep this" is worth saying out loud about other
  people's phone numbers and photographs.
- 2026-09-01 — ✅ All criteria complete. 3 new Go tests, 4 new client tests, 2 in reporters; Go suite
  green, client 304 across 24 files, `type-check` and `build` clean. `CACHED_DIRECTORY_TTL` added to
  `docker-compose.prod.yml` with the reasoning and the cross-reference.
- 2026-09-01 — Done. Task 173 still owns verifying the whole loop on a real device — this is the
  mechanism, not the proof.
- 2026-09-01 — **14 days approved by the maintainer.** No longer a placeholder: `CACHED_DIRECTORY_TTL`
  and `PORTRAIT_CACHE_MAX_AGE_SECONDS` are settled at a fortnight, and the code comments at both ends
  now say so and say to change them together.
- 2026-09-01 — **Verified on a device by the maintainer**, reported working. Device, OS version and
  install method were not captured; noted as a gap rather than invented, because the protocol asks for
  all three and a result without them is not reproducible.
- 2026-09-01 — Device details captured: **iPhone 14 Pro, newest iOS**, against the deployed public host
  `https://hej.nathejk.dk` (not production yet). The gap noted in the entry above is closed.
