# 316 — GlimtView feed, GlimtCard, GlimtMediaStrip

**Status:** open
**Priority:** high
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `/glimt` route and destination in `config/navigation.ts` + `router/index.ts`
- [ ] `GlimtView.vue` renders the feed from the task 313 store, paginated/lazy
- [ ] `GlimtCard.vue` with hold attribution, no name and no avatar anywhere
- [ ] Attribution navigates to the hold collection
- [ ] `Anmeld` on every card in one tap; `Slet` only on your own
- [ ] shadcn `carousel` added and used for multi-item media
- [ ] Empty state is the invitation to post
- [ ] `npm run test:unit` and `npm run build` pass

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
