# 154 — `GET /api/contacts/manifest`

**Status:** open
**Priority:** high
**Created:** 2026-08-31

## Description

The cached directory's data source (PRD 007 §8). Returns the people the caller may
**list** — never a spejder — with everything the pane needs offline.

Per person: id, name, group id + label, own-group flag, phone, still-in-race flag,
portrait version/etag, whether a portrait exists.

Requirements that are easy to miss:

- **`phoneParent` must never appear.** `.rules` invariant; project it out server-side
  rather than trusting the client. Task 159 is the tripwire test.
- **No postal address**, no guardian number: the allow-list is avatar, name, group,
  phone, and function/section for crew (PRD 007 §11.4).
- **Delta support** via `If-None-Match` / version, and it must be able to express
  **removal of a field** — a withdrawn member's phone number has to disappear from a
  device that already synced it, or the purge in task 160 is decorative. Simplest
  approach: re-issue the record and have the client replace rather than merge.
- Spejder callers get `403`.
- OpenAPI annotations are mandatory (`.rules`).

Depends on task 150 (where status comes from), 151 (authorization), 152 (placement),
153 (grouping).

## Acceptance Criteria

- [ ] `GET /api/contacts/manifest` behind `app.requireAuth`, in a new
      `go/cmd/api/contacts.go`.
- [ ] Returns only listable people for the caller's role, via task 151's function.
- [ ] Carries id, name, group id + label, own-group flag, phone, still-in-race flag,
      portrait etag, has-portrait.
- [ ] `phoneParent`, address, postal code and city are absent from the response type
      itself — not merely unset.
- [ ] `If-None-Match` returns `304` when unchanged.
- [ ] Field removal propagates: a test syncs, clears a phone number, re-syncs, and the
      number is gone from the payload.
- [ ] Spejder gets `403`; unauthenticated gets `401`.
- [ ] OpenAPI annotations present.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §8.
