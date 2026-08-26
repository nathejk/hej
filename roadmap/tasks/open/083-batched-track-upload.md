# 083 — Ship the track in 2-minute batches

**Status:** open
**Priority:** high
**Created:** 2026-08-26
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 002 §11.1. Every **2 minutes**, and **only when the track has new points**, upload the
pending points as one batch to `POST /api/track` (task 084).

Depends on 082 (the local track) and 084 (the endpoint).

## Why the "only if changed" condition matters

It is not a micro-optimisation. A phone that is stationary — a member asleep at a
checkpoint, or one whose GPS has not produced a new fix — would otherwise pay for an upload
every 2 minutes for hours and send nothing. Over a 12-hour race that is 360 pointless
requests per idle participant, on rural mobile data, from a battery that has to last the
night.

## Why batches, not per-point

The metatagger envelope is ~250 bytes per message against ~76 bytes per point. At the
chosen 30 s sampling a 2-minute batch carries 4 points, so batching sends 554 bytes where
four individual messages would send 1,304 — **2.4× less**, and 2.4× fewer messages for the
broker to store forever.

## Retry and duplicates

The two failure modes to get right, because both are ordinary rather than exceptional:

- **Offline for a long time.** Points stay pending and ship as a backlog when connectivity
  returns. A member offline for three hours must not lose three hours of track. Consider
  capping the size of a single batch so the backlog ships in chunks rather than one
  request large enough to be rejected.
- **A batch that may or may not have been accepted.** A request that times out after the
  server published it must not silently drop those points, and retrying it must not
  duplicate them. Each point is identified by (person, timestamp) from 082, so duplicates
  are *detectable* — but decide deliberately where they are removed: at the client before
  sending, at the endpoint, or by whatever reads the stream. Writing that down is part of
  this task, because "someone downstream will dedupe" is how duplicates reach a permanent,
  indefinitely retained stream.

Points are only discarded locally once the server has accepted them.

## Acceptance Criteria

- [ ] Upload fires on a 2-minute interval, and is skipped entirely when there are no new
      points
- [ ] Pending points are only cleared locally after the server accepts them
- [ ] An upload failure (offline, 5xx, timeout) leaves the points pending and retries on
      the next interval — verified by simulating each, not just the happy path
- [ ] A long offline period ships its backlog on reconnect, in bounded chunks
- [ ] A retried batch does not result in duplicate points, and the layer responsible for
      that is documented here
- [ ] The interval keeps running while the app is backgrounded to whatever extent the
      platform allows, and the behaviour when it does **not** is recorded rather than
      assumed. Note task 082's finding: a backgrounded web app does not run at all on iOS,
      so the realistic expectation is that nothing is recorded *or* uploaded while
      backgrounded, and the backlog ships when the app is next foregrounded. Design for
      that being the normal case, not the exception
- [ ] No upload is attempted when the user is not signed in

## Progress Log

- 2026-08-26 — Task created from PRD 002 §11.1.
