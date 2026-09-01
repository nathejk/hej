# 190 — Document the version-check and poll convention

**Status:** open
**Priority:** medium
**Created:** 2026-09-01

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

- [ ] The convention documented where an implementer will find it — PRD 009 §8 plus a comment
      at the reference implementation, not a wiki page nobody opens.
- [ ] Any new version endpoint carries **OpenAPI annotations** (`.rules`).
- [ ] The poll-interval config pattern is stated as per-dataset, with the 0-semantics preserved.
- [ ] Where the client loop generalises cheaply, it is extracted; where it does not, that is
      recorded rather than forced.
- [ ] Task 171's remaining criterion — PRD 007 §8 updated with the outcome — is satisfied or
      explicitly handed back to 171.

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
