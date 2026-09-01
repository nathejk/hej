# 190 — Document the version-check and poll convention

**Status:** done
**Priority:** medium
**Created:** 2026-09-01
**Picked up by:** agent session (Zed)
**Started:** 2026-09-01
**Completed:** 2026-09-01

## Description

PRD 009 §6, §8. The during-event freshness mechanism **already exists** — it was built for the
contacts directory (task 155 for the endpoint, task 162 for the client loop) before PRD 009 was
approved. This task writes it down as the shared convention so the second consumer adopts it
instead of inventing a second one.

Nothing new is built here beyond documentation and, where it is cheap, a shared helper.

What the convention fixes:

- **A separate, cheap version endpoint**, not a poll of the manifest. It is answered from a
  projection read, returns a monotonic version **for the caller's permitted set only**, and is
  `ETag`-able. `GET /api/contacts/version` is the reference.
- **`version` travels in the JSON body**, not in a header. Deliberate: `fetchWrapper` does not
  expose response headers. ETags remain for the browser's own conditional requests.
- **Three trigger points, no more:** `visibilitychange` → visible (and launch), an interval
  while visible, and the `online` event. See `composables/useContactsFreshness.ts`.
- **The interval is served, not built in** — `contacts_poll_seconds` on `/api/config`, from an
  env var, remembered client-side across an offline start. Per dataset. The reason is load: this
  is the app's only continuous during-race traffic and it shares the BFF with position
  reporting, so it must be widenable *during* an event. **0 disables the interval but not the
  foreground and reconnect checks**, so "reduce load" can never silently become "stop updating".
- **Metadata propagates ahead of images.** A corrected phone number arriving a minute before the
  new portrait is fine; the reverse is not.
- **Push is not usable for invalidation.** iOS requires every web push to show a notification
  (`vue/public/push-sw.js` always calls `showNotification`), so it would buzz phones over a
  corrected number or risk the permission.

## Acceptance Criteria

- [x] The convention documented where an implementer will find it — as the header of
      `composables/useFreshnessLoop.ts` (the code they will reuse) and on
      `contactsVersionResponse` (the shape they will copy), plus PRD 009 §8. Not a wiki page nobody
      opens.
- [x] Any new version endpoint carries OpenAPI annotations. *None was added — no endpoint changed
      shape. The existing manifest and version annotations already document `If-None-Match` and the
      version.*
- [x] The poll-interval pattern is stated as per-dataset, with the zero-semantics preserved.
- [x] The client loop **was** cheap to generalise, so it was extracted: `useFreshnessLoop`, with
      `useContactsFreshness` now a thin wrapper.
- [x] Task 171's remaining criterion satisfied — PRD 007 §8 updated (done with task 191, which
      closed 171).

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
- 2026-09-01 — Picked up. Plan: extract the dataset-agnostic half of the shipped loop, and put the convention where the next implementer will trip over it rather than in a document.
- 2026-09-01 — **The loop turned out to contain nothing contacts-specific except what to check**, so
  it generalised almost for free: `useFreshnessLoop` takes a `check`, an interval and an optional
  `enabled` gate, and `useContactsFreshness` is now twenty lines that supply this dataset's two
  decisions — `refreshIfStale`, and skip a role that has no pane rather than letting it 403 sixty
  times an hour.
- 2026-09-01 — **The 11 existing tests were left untouched and still pass.** That was the point of
  extracting rather than rewriting: the tests exercise `useContactsFreshness`, so their passing is the
  evidence that the move preserved behaviour, including the operator kill switch. A rewrite that
  needed new tests would have proved nothing about what shipped.
- 2026-09-01 — **The convention lives in two comment blocks, not a document.** One at the top of
  `useFreshnessLoop.ts` — the eight rules, each with its reason — because that is the file the next
  implementer imports; one on `contactsVersionResponse`, because that is the shape they will copy.
  The rule most likely to be got wrong is spelled out at both ends: **a second dataset needs its own
  served interval**, since two datasets on one number cannot be tuned apart and will not cost the
  same.
- 2026-09-01 — Kept the body-not-header rule explicit with its cause (`fetchWrapper` does not expose
  response headers), because without the cause it reads like an oversight somebody will "fix".
- 2026-09-01 — ✅ All criteria complete. No behaviour change: suite 299 across 24 files,
  `type-check` clean, `go build ./...` clean.
