# 162 — Client freshness loop

**Status:** open
**Priority:** high
**Created:** 2026-08-31

## Description

Keeps the cached directory current during the event (PRD 007 §6, §8 "Keeping the
directory fresh"). Polls task 155's version endpoint at moments that already exist:

1. **On foreground** — `visibilitychange` → visible, and on app launch. The case that
   matters most: someone opening the pane wants it current.
2. **While open** — a ~60 s interval, stopped the moment the app is hidden. No
   background timers, no periodic background sync.
3. **On reconnect** — the `online` event, since changes may have been missed offline.

Only a changed version triggers a manifest delta, and only then are newly-referenced
images fetched. **Metadata propagates ahead of images**: a corrected number must not
wait on a portrait download, and a new person appears with a placeholder rather than not
at all.

The interval must be **runtime-configurable** (`vue/src/config/runtime.ts`) — this is
the app's first continuous during-race traffic and it shares the BFF with PRD 002's
position reporting, so it must be widenable without shipping a release.

Push is not usable here: iOS requires every web push to show a notification. See PRD 007
§8.

## Acceptance Criteria

- [ ] Version check on foreground, on `online`, and every ~60 s while visible.
- [ ] Polling stops entirely when the document is hidden.
- [ ] Interval read from runtime config, with a sane default.
- [ ] A changed version triggers a metadata delta; images follow separately.
- [ ] Backoff on repeated failure; no tight retry loop when offline.
- [ ] The list updates in place — scroll position preserved, expanded group not
      collapsed (PRD 007 §5 edge case).
- [ ] Tests with fake timers: hidden → no polling; foreground → immediate check;
      unchanged version → no delta fetch.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §6 / §8.
