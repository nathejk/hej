# 320 — Nav slot ordering with Glimt added

**Status:** done
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:** agent
**Started:** 2026-09-17
**Completed:** 2026-09-17

## Description

PRD 019 §7, §11 Q7. `BottomNav` has `MAX_SLOTS = 5` and already overflows into `MoreMenu` for
most roles. Glimt is a **primary** destination for spejdere and must be in the bottom bar for
them — which forces a decision about what moves to "Mere".

A spejder currently sees: maps, rulebook, updates, schedule, faq, privacy (no contacts pane).
Adding Glimt makes seven for five slots.

This task is the decision plus the implementation: pick the per-role ordering, put it in
`vue/src/config/navigation.ts`, and make sure nothing a role needs becomes unreachable.

The Team-section moderation entry (task 309) must not consume a participant slot.

## Acceptance Criteria

- [x] Per-role ordering decided and recorded in a comment explaining *why*, not just what
- [x] Glimt in the bottom bar for `spejder` and `bandit`
- [x] Nothing becomes unreachable — everything demoted is in `MoreMenu`
- [x] Moderation entry does not take a participant slot
- [x] Existing `BottomNav` / `MoreMenu` tests updated
      (there were none; the rule is now extracted and covered — see the log)
- [x] `npm run test:unit` and `npm run build` pass

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
- 2026-09-17 — **Decision: no per-role ordering.** The task is phrased as "pick the per-role
  ordering", and the answer after working it through is that per-role ordering is the wrong
  mechanism. `navigation.ts` is *one* ordered array that `visibleDestinations` filters, and
  role-gating already does the work — a destination a role cannot see costs it no slot. So placing
  `glimt` above `updates` puts it in the bar for **all seven roles** simultaneously:

  | role | bar |
  |---|---|
  | spejder | Kort · Regler · Glimt · Nyt + Mere |
  | bandit, gøgler | Kort · Kontakter · Regler · Glimt + Mere |
  | crew, samarit, guide, postmandskab | Kort · Kontakter · Regler · Glimt + Mere |

  A spejder gets Glimt in third position rather than fourth only because they have no `contacts`
  entry (PRD 007) — the same list, filtered. Branching the order per role would have bought
  nothing and given us seven orderings to keep correct instead of one.

  **Nothing was demoted.** `schedule`, `faq`, `privacy` and `sos` were all already behind "Mere"
  for every role that sees them, before Glimt existed. This answers PRD 019 §11 Q7: the honest
  answer is *nothing had to move*, which is why the question looked harder than it was.

- 2026-09-17 — **`sos` deliberately left in the overflow.** For samarit/guide/postmandskab the
  emergency page is two taps behind the burger. That predates this task (task 011) and Glimt did
  not cause it, but it is the one placement in the list that looks wrong, so it should not go
  unremarked: whether a medic should reach SOS in one tap is an operational question for someone
  who knows the response chain, and it is not something to decide as a side effect of adding a
  photo feature. Recorded in `navigation.ts` with the cost of changing it — moving `sos` up is free
  for the roles that cannot see it, but it takes the Glimt slot from the three that can. **Worth
  putting to the maintainer separately.**

- 2026-09-17 — **Extracted the rule so it can be tested.** The bar/overflow split lived inside
  `BottomNav.vue` as two `slice` calls, and Vitest here runs in node with no DOM — so the ordering
  requirement PRD 019 imposes was, until now, unassertable. Moved to
  `vue/src/config/navSlots.ts` (`MAX_SLOTS`, `splitNavSlots`, `inBar`) with
  `vue/src/config/navSlots.spec.ts`.

  The tests assert **outcomes, not the array's contents** — "a spejder has Glimt in the bar",
  "every visible destination is in exactly one of the two lists for every role" — so a future
  re-ordering that preserves the requirements is free and one that breaks them fails. That matters
  because the bar a role sees is an *emergent* property of list order plus role gates: nobody can
  read `navigation.ts` and be certain, least of all whoever adds the eleventh destination.

  Two of the nine tests are guards rather than requirements: the off-by-one in
  `MAX_SLOTS - 1` (the burger occupies a slot, so getting this wrong drops a destination from
  *both* lists — a page with no way in), and the absence of a `glimt-moderation` destination, so
  task 309 cannot spend a participant slot on organizer tooling without tripping over it.

  Note the acceptance criterion "existing `BottomNav`/`MoreMenu` tests updated" could not be met
  as written: **there were no such tests.** Components are never mounted in this suite. The
  equivalent coverage now exists at the level the suite can actually reach.

  846 tests passing, `type-check` and `build` clean.
