# 188 — Global "running on cached data" indicator in the app shell

**Status:** done
**Priority:** medium
**Created:** 2026-09-01
**Picked up by:** agent session (Zed)
**Started:** 2026-09-01
**Completed:** 2026-09-01

## Description

PRD 009 §6, §7. One indicator in the shell, so no feature invents its own and no screen has to
apologise separately for being offline.

Two states worth distinguishing, and only two — resist more:

- **offline** — the app is working from cache. Say so once, quietly, in the shell.
- **stale** — this *particular* screen's data may be old. That stays inline and per-feature: a
  timestamp, not a warning, because most staleness is harmless. The exception is where acting
  on old data is risky, and there is a live example: a phone number for a crew member who may
  have withdrawn (PRD 007).

`navigator.onLine` is the only signal available (`navigator.connection` is absent in Safari),
and it is a weak one — it reports the radio, not reachability. Prefer "our last request
failed / our last request succeeded" over trusting the flag alone; a phone with a bar of
signal and no working data connection is the normal case in a forest at night.

## Acceptance Criteria

- [x] One shell-level indicator, in Danish. *It already existed — `components/OfflineNotice.vue`
      from task 090. This task confirmed it as **the** one and stopped a second source of truth
      appearing; see the log.*
- [x] Derived from actual request outcomes as well as `navigator.onLine`, not the flag alone.
      *Already true via `app.store.online`, which `fetchWrapper`'s `NetworkError` corrects.*
- [x] Unobtrusive: one line, in the document flow, not an overlay.
- [x] Lucide `WifiOff`; no new icon set.
- [x] It links to the readiness view rather than explaining itself in place.
- [x] No duplicate global banners — asserted structurally by
      `components/offlineIndicator.spec.ts`, with the one justified exception documented.

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
- 2026-09-01 — **Most of this was already shipped.** `OfflineNotice.vue` (task 090) is a
  one-line shell-level notice driven by `app.store.online`, which is seeded from
  `navigator.onLine` and then corrected by real request failures. Building a second indicator to
  satisfy a PRD written before it existed would have been the exact duplication PRD 009 exists to
  prevent.
- 2026-09-01 — **Deleted `servingFromCache` from `offline.store` instead of wiring it.** It was a
  second copy of connectivity, and `app.store.online` owns it better — `onLine` alone means only
  "there is an interface with a route", which is true on a captive portal and true with one
  useless bar. Two copies would have disagreed in precisely the situation both exist for.
- 2026-09-01 — Made the notice a link to `/profil`: what a user wants when they see it is "so
  what *do* I still have?", and that is now a whole section there. Reworded to
  "Ingen forbindelse — se hvad du har hentet", still one line — the file's own comment records
  that an earlier three-line version was honest and unusable.
- 2026-09-01 — **Did not remove the per-feature messages, deliberately.** The criterion said to
  audit them, and the audit says keep: `ContactsView`'s stale marker is PRD 009 §7's inline
  honesty about *data*, and `PatrolLookup`'s "needs signal, use the radio" is required by PRD 007
  — that lookup is deliberately live-only, so a generic "the app still works offline" would be a
  lie about the one thing the crew member is trying to do. `WelcomeStepLogin` keeps its own
  because the global notice cannot render there at all (no session yet) and login genuinely
  needs the network. All three recorded in the spec's allowlist with reasons, so the next author
  meets the decision rather than the rule.
- 2026-09-01 — ✅ All criteria complete. 3 structural tests; suite 280 across 22 files;
  `type-check` and `build` clean.
