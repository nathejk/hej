# 120 — InstallInstructions: per-platform add-to-home-screen steps

**Status:** open
**Priority:** high
**Created:** 2026-08-30
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 005 §6/§7. Add `vue/src/components/onboarding/InstallInstructions.vue` — the fallback
half of the install wall (task 119), shown whenever there is no `beforeinstallprompt` to
offer. It takes the platform from `installPlatform()` (task 116) and renders the right set of
steps.

Three variants, and they are genuinely different situations rather than three wordings of one:

**`ios-safari`** — Share → *Tilføj til hjemmeskærm* → *Tilføj*. iOS fires **no
`beforeinstallprompt`**, so one-tap install does not exist here, and it gives **no
install-accepted event** either — meaning this variant cannot switch itself to a success state
when the user succeeds. The steps are the whole interaction; the confirmation is the user
opening the app from the home screen (task 119's "jeg har allerede installeret" affordance).
Design the copy accordingly: it must read as complete on its own, not as a screen waiting for
something to happen. Name the Share control by what the user sees, and note it is in Safari's
toolbar — on iPhone that is the bottom bar, which is not where people look for it.

**`other`** (Android, non-Chrome — e.g. Firefox) — the browser menu carries an
add-to-home-screen or install item, but the label and its position vary by browser and
version. Do not claim an exact menu path we cannot guarantee; describe it as the browser menu
item for installing/adding to the home screen, and offer opening the site in Chrome as the
reliable route.

**`webview`** (Facebook/Instagram in-app browsers, and friends) — **installing is impossible
here.** There is no add-to-home-screen and no install prompt in an embedded webview, so
instructions of the usual shape are actively misleading. The only correct advice is to leave
the webview: open the link in Safari (iOS) or Chrome (Android), typically via the webview's
"…" menu. This is the variant most likely to be hit in practice, because event links get
shared in Facebook groups — treat it as a first-class case, not an afterthought.

## Accessibility is a requirement, not a nicety

PRD 005 §6 (Non-Functional): the instructions must be readable **as text**, not only as an
image, and the flow must be operable **without gestures.**

So: an ordered list of real text steps, with the Lucide icons (`Share`, `PlusSquare`) as
*inline illustration alongside* the words — never as the sole carrier of a step. If a
decorative platform illustration is added (the one place PRD 005 §7 permits a hand-rolled
component), it is `aria-hidden` and the text stands alone with it removed. Nothing here may
require a swipe, a long-press or a drag to read or complete.

Screenshots are deliberately avoided: they are unreadable to a screen reader, they go stale
with every OS release, and they are the usual way "add to home screen" instructions rot.

## UI

shadcn-vue where a primitive fits — `Alert` (already generated) suits the webview variant,
which is a warning rather than a set of steps. Lucide icons only. All copy in Danish, matching
the existing views. Any headline uses `font-nathejk`.

## Acceptance Criteria

- [ ] `vue/src/components/onboarding/InstallInstructions.vue` renders from a `platform` prop
      typed as `installPlatform()`'s return union
- [ ] `ios-safari` variant: Share → Tilføj til hjemmeskærm → Tilføj, worded as a
      self-contained instruction that does not imply a pending state change
- [ ] A comment records that iOS fires neither `beforeinstallprompt` nor an install-accepted
      event, so this variant cannot confirm success
- [ ] `other` variant: browser-menu install/add-to-home-screen, described without inventing an
      exact menu path, plus "open in Chrome" as the reliable route
- [ ] `webview` variant: states that installing is not possible in an in-app browser and tells
      the user to open the link in Safari/Chrome — no add-to-home-screen steps shown
- [ ] Every step is **text**; icons and any illustration are supplementary and `aria-hidden`
- [ ] Steps are an ordered list, readable and completable without any gesture
- [ ] No screenshots or bitmap-only instructions
- [ ] shadcn-vue primitives where one fits (`Alert` for the webview case), Lucide icons only,
      copy in Danish
- [ ] `npm run type-check` clean

## Depends on

- **Task 116** — `installPlatform()` supplies the variant, including the `webview` case.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
