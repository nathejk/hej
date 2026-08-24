# 026 — Confirm the browser baseline implied by Tailwind v4

**Status:** done
**Priority:** high
**Created:** 2026-08-24
**Picked up by:** agent (opus-5) + product owner
**Started:** 2026-08-24
**Completed:** 2026-08-24

## Description

Tailwind v4 drops support for older engines: it requires roughly **Safari 16.4+**
(iOS 16.4, spring 2023) and **Chrome 111+**, because it relies on modern CSS
(`@property`, `color-mix()`, cascade layers).

"Hej Nathejk" is an in-event PWA used on whatever phone a participant happens to
own, including hand-me-down devices that may never see another iOS update. This
is the one open question that could invalidate the Tailwind half of PRD 004, so it
needs an explicit, recorded answer rather than an assumption.

Deliverable is a decision, not code:

- State the supported baseline and write it down (`.rules` and/or the
  `vue3-pwa-layout` skill).
- If the baseline is judged too aggressive, escalate: PRD 004 goes back to
  `draft/` for the Tailwind portion and the shadcn migration proceeds on
  Tailwind 3.
- Decide what an unsupported browser should experience — a degraded but usable
  page, or an explicit "your phone is too old, use X" message. Silent CSS
  breakage on event night is the outcome to avoid.

Useful input: any analytics/user-agent data we have, and the age profile of
participant devices (many participants are minors using older hand-me-downs).

PRD: 004. Related: 024. Should ideally be answered **before** 024 is merged.

## Acceptance Criteria

- [x] The supported browser/OS baseline is decided and documented in `.rules`
      and/or `.agents/skills/vue3-pwa-layout/SKILL.md`.
- [x] The decision is justified with whatever device evidence is available (or the
      absence of evidence is stated).
- [x] The behaviour on an unsupported browser is decided (degrade vs. notify).
- [x] If the baseline is unacceptable, PRD 004 is updated/reopened accordingly and
      tasks 024/025 are adjusted.

## Decision

**Baseline: iOS/iPadOS Safari 16.4+ and Chrome 111+ (Android equivalent).**
Accepted — no rollback to Tailwind 3 needed.

## Progress Log

- 2026-08-24 00:00 — Task created from PRD 004.
- 2026-08-24 01:05 — Decided by the product owner: the baseline is a non-issue,
  because it is **already implied by the product**, not imposed by Tailwind.
  The decisive detail: iOS shipped **Web Push for home-screen web apps in exactly
  iOS/Iphone Safari 16.4** — the same floor Tailwind v4 requires. Since push
  notifications, service workers, installability and geolocation are core to this
  app (PRD 001), a user on Safari <16.4 cannot use its primary features whatever
  we do about CSS. Tailwind v4 therefore costs us **zero reach**.
- 2026-08-24 01:07 — Consequence recorded: no polyfills or fallbacks for older
  engines, and modern CSS (`@property`, `color-mix()`, cascade layers, `:has()`)
  is fair game. Unsupported browsers are not given a special notice — they already
  fail at the push/install step, which is where the product conversation belongs.
- 2026-08-24 01:08 — Documented as a **Browser baseline** bullet in `.rules` and a
  "Browser baseline" section in the `vue3-pwa-layout` skill, in both cases stating
  the iOS 16.4 Web Push rationale so nobody later mistakes it for an arbitrary
  tooling constraint.
- 2026-08-24 01:09 — PRD 004 needs no change: the Tailwind half stands as written.
  Completed.
