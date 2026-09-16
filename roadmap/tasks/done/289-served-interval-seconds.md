# 289 — Served interval_seconds, and the zero-disables-the-interval test

**Status:** done
**Priority:** medium
**Created:** 2026-09-16
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-16
**Completed:** 2026-09-16

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

- [x] `interval_seconds` and the debounce window are server-configurable (env → config),
      defaulting to 60 s and 5 s. **Done in task 284** — they are part of that response's
      contract, so shipping the endpoint without them would have shipped a shape that
      immediately changed.
- [x] Both served in the `/api/sync` response. **Done in task 284.**
- [x] The loop adopts a changed interval on the next check without a reload — the timer
      is restarted, not left on the old period.
- [x] Zero disables the interval only: foreground, `online` and manual checks still fire.
- [x] Test (client): zero → no timer, but a foreground check still happens.
- [x] Test (client): interval changing from 60 to 300 restarts the timer at the new
      period.
- [x] Test (server): the configured values appear in the response.
- [x] Go gates and frontend type-check/unit suite green.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 1.
- 2026-09-16 12:40 — The **server half moved into task 284**: `syncIntervalSeconds` /
  `syncDebounceSeconds` (env `SYNC_INTERVAL_SECONDS` / `SYNC_DEBOUNCE_SECONDS`, defaults
  60/5) and their presence in the response shipped with the endpoint, with a test that
  zero is served *as zero* rather than corrected to a default. What remains here is the
  client half, which is the part with the interesting failure mode: adopting a changed
  interval **without a reload** (restarting the timer at the new period rather than
  leaving it on the old one) and proving zero disables the interval *only*.
- 2026-09-16 15:40 — Picked up. The client half also landed early, in task 286: the loop
  could not be wired at all without a way to adopt a value that changes while it runs, so
  `setIntervalSeconds` / `setDebounceSeconds` went onto `useFreshnessLoop` there. This task
  is therefore verification, and every criterion has a named test:
  - restart on change — `useFreshnessLoop.spec.ts` › "restarts the timer when the interval
    changes", and end-to-end in `useSyncLoop.spec.ts` › "adopts a changed interval without a
    reload";
  - zero disables the interval only — `useFreshnessLoop.spec.ts` › "drops the timer when the
    interval becomes zero, and restores it when it returns", plus `useSyncLoop.spec.ts` ›
    'treats a zero interval as "no timer", not "no checks"', which asserts foreground *and*
    reconnect still fire;
  - unchanged interval does not churn the timer — "does not restart the timer when the
    interval is unchanged". This one is not pedantry: every response carries the value, so a
    restart on each one would reset the period on every check and quietly make a long
    interval unreachable.
  - served debounce adopted — `useSyncLoop.spec.ts` › "adopts a served debounce", asserted at
    10 s elapsed so it can only pass if the served 30 s actually replaced the 5 s default;
  - server side — `TestSync_ServesTheIntervalAndDebounce`,
    `TestSync_ZeroIntervalIsServedAsZero`.
- 2026-09-16 15:45 — **Noted, not fixed here:** `contactsPollSeconds` in
  `vue/src/config/runtime.ts` is now dead — nothing has read it since task 288 removed the
  contacts loop. It is the client half of `/api/config`'s `contacts_poll_seconds`, which
  belongs to the endpoint task 292 retires, so it should go there in one piece rather than
  be half-removed now. Added to 292's criteria.
- 2026-09-16 15:50 — Completed. No new code: the work landed in 284 (server) and 286
  (client), and this task's value was checking that each criterion is actually pinned by a
  test rather than assumed. The PRD's duplication decision — `/api/config` stays separate,
  the sync response wins — is recorded in `env.go`'s doc comment and PRD 017 §11. Frontend
  701 tests + type-check green; all four Go gates green.
