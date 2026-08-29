# 124 — WelcomeView: onboarding shell

**Status:** open
**Priority:** high
**Created:** 2026-08-30
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 005 §7. Add `vue/src/views/WelcomeView.vue`, mounted at `/welcome` with route name
`welcome` and `meta: { public: true }`. It is the shell the whole onboarding flow renders
inside: it asks the onboarding store (task 118) which step is current, renders that step
component, and shows how far through the flow the user is.

The shell owns **no step logic of its own**. Which step comes next, which steps are
skipped for this user, and whether onboarding is complete are all the store's answers —
PRD 005 §8 makes the point that the step machine derives its state from `session.store`,
`location.store.permission` and `notifications.store.permission` so it is self-healing
rather than a persisted cursor. If the shell starts branching on user role or permission
state, that rule has been broken and a user who kills the app mid-flow will resume in the
wrong place.

The canonical order is PRD 005 §6: `login` → `confirm profile` → `portrait` →
*(vehicle slot, PRD 010)* → `location` → `notifications` → *(offline first sync slot,
PRD 009)*. Steps 4 and 7 are **flag-gated slots**: the sequence must be data the shell
iterates, not a hard-coded template with `v-if`s, so an unapproved PRD cannot change this
file. Progress must be computed from the steps that actually apply to *this* user —
a spejder-only confirmation step that a bandit skips must not leave them staring at
"trin 2 af 6" and never seeing step 2.

**Progress indicator:** shadcn-vue `Progress`, or a stepper built from `Separator`
(PRD 005 §7 permits either). `progress` is one of the primitives not yet generated in
`vue/src/components/ui/` — task 122 generates it.

**Chrome is hidden here.** `App.vue`'s `showShell` becomes
`isAuthenticated && onboardingComplete && !route.meta.public`, so the top bar and
`BottomNav` are both absent on `/welcome`. Task 131's sibling work and the App.vue shell
task own that expression; this task only relies on it.

**Do not reach for `fullBleed`.** It is tempting — the map uses it to lose the header —
but it only suppresses the header *inside* `showShell`, and `showShell` is already false
on onboarding routes. Setting it on `/welcome` would be a no-op that reads like the
mechanism keeping the chrome away, which is exactly the sort of decoration that survives
into a later refactor and then confuses whoever changes `showShell`.

Per PRD 005 §6 (Non-Functional), `/welcome` is part of the precached shell, and the
headline uses `font-nathejk` per `.rules`.

On completion the store marks onboarding complete and the shell redirects to `/maps`
(PRD 005 §5).

## Acceptance Criteria

- [ ] `vue/src/views/WelcomeView.vue` exists, routed at `/welcome` as `welcome` with
      `meta: { public: true }`
- [ ] The current step comes from the onboarding store; the view contains no step-order
      or eligibility logic of its own
- [ ] The step sequence is **data**, iterated by the shell, with the vehicle and
      offline-sync entries absent until PRD 010 / PRD 009 are approved
- [ ] Progress is rendered with shadcn-vue `Progress` (or a `Separator` stepper) and
      counts only the steps that apply to the current user
- [ ] Resuming after the app is killed mid-flow lands on the first unsettled step, not
      back at the beginning of the flow
- [ ] Top bar and `BottomNav` are absent on `/welcome`
- [ ] `fullBleed` is **not** set on the route, and a comment records why (no-op outside
      `showShell`)
- [ ] Completing the last applicable step marks onboarding complete and redirects to
      `/maps`
- [ ] `/welcome` is in the precached shell
- [ ] Page headline uses `font-nathejk`; all copy in Danish
- [ ] `vue-tsc` and `npm run build` clean

## Depends on

- **Task 118** — the onboarding store / step machine this shell renders.
- **Task 122** — the `progress` shadcn-vue primitive.
- **Task 116** — platform helper, indirectly, via the router gate that sends users here.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
