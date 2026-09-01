# 188 — Global "running on cached data" indicator in the app shell

**Status:** open
**Priority:** medium
**Created:** 2026-09-01

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

- [ ] One shell-level indicator, driven by `offline.store` (task 184), in Danish.
- [ ] Derived from actual request outcomes as well as `navigator.onLine`, not the flag alone.
- [ ] Unobtrusive: it must not cover content or shift layout on every reconnect flap — debounce
      the transition back to online.
- [ ] Lucide `WifiOff`; no new icon set (`.rules`).
- [ ] It links to the readiness view (task 187) rather than explaining itself in place.
- [ ] No feature-level "you are offline" banners remain once this lands; audit and remove.

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
