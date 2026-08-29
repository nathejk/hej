# 136 — BFF: POST /api/me/profile/report-incorrect

**Status:** open
**Priority:** high
**Created:** 2026-08-30
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 005 §8. The endpoint behind the non-punitive "nummeret er forkert" and "jeg kender
ikke nummeret" paths in the profile-confirmation step. It flags the record for organizer
follow-up with a reason distinguishing **"wrong"** from **"unknown to me"**, and lets the
user into the app either way.

Status codes: `204` / `401`.

## Why this is required, not optional

PRD 005 §8 states it outright: a guardian number nobody can confirm is an **operational
problem that has to reach a human before the event**, not just a dead end in the UI. If
the escape paths only unblocked the flow and recorded nothing, the app would be
systematically discovering bad emergency contacts and then discarding the findings — the
worst possible outcome, since the number's whole purpose is to be reachable when
something happens or when a resigning member has to be picked up.

The two reasons are kept distinct because they call for different follow-up: "wrong"
means there is a number on file and it is incorrect; "unknown to me" means the member
cannot vouch for it, which may equally be a member who genuinely does not know their
guardian's number. Collapsing them into one flag throws away the only information an
organizer has for triage.

**Storage.** The flag is stored in this repo. Consistent with tasks 133/134, it enters
via the event log rather than a direct SQL write. How organizers eventually read it is
**follow-up work and not a blocker** for this task — and the correction channel itself is
still open (PRD 005 §12 Q2, shared with PRD 003), so do not block on it either.

## Acceptance Criteria

- [ ] `POST /api/me/profile/report-incorrect` registered in `go/cmd/api/routes.go` behind
      `requireAuth`, handled in `go/cmd/api/profile.go` per `go-bff-layout`
- [ ] Body carries a reason that distinguishes "reported wrong" from "could not confirm";
      an unrecognised reason is rejected rather than stored as free text
- [ ] `204` on success, `401` unauthenticated
- [ ] The user is resolved from the session, never from a client-supplied id
- [ ] The flag is recorded durably in this repo via the write facade — no direct SQL write
- [ ] Reporting does **not** set `verified_at` and does not block the user from continuing
      into the app
- [ ] Repeated reports are harmless (idempotent or additive) — a user who taps twice must
      not get an error
- [ ] OpenAPI annotations present and complete, documenting both reason values
- [ ] No organizer-facing surface is built here, and the task's completion does not depend
      on one existing

## Depends on

- **Task 134** — the `person` projection this flag hangs off.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
