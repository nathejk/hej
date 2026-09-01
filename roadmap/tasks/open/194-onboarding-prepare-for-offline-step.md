# 194 — "Forbered til offline" step in onboarding

**Status:** open
**Priority:** medium
**Created:** 2026-09-01

## Description

PRD 009 §7 and PRD 005 §5 step 6. PRD 005 reserved a **flag-gated slot** for this step (step 7
of its machine, absent until PRD 009 was approved) — 009 is now approved, so the slot gets
filled. The step machine treats the sequence as data, so this is a registration, not a rewrite
of the flow.

Why onboarding: it is the one moment the user is reliably on wifi, at home, with the app open
and expecting setup steps. Everything after that is worse.

**It is skippable.** PRD 005 allows only login to be mandatory, and skipping is a *reasonable*
choice for someone on cellular facing a ~324 MB estimate — so the step must not shame or block.
It also means a user can reach the race with nothing cached, which is what task 187's readiness
banner and PRD 009 §11.6 are about.

**Two tiers, one screen.** The cheap datasets (directory, rulebook, contacts, schedule, z12–14
tiles ≈ 56 MB) can be offered as the default; the expensive z15–16 tiles (≈ 268 MB) need an
explicit opt-in with the size stated. The app cannot tell WiFi from cellular on iOS
(`navigator.connection` is unavailable in Safari), so the number in front of the user is the
consent mechanism — there is no automatic "only on WiFi" to fall back on.

Also the natural place to request storage persistence (task 185).

## Acceptance Criteria

- [ ] The step registers into PRD 005's existing slot; the sequence stays data-driven.
- [ ] Skippable, with no dark patterns — a plain "Spring over".
- [ ] A combined size estimate is shown **before** anything downloads, split by tier.
- [ ] Progress is visible per tier and in aggregate; 5,291 tiles over rural mobile data is
      minutes, and a silent multi-minute download is indistinguishable from a hang.
- [ ] Interrupting it (backgrounding, losing signal, closing the app) **resumes** rather than
      restarts, and never reports a half-synced dataset as complete.
- [ ] Cancelling keeps what was already fetched.
- [ ] `persist()` requested here (task 185).
- [ ] Danish copy; headings in `font-nathejk`.

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
