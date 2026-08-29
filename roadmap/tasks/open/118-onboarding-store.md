# 118 — Onboarding store: resumable step machine

**Status:** open
**Priority:** high
**Created:** 2026-08-30
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 005 §8 (Frontend). Add `vue/src/stores/onboarding.store.ts`: the step machine behind
`/welcome`, plus the per-device "onboarding complete" flag the router gate reads.

Two kinds of state, and keeping them apart is the point:

- **Per-device**, in `localStorage` under `hej.onboarding.*` (PRD 005 §8, Data/storage): the
  completion flag, and per-device dismissals.
- **Per-user**, from the BFF: profile confirmation, via `confirmation_required` on
  `GET /api/me/profile`. **Never stored client-side** (PRD 005 §6) — it must survive
  reinstalls, new devices and cleared site data, and a client-side copy would let a
  reinstall silently skip the step or a stale flag re-ask a member who already confirmed.

## Derived, not a persisted cursor

The current step is **derived** from state the app already holds — `session.store` (logged in
or not), `location.store.permission`, `notifications.store.permission`, and the BFF's
`confirmation_required` — rather than a step index written to storage as the user advances.

This is what makes it resumable in the honest sense: a user who kills the app mid-flow, or
loses the tab, or grants location in Settings instead of in the dialog, resumes at the first
step that is genuinely unsettled. A persisted cursor drifts out of step with reality and then
either re-asks for a permission the OS has already granted or skips one that was revoked —
and revocation happens in Settings, entirely outside the app's knowledge. Derived state is
self-healing; a cursor is a second source of truth for facts the platform already answers.

Note `location.store` already documents a WebKit trap worth respecting here: Safari answers
`prompt` for a *granted* geolocation permission. Read that store's resolved `permission`
value — do not re-query `navigator.permissions` in this store, or the step machine will
disagree with the map about whether location works.

## The sequence is data

PRD 005 §6 defines the canonical order:

1. `login` (mandatory — the only mandatory step)
2. `confirm profile` (spejder only, first run, skippable by rule)
3. `portrait`
4. *(slot)* `vehicle` — bandit/gøgler/crew only, **owned by PRD 010**
5. `location`
6. `notifications`
7. *(slot)* `offline first sync` — **owned by PRD 009**

Steps 4 and 7 are **flag-gated slots**. The machine must treat the sequence as a **data
structure** — an array of step descriptors with an `enabled`/`applies` predicate — not as
hardcoded `if`/`else` control flow. Two reasons: PRD 009 and PRD 010 are not approved, so
their slots must be absent today and addable without touching the machine's logic; and the
step count has already been a source of drift (§6 notes §5 and §6 previously disagreed on it),
which is exactly what happens when the sequence is implied by code rather than written down
once.

The same structure carries the applicability rules, so they stay next to the steps: `confirm
profile` is spejder-only and skipped when the user has already started the event or confirmed
before — and **skipping it must not skip the portrait step**, which runs for every user with
no portrait (PRD 005 §6; the two are independent facts).

Onboarding **never hard-blocks** on a declined permission or a failed confirmation. Only
login is mandatory (PRD 005 §6).

## Acceptance Criteria

- [ ] `vue/src/stores/onboarding.store.ts` exposes the current step, the ordered list of
      applicable steps (for the progress indicator) and a `complete` flag
- [ ] The step sequence is a **declarative array of descriptors**; adding PRD 009's or
      PRD 010's slot is a data change, not a control-flow change
- [ ] The `vehicle` and `offline first sync` slots are **absent** while their PRDs are
      unapproved, with a comment naming the owning PRD
- [ ] Step state is derived from `session.store`, `location.store.permission`,
      `notifications.store.permission` and `confirmation_required` — **no persisted step index**
- [ ] Killing the app mid-flow and reopening resumes at the first unsettled step; a permission
      granted or revoked outside the app is reflected without any in-app bookkeeping
- [ ] `location.store`'s resolved `permission` is read as-is; `navigator.permissions` is not
      re-queried here (WebKit reports `prompt` for granted geolocation)
- [ ] Per-device completion persists under `hej.onboarding.*`, with wrapped `localStorage`
      access so a blocked/throwing storage cannot break the router gate
- [ ] **No per-user confirmation state in `localStorage`** — `confirmation_required` is read
      from the BFF on every start
- [ ] Skipping `confirm profile` does not skip `portrait`
- [ ] Only `login` blocks progress; every other step can be left unsettled and onboarding
      still completes
- [ ] `npm run type-check` clean

## Depends on

- **Task 116** — `isStandalone()`, since the completion flag is only meaningful for a
  standalone launch.
- The BFF's `confirmation_required` on `GET /api/me/profile` (PRD 005 §8; a separate BFF task
  from §10's list). Until it exists, the store should treat the field as absent → step not
  applicable, so this task is not blocked on the Go work.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
