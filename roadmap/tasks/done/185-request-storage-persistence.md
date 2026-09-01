# 185 — Request storage persistence at onboarding, and report the answer

**Status:** done
**Priority:** high
**Created:** 2026-09-01
**Picked up by:** agent session (Zed)
**Started:** 2026-09-01
**Completed:** 2026-09-01

## Description

PRD 009 §6. Everything this app caches is **best-effort and evictable by default**.
`navigator.storage.persist()` is what asks the browser to stop treating it that way, and it is
a **per-origin** request — so it belongs to the app at install/onboarding, not to whichever
feature happens to cache first.

`helpers/trackDb.ts` already calls it for its own store. That is not wrong, but it means the
answer depends on which feature ran first. Lift the request to onboarding (PRD 005 step 6,
task 194) and record the result in `offline.store` (task 184).

**The request should actually succeed here.** WebKit grants persistence "based on heuristics
like whether the website is opened as a Home Screen Web App", and this app's onboarding is
install-first by design (PRD 005). Same decision also exempts us from Safari's seven-day
inactivity eviction. So this is a cheap call with a real payoff — but it is a *request*, and a
denial must be shown honestly in the readiness view rather than swallowed.

## Acceptance Criteria

- [x] `persist()` is requested once at onboarding, not per feature, and the result stored.
      *At app mount rather than only in the welcome flow — see the log; it also repairs devices
      onboarded before this shipped.*
- [x] `trackDb.ts`'s own call is reconciled — it now delegates to the shared helper, so there is
      one code path and one answer.
- [x] Guarded for absence: the API is missing in some engines, and a failure must not break
      onboarding.
- [x] A denial is surfaced as a real state: `offline.store.evictable` is true **only** for
      'denied', so 'unsupported' cannot produce a warning. *The Danish sentence itself renders in
      task 187, which owns that view; the state and the rule about when to say anything are
      settled here.*
- [x] Tested: granted, denied, and API-absent — plus already-granted and throwing.

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
- 2026-09-01 — Picked up. Plan: lift the request out of trackDb into helpers/offline/, three outcomes instead of a boolean, one idempotent call at app level.
- 2026-09-01 — **Three outcomes, not a boolean.** `granted / denied / unsupported`. The reason is
  copy, not tidiness: only a *denial* is worth a sentence to the user, and a boolean would have
  us warn everyone on a browser that never implemented persistence about a decision nobody made.
- 2026-09-01 — **`refreshStorage` only ever upgrades the answer.** `persisted()` returning false
  cannot distinguish "asked and refused" from "not asked yet", so allowing it to write 'denied'
  would invent a refusal and put a warning in front of the user for it. It can promote 'unknown'
  to 'granted' and nothing else. Has its own test.
- 2026-09-01 — **Called from `App.vue` on mount, not from the welcome flow.** PRD 009 says "at
  install/onboarding", and for a new install the first mount *is* onboarding — but a
  once-per-install hook would leave every device that onboarded before this shipped permanently
  evictable. The helper short-circuits when already granted, so repeating it costs nothing.
- 2026-09-01 — `trackDb.requestPersistentStorage` kept as a named function but now delegates.
  It has a genuine reason to care that the others do not — an evicted track cannot be re-fetched
  from anywhere — so the call site stays; only the duplicated implementation went.
- 2026-09-01 — ✅ All criteria complete. 6 new helper tests, 4 new store tests, one rewritten;
  full suite 240 passing across 19 files; `type-check` clean.
