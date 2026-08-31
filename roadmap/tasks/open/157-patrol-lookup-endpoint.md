# 157 — Crew patrol lookup endpoint

**Status:** open
**Priority:** high
**Created:** 2026-08-31

## Description

`GET /api/contacts/patrols/{number}` — one patrol's members with portrait ref, current
status and phone number. **Crew only.** This is the only path by which a spejder's
details are reachable in the PWA (PRD 007 §6, §8).

The design is deliberately narrow, and each constraint is load-bearing:

- **Exact match on the full patrol number.** No prefix matching, no listing, no
  "recent lookups", no patrol picker. The endpoint answers "show me patrol 138" and
  refuses to answer "which patrols exist".
- **`403` and `404` indistinguishable**, including for a bandit klan number, so it
  cannot be used to enumerate patrols or probe the klan numbering.
- **`Cache-Control: no-store`**, and excluded from the service worker's routes
  (task 170). Nothing from a lookup is ever written to a device — that is what keeps
  "scope the payload, not the view" intact, and it is why ~557 spejder thumbnails are
  *not* shipped to every crew device.
- **Every call is logged server-side** (PRD 007 §11.7). Because the lookup is online by
  definition, this is a log line on the handler — no client queue, no batch upload, no
  ingestion endpoint.
- **All crew**, including the generic `crew` fallback (2026-08-31, deliberate).
- `phoneParent` is absent, as everywhere (`.rules`).

Companion endpoint for the images:
`GET /api/contacts/patrols/{number}/photo/{personId}?size=thumb`, also `no-store`, so
spejder images stay off the general portrait route and "never cached" is a property of
the endpoint rather than a convention.

## Acceptance Criteria

- [ ] Both endpoints behind `app.requireAuth`, crew-only via task 151's function.
- [ ] Exact-match number only; no prefix/partial/list behaviour exists.
- [ ] `403` and `404` byte-identical, including for a valid klan number.
- [ ] `Cache-Control: no-store` on both responses.
- [ ] Members carry name, status and phone; `phoneParent` absent.
- [ ] Each lookup writes an audit log line (caller, patrol, timestamp).
- [ ] Tests: crew succeeds; bandit, gøgler and spejder are refused indistinguishably;
      probing a klan number reveals nothing.
- [ ] OpenAPI annotations present.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §6 / §8.
