# 218 — Document laptop testing

**Status:** done
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

## Description

PRD 014, phase 5. The layer is worthless if nobody knows the URL. Add a short
"testing on the laptop" section to `README.md`:

- The `?dev=` presets and what each one produces, including that `?dev=` must be
  typed onto `/desktop.html` on a first visit, because an unsimulated laptop is sent
  there before the app boots.
- `?dev=off` to stop simulating, and the panel's always-visible marker.
- How this differs from `?nogate=1` — the bypass disables the onboarding redirect and
  is therefore *not* the way to test onboarding. This distinction is the reason PRD
  014 exists and it should be stated where a developer will read it.
- `VAPID_PUBLIC_KEY` / `VAPID_PRIVATE_KEY` in `docker-compose.override.yml`, since
  desktop Chrome does real Web Push and the notification flow is fully testable on a
  laptop once they are set. Note the private key never gets committed.
- A pointer to the mobile-only checklist (task 219).

## Acceptance Criteria

- [x] `README.md` section covering the presets, the off switch and the `/desktop.html`
      entry point
- [x] The `?nogate=` distinction stated explicitly, under its own heading
- [x] VAPID dev setup documented, with the secret-handling caveat
- [x] `docker-compose.yml`'s existing VAPID comment cross-references it
- [x] The existing "Logging in locally" section now points at the dev panel's PIN readout,
      since that is the faster path and the log-grep instructions were the first thing this
      layer made obsolete

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 5.
- 2026-09-12 10:15 — Written as "Testing on this laptop (`?dev=`)" inside the existing dev-stack
  narrative rather than as an appendix, and placed next to "Logging in locally" — which it also
  amends, since the PIN readout supersedes grepping the API log.
- 2026-09-12 10:16 — Gave the `?dev=` vs `?nogate=1` distinction its **own heading**. It is the
  single most confusable thing in this PRD (both look like "let me past the gate"), and burying it
  in prose would guarantee someone tests onboarding with the flag that disables onboarding.
- 2026-09-12 10:17 — Documented the `/desktop.html?dev=iphone` entry point explicitly. Without it
  the feature is undiscoverable on a first visit: the laptop never loads the app, so there is no
  URL to append to.
- 2026-09-12 10:17 — VAPID documented as something worth *doing* rather than a footnote — desktop
  Chrome does real Web Push, so it makes the whole notification flow laptop-testable; only iOS needs
  hardware. Cross-referenced from `docker-compose.yml`'s VAPID comment.
