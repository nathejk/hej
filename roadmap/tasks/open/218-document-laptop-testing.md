# 218 — Document laptop testing

**Status:** open
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `README.md` section covering the presets, the off switch and the `/desktop.html`
      entry point
- [ ] The `?nogate=` distinction stated explicitly
- [ ] VAPID dev setup documented, with the secret-handling caveat
- [ ] `docker-compose.yml`'s existing VAPID comment cross-references it

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 5.
