# 185 — Request storage persistence at onboarding, and report the answer

**Status:** open
**Priority:** high
**Created:** 2026-09-01

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

- [ ] `persist()` is requested once at onboarding, not per feature, and the result stored.
- [ ] `trackDb.ts`'s own call is reconciled — either delegating here or made idempotent — so
      there is one answer, not two.
- [ ] Guarded for absence: the API is missing in some engines, and a failure must not break
      onboarding.
- [ ] A denial is surfaced in the readiness view as a real state ("data may be removed by the
      phone"), in Danish, without alarming a user who cannot act on it.
- [ ] Tested: granted, denied, and API-absent.

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
