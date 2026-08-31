# 165 — Local search across the directory

**Status:** done
**Priority:** medium
**Created:** 2026-08-31
**Picked up by:** agent session (Zed)
**Started:** 2026-08-31
**Completed:** 2026-08-31

## Description

A sticky search field at the top of the pane, spanning **every person the caller may list**
(PRD 007 §6). Matches on name, group (klan / section) and arm number. **Runs locally** against
the synced index. Spejdere are never in the index; favourites rank first.

## Implementation

`vue/src/helpers/contactSearch.ts` + `contactSearch.spec.ts` (22 tests); the field itself is in
`ContactsView.vue`.

**Extracted as a pure function rather than left in the view.** The repo has no jsdom and no
component-test stack, by the explicit choice recorded in `vitest.config.ts`, so logic worth
testing has to be reachable without mounting anything. This is the same split as
`config/nudge.ts`: the rules live apart from the component that renders their result.

**Danish folding is the feature, not a nicety.** `foldForSearch` lower-cases, strips combining
diacritics via NFD, and maps ø→o, æ→ae, å→a explicitly, because those three do not decompose the
way é and ü do. So "soren" finds "Søren" and "aerlige" finds "Ærlige" — which is how people type
when a keyboard makes the real letter awkward. Sorting uses `localeCompare(…, 'da')`, tested with
Æ and Ø, which sort *after* Z in Danish rather than near A and O as a codepoint sort would have it.

**Spejdere are excluded structurally, not by a filter.** The index contains none, because the BFF
never lists one. There is nothing here to get wrong, which is the point.

**The patrol lookup is deliberately not merged into this field.** A patrol is asked for through
its own explicit action (task 168). One "smart" field that also accepts patrol numbers would make
them browsable by accident — the exact thing this design exists to avoid.

## A design flaw the tests found

My first version folded the phone number into the same haystack as the name. A test asserting
that a two-digit query is *not* a number search failed — `"30"` matched every person in the
fixture, because every Danish mobile contains it. So typing the first two digits of a number
would have shown the entire directory.

Split into two haystacks: text (name, crew function, group labels) matches on any substring, while
the number matches only once the query carries **at least three digits**. "Who was that missed
call from?" is a real question during an event; nobody asks it two digits at a time.

Arm number is not in the matched fields, because the manifest does not carry one — noted for
whoever adds it; the search will pick it up by adding one line to `textOf`.

## Acceptance Criteria

- [x] Search field sticky at the top, reachable one-handed while scrolling.
- [x] Matches name, group and arm number, case- and diacritic-insensitively (Danish names:
      æ/ø/å behave) — **arm number is not in the manifest**, so it is not matched; the field
      list is one line to extend when it is.
- [x] Runs entirely offline against the synced index.
- [x] No spejder can ever appear in results — structurally, since the index contains none;
      asserted server-side in `contacts_test.go`.
- [x] Favourites ranked first.
- [x] Empty state distinguishes "no matches" from "nothing synced yet".
- [x] Results update without losing input focus (the field is outside the results container, so
      re-rendering results cannot remount it).

## Progress Log

- 2026-09-01 00:00 — Picked up alongside task 163, since the field lives in that view.
- 2026-09-01 00:15 — Extracted the matching rules into a helper so they can be tested without a
  DOM, following the `config/nudge.ts` precedent.
- 2026-09-01 00:20 — Test failure exposed a real flaw: phone digits in the shared haystack meant a
  two-character query matched everyone. Split text and number matching, with a three-digit floor
  for numbers.
- 2026-09-01 00:25 — ✅ Criteria met, with arm number noted as absent from the manifest rather
  than silently skipped. 22 search tests pass; `vue-tsc --noEmit` clean.
