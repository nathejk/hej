# 316 — GlimtView feed, GlimtCard, GlimtMediaStrip

**Status:** done
**Priority:** high
**Created:** 2026-09-17
**Picked up by:** agent session (Zed)
**Started:** 2026-09-18
**Completed:** 2026-09-18

## Description

PRD 019 §7. The feed: chronological, newest first, thumbnail-first.

**`GlimtCard`** shows the **hold attribution** ("Patrulje 42 · Ørnene · Spejder", "Din
patrulje" on your own) — no name, no portrait, so there is **no avatar slot in this design and
no reason to fetch one**. Then relative time, an audience chip ("Min gruppe" / "Alle" /
"Offentligt"), the media, the caption, and an overflow menu: *Slet* for your own, *Anmeld* for
everyone else's.

The attribution is **tappable and opens that hold's collection** (task 319) — the main way
anyone gets into the post-race browse.

`Anmeld` sits in the one-tap overflow on **every** card, not behind a long-press: with an
unmoderated public scope, reporting is the safety mechanism (PRD 019 §0) and must be as easy as
posting.

Use shadcn-vue `card` and `dropdown-menu`. The swipe strip has no shadcn equivalent in
`vue/src/components/ui/` — **add `carousel` from the catalogue** rather than hand-rolling
(`.rules`). Headlines use `font-nathejk`; icons are Lucide.

Accessibility: a list of articles, not a gesture-only tape. Captions are real text.

## Acceptance Criteria

- [x] `/glimt` route and destination in `config/navigation.ts` + `router/index.ts`
- [x] `GlimtView.vue` renders the feed from the task 313 store, lazily
- [x] `GlimtCard.vue` with hold attribution, no name and no avatar anywhere
- [x] Attribution navigates to the hold collection (guarded until task 326 registers the route)
- [x] `Anmeld` on every card in one tap; `Slet` only on your own
- [x] shadcn `carousel` added and used for multi-item media
- [x] Empty state is the invitation to post
- [x] `npm run test:unit` (784 tests), `type-check` and `build` all pass

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
- 2026-09-18 — Picked up.
- 2026-09-18 — ⚠️ **The shadcn CLI tried to do two things I did not ask for.** It prompted to
  overwrite `ui/button/`, which the skill warns carries our `LOCAL DEVIATION` touch-target changes —
  accepting would have silently undone them. It also bumped `@lucide/vue` from ^1.37.0 to ^1.47.0 as
  a side effect. Re-ran answering no to the overwrite, then reverted the lucide bump by hand and
  regenerated the lockfile, so the only dependency change is `embla-carousel-vue` (which the
  carousel needs). Verified `git diff` on `ui/button/` is empty.
- 2026-09-18 — Extracted `glimtPresentation.ts` before writing any markup, per the skill: the suite
  runs in `node` with no DOM and there is no `@vue/test-utils`, so a decision left in a template is
  a decision that cannot be tested. Everything in it turned out to be a **privacy rule wearing the
  clothes of a formatting function** — the attribution line is what the card shows *instead of* a
  name, the audience chip is how a member learns how far their photo went, and the action list is
  what decides whether reporting is one tap away. 21 tests.
- 2026-09-18 — Design consequence worth recording: because a glimt is attributed to its hold, there
  is **no avatar slot in this design and no portrait to fetch**. That is not a saving I planned for;
  it falls out of task 302's decision, and on a feed of a hundred cards it is a hundred fewer
  requests than the contacts pane makes.
- 2026-09-18 — The attribution is a **button, not a RouterLink**, so the parent owns navigation: the
  feed pushes a route, the moderation view (task 309) will not navigate at all. Renders as plain
  text when the hold has no number — crew, or an unnumbered hold — rather than as a link that 404s.
- 2026-09-18 — `GlimtMediaStrip` skips the carousel entirely for a single item. A one-item strip
  would add a scroll container, a swipe gesture and two ARIA roles for a photograph that cannot be
  swiped anywhere.
- 2026-09-18 — Dots rather than arrows: arrows are a desktop affordance, and on a phone the gesture
  *is* the control. They are `aria-hidden` because the count is already in each image's alt text
  ("billede 2 af 4") and announcing it twice is noise.
- 2026-09-18 — Aspect ratio is fixed from the stored dimensions so the layout does not jump as
  images arrive. On a slow link that is the difference between scrolling and chasing — and it is why
  task 301 stores width/height at all.
- 2026-09-18 — Went slightly beyond the criteria: the overflow menu's actions are **wired**, with a
  confirmation dialog and two new store actions (`remove`, `report`). A dropdown item that does
  nothing is worse than one that is absent, and both actions are consequential enough to deserve a
  second tap — a delete cannot be undone by its author, and a report takes somebody else's
  photograph off the public feed immediately. A reported glimt is removed from the local copy,
  because the server has hidden it from every audience the reporter belongs to; leaving it on screen
  would have the member watch the thing they objected to stay put.
- 2026-09-18 — Route entry points for the composer (317) and hold view (326) are behind
  `router.hasRoute(...)` checks. `router.push({ name })` **throws** on an unregistered name, so
  without the guard this view would break in its own commit and then silently start working two
  tasks later. The buttons appear when those routes land, with no edit here.
- 2026-09-18 — Nav placement: put above `updates`, which lands Glimt **in the bottom bar for every
  role** — position 3 for spejdere (who have no `contacts` entry) and 4 for everyone else, since
  `BottomNav` shows the first four plus "Mere". No `roles` list, deliberately: spejdere are the
  primary audience, so this is the one destination they get that `contacts` denies them. The
  per-role ordering decision proper is task 320's.
- 2026-09-18 — ✅ All criteria met. 21 new tests (784 total across 61 files), `type-check` and
  `build` clean. Moving to done.

### ⚠️ Not verified, and cannot be from here

The suite runs in `node` and mounts nothing, so **nothing about how this looks or feels has been
checked**. Specifically worth a look on a phone viewport before this is called finished:

- whether the carousel's swipe feels right, and whether the dots are visible against a bright photo
- the 44px overflow-menu target with a thumb, in the dark
- safe areas: the feed sits inside App.vue's scroll container, so it should be fine, but the compose
  button in the header has not been seen next to a notch
- whether `font-nathejk` at `text-2xl` reads as a page title or as shouting
