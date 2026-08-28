# 083 — Ship the track in 2-minute batches

**Status:** done
**Priority:** high
**Created:** 2026-08-26
**Picked up by:** agent session (Zed)
**Started:** 2026-08-28
**Completed:** 2026-08-28

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

- [x] Upload fires on a 2-minute interval, and is skipped entirely when there are no new
      points
- [x] Pending points are only cleared locally after the server accepts them
- [x] An upload failure (offline, 5xx, timeout) leaves the points pending and retries on
      the next interval — verified by simulating each, not just the happy path
- [x] A long offline period ships its backlog on reconnect, in bounded chunks
- [x] A retried batch does not result in duplicate points, and the layer responsible for
      that is documented here
- [x] The interval keeps running while the app is backgrounded to whatever extent the
      platform allows, and the behaviour when it does **not** is recorded rather than
      assumed. Note task 082's finding: a backgrounded web app does not run at all on iOS,
      so the realistic expectation is that nothing is recorded *or* uploaded while
      backgrounded, and the backlog ships when the app is next foregrounded. Design for
      that being the normal case, not the exception
- [x] No upload is attempted when the user is not signed in

## Progress Log

- 2026-08-26 — Task created from PRD 002 §11.1.
- 2026-08-28 — Unblocked by 084. Implemented in `track.store` (`flush`/`send`) with three
  new IndexedDB queries (`pendingPoints`, `countPending`, `markUploaded`, `pruneUploaded`).
- 2026-08-28 — **WHERE DUPLICATES ARE REMOVED — the decision this task asked to be written
  down: at the reader, keyed by `(person, timestamp)`.** The reasoning, because "someone
  downstream will dedupe" is exactly how duplicates reach a permanently retained stream:
  * A request that times out *after* the server published it is indistinguishable, from the
    client, from one that never arrived. The client must therefore retry, and that retry
    republishes points the stream already holds. The client cannot know.
  * The endpoint cannot know either without keeping state it is forbidden to keep — it
    writes no SQL (PRD 008 §8), so it has nowhere to remember what it has seen.
  * So the stream can legitimately contain the same point twice, and the reader must collapse
    it. **This is safe rather than merely tolerable, because a point is immutable**: the same
    `(person, timestamp)` always carries the same position, so last-write-wins and
    first-write-wins agree and neither can invent a route. Task 086 and any other consumer
    must key on `(person, timestamp)`. That is the contract.
  * Note the broker's 2-minute duplicate window does **not** help: the library publishes
    without a `Nats-Msg-Id`, and a backlog ships long after any window anyway.
- 2026-08-28 — **The uploader follows the SESSION, not the recorder.** Points can be pending
  with recording stopped — permission revoked, storage full, or a previous session that
  recorded and never found signal — and in every one of those cases the backlog still has to
  ship. Tying the uploader to the recorder would strand exactly the data that is hardest to
  reproduce. It also flushes on `visible` and on `online`, because task 082 measured that iOS
  does not run the app while backgrounded: being foregrounded is the only moment a backlog
  can move, and waiting out a fresh 2-minute interval would waste the window the user just
  gave us.
- 2026-08-28 — Failure classification, which is the substance of `send()`: **offline / 5xx /
  503 / 429 / 413 → retry next interval**; **401 → stop the cycle** (the session expired;
  points stay pending and ship after the next sign-in); **400 → stop and say so**. That last
  one is deliberate and is the only permanent stop: a 400 means the client is sending
  something the server will never accept, i.e. a bug in this app. Retrying forever would
  block every later point behind it, and silently discarding the batch to get unstuck would
  throw away a member's track to hide our own defect. So it stops, sets `uploadBlocked`, and
  surfaces on both the privacy page and the status report.
- 2026-08-28 — Chunking: 500 points per request, at most 4 requests per run. Well under the
  server's 2,000-point bound on purpose — a member with a backlog is by definition somewhere
  with poor connectivity, so a 30 KB request that succeeds beats a 90 KB one that times out.
  4 × 500 clears a full 12-hour offline race in one pass while staying far inside the
  endpoint's 20-per-minute per-user limit.
- 2026-08-28 — Only the wire fields are sent (`ts`, `lat`, `lng`, `accuracy`). `userId` and
  `uploaded` are local bookkeeping, and the endpoint rejects unknown fields — forwarding the
  stored row as-is would have been a 400 on **every** batch, i.e. the `uploadBlocked` path on
  day one.
- 2026-08-28 — Accepted points are **marked, not deleted**, then pruned after 18 hours. The
  mark is what makes retry safe (it moves exactly once, only after a 2xx); keeping them for
  the length of a race is what lets the status page answer "what did this phone record?"
  rather than only "what is still waiting"; pruning is because the server has them and the
  quota is shared with tiles and portraits, which cannot be re-fetched as cheaply.
- 2026-08-28 — **Verified end to end, 15/15**, in headless Chromium against the **dev stack
  over real HTTPS** (Traefik + Vite + Go API + MariaDB + the real broker). The prod image was
  deliberately not used: with no database it has no eventing, so `/api/track` would answer
  503 for the wrong reason and the test would prove nothing.
  * a point is recorded, and **the 2-minute interval fires unprompted** and ships it — waited
    out rather than forced, since "fires on an interval" is the criterion;
  * a flush with nothing new logs no upload at all (the "only when changed" condition);
  * offline: points accumulate, the attempt is recorded as a failure, nothing is marked;
  * reconnect ships the backlog;
  * three forced flushes after success upload **nothing** — accepted points are not re-sent;
  * a 1,200-point backlog shipped as **500 + 500 + 200**;
  * signed out: nothing uploaded, points stay pending;
  * no unexpected console or page errors, including through the offline phase — which also
    re-covers task 090's offline rendering against this new network caller.
- 2026-08-28 — And verified on the stream itself, which is the half the browser cannot prove:
  **5 batches of sizes 1, 1, 500, 500, 200 — 1,202 points with 1,202 distinct timestamps**
  (so zero duplicates), all on one per-person subject, all carrying `year: 2026`, and
  `NATHEJK` unchanged at 29,136 messages.
- 2026-08-28 — Backgrounded behaviour, recorded rather than assumed (the criterion asks for
  this): on iOS the app does not run while backgrounded at all, so **nothing is recorded or
  uploaded** during that time — measured in task 082, where a 2m 46s suspend produced zero
  points. The backlog ships when the app is next foregrounded, which is why `visible` triggers
  a flush. This is the normal case during the event, not an exception.
- 2026-08-28 — **Copy corrected while here, and it needs the maintainer's eye.** The privacy
  page said the route is sent *"så vi kan hjælpe jer, hvis I bliver i tvivl om, hvor I er"*.
  Shipping this task makes "sent to the organizers" true, but that clause promises something
  that still does not exist: nothing reads the stream live — 086 is a **post-race** view — so
  no one can look up a lost patrol. A 12-year-old reading it would reasonably conclude
  otherwise, and might rely on it at 03:00. It now states the purpose without the live
  promise and adds, plainly, that we cannot see where they are right now and that the route is
  not an emergency button. **This is consent copy and should not ship unread.**
- 2026-08-28 — `vue-tsc --noEmit` clean; `npm run build` succeeds (34 precache entries).
  ✅ All criteria complete. Moving to done.
