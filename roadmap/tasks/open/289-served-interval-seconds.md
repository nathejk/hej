# 289 — Served interval_seconds, and the zero-disables-the-interval test

**Status:** open
**Priority:** medium
**Created:** 2026-09-16
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 017 phase 1. The interval and debounce are **operational levers, not constants**:
if a few hundred devices cost more than expected at 02:00 during an event, the interval
has to be widenable without shipping a release.

`interval_seconds` is served **in the `/api/sync` response**, not only in
`/api/config`. That is deliberate: an operator shedding load wants it to take effect on
the next check, on every device, without waiting for a config refetch. `/api/config`
stays separate because its contract is that everything it carries is public by
definition, while `/api/sync` is per-caller — the sync response is the one that wins.

Values decided in PRD 017 §11: **interval 60 s, debounce 5 s**. The interval is set by
the strictest dataset (scans should surface inside a minute); contacts, portraits and
profile need only "within a few minutes" and get better than that for free, because one
multiplexed check covers them all at the same cost.

**Zero disables the interval and nothing else.** Foreground, reconnect and manual checks
keep running. This is the distinction an operator would get wrong — "poll less" must
never silently become "stop updating" — so it is the one with its own test. The
behaviour already exists in `useFreshnessLoop`; what is new is that the value arrives
per-response and can change *while the loop is running*.

## Acceptance Criteria

- [ ] `interval_seconds` and the debounce window are server-configurable (env → config),
      defaulting to 60 s and 5 s.
- [ ] Both served in the `/api/sync` response.
- [ ] The loop adopts a changed interval on the next check without a reload — the timer
      is restarted, not left on the old period.
- [ ] Zero disables the interval only: foreground, `online` and manual checks still fire.
- [ ] Test (client): zero → no timer, but a foreground check still happens.
- [ ] Test (client): interval changing from 60 to 300 restarts the timer at the new
      period.
- [ ] Test (server): the configured values appear in the response.
- [ ] Go gates and frontend type-check/unit suite green.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 1.
