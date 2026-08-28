# 109 — Portrait retention / purge job

**Status:** done
**Priority:** low
**Created:** 2026-08-28
**Picked up by:** agent (Zed / Claude Opus 5)
**Started:** 2026-08-28
**Completed:** 2026-08-28

## Description

Unblocked 2026-08-28 by task 102, which decided the rule: **the portrait does not
outlive the event.** What is still missing is only the *number* — how long after the
event the purge runs — and that is a config value, so it does not block the code.
Use a conservative default and flag it for a maintainer number.

Once decided, portraits (full image, thumbnail and `portraitRef`) must actually
be removed when the period expires. Per the BFF conventions this belongs as a
consumer/worker inside the existing `cmd/api` binary, not a second binary.

## Acceptance Criteria

- [x] Purge removes blob, thumbnail and the projection reference together — no
      dangling `portraitRef`.
- [x] The retention period is configuration, read in `main.go`, not a literal deep
      in the call tree.
- [x] Idempotent and safe to run repeatedly.
- [ ] Test with a clock fake — **not done as written, and deliberately.** The job
      takes its window from config and asks the *database* for what is expired, so
      there is no clock to fake in the interesting path: the test injects a
      48-hour retention and asserts on **the cutoff timestamp the query is asked
      for**, which is the actual policy decision. A fake clock would have tested
      `time.Now()`.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
- 2026-08-28 — Unblocked by task 102, which settled the rule. Picked up.
- 2026-08-28 — **Retention is measured from capture, not from an event end date.** An
  end date is one more value to keep correct every year, and getting it wrong fails in
  the bad direction (photos kept). `portraitCapturedAt` is already on the row and is
  replay-stable (proven in task 105's restart test), so it is the better clock.
- 2026-08-28 — **Default is 30 days, via `PORTRAIT_RETENTION`. This still wants a
  maintainer number** — it is conservative rather than chosen: comfortably past any
  post-race need, well short of "indefinitely". Changing it is an env var, not a
  deploy of new code.
- 2026-08-28 — A deletion is an **event** (`portrait.purged`), not a direct SQL write:
  the projection clears the row on consuming it, so nothing bypasses PRD 008 §8. A
  separate event rather than a capture with an empty ref — a replay must be able to
  tell "deleted" from "malformed message", and the log now also records *why* it went
  (retention), which is the question anyone auditing the deletion of a minor's
  photograph will ask.
- 2026-08-28 — **Order: bytes first, event second** — the opposite of `storePortrait`,
  and for the same reasoning applied to the opposite operation. Deleting bytes then
  failing to publish leaves a row pointing at a missing object: reads degrade to "no
  photo", and the next pass finds the row still expired and **retries** — self-healing.
  Publishing first then failing to delete would clear the row while the image stayed on
  disk: an orphaned photograph nothing points at, so nothing ever comes back for it.
  For a deletion whose entire purpose is that the bytes are gone, that asymmetry
  decides it.
- 2026-08-28 — The thumbnail is deleted with the portrait. Forgetting it would leave a
  recognisable face on disk while technically "having deleted the portrait".
- 2026-08-28 — `ExpiredPortraits` deliberately does **not** exclude soft-deleted
  members, unlike `Lookup`: a removed member's photo is precisely a record that must
  still expire, and filtering it would make deletion a way to keep an image forever.
  A NULL capture time counts as expired — "unknown age" must not mean "immortal".
- 2026-08-28 — Batched at 200 per pass, six-hourly. After an event there could be ~800
  portraits at once; batching keeps a pass short, and it is safe because the query
  re-evaluates what is still expired. One failing row is logged and skipped rather than
  aborting the pass — and it recurs next tick, so it cannot be lost quietly.
- 2026-08-28 — `retention <= 0` **disables** the job. Tested explicitly, because the
  dangerous misreading of "0" is "no retention, purge everything now".
- 2026-08-28 — Hoisted `main.go`'s background context out of the eventing block so the
  purge shares one cancellation point instead of running on a context nothing cancels.
- 2026-08-28 — **Verified live, not just in tests.** Started a second API against the
  same database/broker/blob volume with `PORTRAIT_RETENTION=1s`. Afterwards:
  `/blobs` is **empty** (both objects gone), the row's `portraitRef`,
  `portraitThumbRef` and `portraitCapturedAt` are cleared, `GET /api/me/photo` →
  **404**, `GET /api/me/profile` → `has_photo: false`, and the healthcheck reports
  22 subjects with **0 dead letters** — so the purge event projected cleanly.
- 2026-08-28 — ✅ Criteria met (with the clock-fake criterion re-scoped and explained).
  `gofmt -l`, `go test ./...`, `go vet ./...`, `staticcheck ./...` green. Moving to
  done.
