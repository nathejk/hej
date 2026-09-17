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

- 2026-09-18 (later) — 🐞 **Bug reported from a device, fixed.** "The box surrounding the image is way
  too high." A `GlimtMediaStrip` with mixed orientations rendered a container far taller than the
  visible photograph, with a screen of empty card underneath.

  Cause: **a carousel is a flex row, and a row is as tall as its tallest child.** Each slide carried
  its own `aspect-ratio`, so hold 45 — a 1600×900 landscape followed by a 900×1600 portrait — got a
  portrait-tall box with the landscape floating at the top. Measured against the real fixture data:

  | hold | items | first item | card height before | after |
  |---|---|---|---|---|
  | 45 | 2 | 1600×900 | **693px** | 219px |
  | 43 | 2 | 1600×1200 | 520px | 292px |
  | 44 | 1 | 1400×1400 | 390px | 390px (single items were never broken) |

  Fixed by giving the **container** one aspect ratio and making every level below it `h-full`, so
  nothing inside contributes a height. The ratio comes from the *first* item — the author put it
  first — and is clamped to 4:5–16:9. Sizing the container to each slide instead would have been
  worse: the card would then change height as you swipe, moving everything below it in the feed.

  Extracted as `stripAspectRatio()` and tested (8 cases), including the reverse-order case that
  proves a later item cannot stretch the strip. One test I had to correct rather than the code: I
  claimed 3:4 was inside the clamps, and it is not — 0.75 is taller than 4:5, so an ordinary phone
  portrait is cropped by ~6%. That is deliberate and now says so out loud, since anyone changing
  `MIN_STRIP_RATIO` should know that is what they are changing.

  Worth noting the shape of this: the bug was **invisible to every test in the file** and obvious in
  one screenshot. It is the second thing looking at the output caught in an hour (the first was the
  fixture's invisible markers, task 327), which is a fair verdict on how much of this feature's
  correctness the suite can actually speak to.

- 2026-09-18 (later still) — **Arrows added, for desktop and for discoverability.** Maintainer: "since
  the public version needs to be accessible on a desktop computer, we need some way to move the
  carousel without swiping."

  A finding while fixing it: shadcn's `Carousel` **already** wires ←/→ and takes focus via
  `tabindex="0"`, so the strip was never swipe-*only*. But nobody discovers a keyboard shortcut on a
  photograph, and a mouse cannot swipe an Embla carousel (a trackpad sometimes can), so on a laptop
  the second photo was effectively unreachable.

  So `CarouselPrevious`/`CarouselNext` are now rendered — they had to go **inside** `<Carousel>`,
  since they consume its provider through `useCarousel()` — and shown only where the pointer is fine
  (`pointer-coarse:hidden`). On a phone they would sit on top of the photograph duplicating a gesture
  that already works; on a laptop they are the only way through. Repositioned inside the frame, since
  shadcn's default `-left-12` assumes a carousel with margin around it and this one is full-bleed in a
  card. The dots stay in both cases — saying *how many* there are is a different job from moving
  between them.

  Also translated the two `sr-only` labels in `ui/carousel/` to Danish, with `LOCAL DEVIATION`
  comments. All user-facing copy in this app is Danish, and a screen-reader label is user-facing copy
  — it is just the part only some users hear.

  Verified the variant actually emits rather than silently doing nothing: the built CSS contains
  `@media (pointer:coarse){.pointer-coarse\:hidden{display:none}}`. Worth checking, because an
  unrecognised Tailwind variant produces no error and no rule — it simply never hides anything. (My
  first grep for it failed on a space Tailwind does not emit, which is a neat illustration of the same
  trap.)

  **The public page cannot reuse any of this** — it is server-rendered with no bundle, so there is no
  Embla to put arrows on. Recorded as a criterion and an options analysis in task 323, where the
  recommendation is to show every item rather than port a carousel.

- 2026-09-18 (later still) — 🐞 **The carousel's slides were top-aligned and not filling their frame.**
  Maintainer: "in the carousel everything is top aligned, it should be centered both horizontal and
  vertical."

  The clue that found it was theirs: *in the carousel*. Single-item glimt looked right, and they skip
  the carousel entirely — so the fault was in the Embla path, not in the aspect box.

  Cause: upstream `CarouselContent` forwards `props.class` to the inner flex **track** but leaves its
  own viewport div auto-height. So `<CarouselContent class="h-full">` set the height on the track,
  whose parent was auto — and `h-full` against an auto-height parent resolves to auto. The chain
  broke there, silently: slides sized themselves from their content, sat at the top of the frame, and
  `object-cover` had no box to cover. Fixed with `h-full` on the viewport div and a LOCAL DEVIATION
  note explaining why it is harmless upstream (auto parent → auto).

  Also wrote `object-center` out explicitly rather than relying on it being Tailwind's default: this
  is the line that decides *which part* of a photograph survives a crop, and leaving it implicit
  invites somebody to assume the top is kept.

- 2026-09-18 (later still) — **Crop budget loosened**, per "crop the long direction a little (maybe
  10–15%)". `MIN_STRIP_RATIO` went from **4:5 to 3:4**, so the most ordinary shape a phone produces in
  portrait is now shown **whole** instead of losing ~6% for no good reason. Everything between 3:4 and
  16:9 is used exactly, so most glimt are not cropped at all. Measured on the live fixture data:

  | hold | items | first item | strip shape | height @390px | crop per slide |
  |---|---|---|---|---|---|
  | 45 | 2 | 1600×900 | 1.78 exact | 219px | 0%, **68%** |
  | 43 | 2 | 1600×1200 | 1.33 exact | 292px | 0%, 44% |
  | 42 | 4 | 1200×1600 | 0.75 exact | 520px | 0%, 44%, 25%, 58% |
  | 44 | 1 | 1400×1400 | 1.00 exact | 390px | 0% |

  **The lead photograph is never cropped now. Later slides in a different orientation still are,
  heavily.** That is inherent to one-shape-per-strip and is the trade for a card that does not change
  height as you swipe. Worth knowing that the fixture is adversarial here on purpose — it mixes
  landscape and portrait in one glimt precisely to expose this — whereas real glimt are usually all one
  orientation, because people hold a phone one way. Raised with the maintainer rather than
  re-decided unilaterally; the alternative is `object-contain` with a backdrop for non-matching
  slides, which crops nothing and letterboxes instead.

### ⚠️ Not verified, and cannot be from here

The suite runs in `node` and mounts nothing, so **nothing about how this looks or feels has been
checked**. Specifically worth a look on a phone viewport before this is called finished:

- whether the carousel's swipe feels right, and whether the dots are visible against a bright photo
- the 44px overflow-menu target with a thumb, in the dark
- safe areas: the feed sits inside App.vue's scroll container, so it should be fine, but the compose
  button in the header has not been seen next to a notch
- whether `font-nathejk` at `text-2xl` reads as a page title or as shouting
