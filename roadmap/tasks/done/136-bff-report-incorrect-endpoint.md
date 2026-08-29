# 136 — BFF: POST /api/me/profile/report-incorrect

**Status:** done
**Priority:** high
**Created:** 2026-08-30
**Picked up by:** agent session (Zed)
**Started:** 2026-08-30
**Completed:** 2026-08-30

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

- [x] `POST /api/me/profile/report-incorrect` registered in `go/cmd/api/routes.go` behind
      `requireAuth`, handled in `go/cmd/api/profile.go` per `go-bff-layout`
- [x] Body carries a reason that distinguishes "reported wrong" from "could not confirm";
      an unrecognised reason is rejected rather than stored as free text
- [x] `204` on success, `401` unauthenticated
- [x] The user is resolved from the session, never from a client-supplied id
- [x] The flag is recorded durably in this repo via the write facade — no direct SQL write
- [x] Reporting does **not** set `verified_at` and does not block the user from continuing
      into the app
- [x] Repeated reports are harmless (idempotent or additive) — a user who taps twice must
      not get an error
- [x] OpenAPI annotations present and complete, documenting both reason values
- [x] No organizer-facing surface is built here, and the task's completion does not depend
      on one existing

## Depends on

- **Task 134** — the `person` projection this flag hangs off.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
- 2026-08-30 — **Endpoint implemented**, in the same commit as task 135 for the reason recorded
  there: shared file, shared limiter, shared error helpers.

  Decisions:

  - **A new `person.GuardianReported` event** rather than a column, and **nothing consumes it.**
    That is deliberate rather than unfinished: PRD 005 §4 puts the organizer-facing surface out of
    scope and §12 has not decided who reads the flag or where, so a column now would be a guess at a
    shape. The log keeps every report with its reason and timestamp until there is a consumer to
    project them for — which is the cheap direction on an append-only log, and the honest one given
    the open question. It is written up in the type's doc comment so the absence of a projection
    reads as a decision rather than an omission.
  - **The reason is a closed pair, validated at the edge and rejected if unknown.** Coercing an
    unrecognised value to `wrong` would tell an organizer to correct a number the member never
    claimed was incorrect. `wrong` is a record to *fix*, `unknown` is a record to *check* — different
    jobs, so the distinction has to survive to the log.
  - **Additive, therefore trivially idempotent-enough:** two taps publish two reports, which is
    harmless and arguably informative (a member who reports twice is telling us something). No error
    on a repeat, and nothing to reconcile.
  - **Reachable regardless of `confirmation_required`.** A member whose number was verified last
    week may still discover today that it is wrong, so the report path is not gated on the
    confirmation being outstanding. There is a test for exactly that.
  - It never touches `verified_at`, and never blocks: the member continues into the app on every
    path, per PRD 005 §5.
- 2026-08-30 — The correction channel (PRD 005 §12, shared with PRD 003) remains open and was not
  invented here. The client (task 128) carries the honest minimum — Nathejk has been told and will
  check it — and tells the member plainly when the report could not be sent, which is only possible
  because this endpoint reports failures rather than swallowing them.
- 2026-08-30 — ✅ Five tests: both reason codes round-tripped through the published JSON, an unknown
  reason refused with nothing published, a verified member still able to report, and auth required.
