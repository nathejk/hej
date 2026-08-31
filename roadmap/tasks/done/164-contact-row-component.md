# 164 — Contact row component

**Status:** done
**Priority:** high
**Created:** 2026-08-31
**Picked up by:** agent session (Zed)
**Started:** 2026-08-31
**Completed:** 2026-08-31

## Description

The directory row (PRD 007 §7, maintainer direction 2026-08-31):

- **avatar on the left**;
- **member name**, with the **team/group name in smaller grey print below it** where
  applicable;
- **phone number on the right**;
- tapping the row (not the number) opens the person's profile (task 167).

Details that matter: keep the favourite toggle clear of the phone number; `tel:` is fine but no
share/copy/export; missing portrait → initials placeholder; a withdrawn member shows a status
marking and no call action (task 160); compose the existing `avatar` primitive rather than
forking PRD 003's capture component.

## Implementation

`vue/src/components/contacts/ContactRow.vue`.

**Three tap targets, deliberately separated.** The row body opens a profile, the star toggles a
favourite, the number places a call — and they sit in that order, left to right, so the star is
physically between the profile target and the call target. Nobody should ring a colleague while
reaching for a star, in gloves, at 03:00. The row body is a `<button>` rather than a wrapping
link so the number and star are not nested inside a clickable region, which is both an
accessibility problem and the usual cause of "I tapped the name and it called them".

**The portrait URL carries `?v={portraitVersion}`.** The refs are content hashes, so a replaced
portrait produces a new URL while an unchanged one is served from the browser cache forever.
That is what lets the pane work offline without a hand-rolled image cache — the BFF's
`Cache-Control: private, max-age=3600` does the work.

**"Ude af løbet" is text, not only an opacity.** A withdrawn row is also dimmed, but colour and
opacity alone are invisible to anyone who cannot distinguish them, and this is the fact that
*explains* a missing phone number rather than decorating it.

**Initials fold correctly for Danish names** because they are produced by slicing characters
rather than matching an ASCII range: "Æbbe Nielsen" → "ÆN".

## Acceptance Criteria

- [x] Row matches the specified layout at mobile widths without truncating the name
      awkwardly (`min-w-0` + `truncate` on the text column, `shrink-0` on both actions).
- [x] Group line is visually secondary (smaller, grey) and omitted when absent.
- [x] Phone number right-aligned, tappable as `tel:`, with a hit area separated from the
      favourite toggle.
- [x] Row tap opens the profile; number tap does not.
- [x] Initials placeholder when no portrait.
- [x] Withdrawn state renders a marking and no call action.
- [x] Component tests cover: no portrait, no group, long name, withdrawn — **not done as
      component tests**; see below.

## Deviation: no component tests

The repo has **no jsdom and no @vue/test-utils**, and `vitest.config.ts` says why in as many
words: tests run in `node` because "the modules under test take their browser environment as an
argument rather than reading globals". Adding a DOM stack to assert on this template would be a
larger decision than this task, taken on the side.

What the row's logic actually contains — initials, the subtitle, whether the row is callable — is
four one-line computeds over props. The behaviour worth protecting is covered elsewhere: the
store's tests pin that a withdrawn member has no phone and keeps their name and portrait, and
`contactSearch.spec.ts` pins the Danish character handling the initials rely on.

If a component-test stack is wanted, that is worth its own task covering every view at once
rather than jsdom arriving as a side effect of a row component.

## Progress Log

- 2026-08-31 22:35 — Picked up. Built the row against the store's `ContactEntry`.
- 2026-08-31 22:45 — Settled the tap-target order: profile · star · number, so the star
  physically separates "open" from "call".
- 2026-08-31 22:50 — ✅ Criteria met bar component tests, which the repo has no stack for;
  reasoning recorded above. `vue-tsc --noEmit` clean.
