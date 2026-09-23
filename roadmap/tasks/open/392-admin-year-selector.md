# 392 — Choose the year being worked on, defaulting to the current one

**Status:** open
**Priority:** high
**Created:** 2026-09-23
**Picked up by:**
**Started:**
**Completed:**

## Description

Maintainer:

> *"We still have some photos from previous years, it should be possible to select what year we are working
> on — default to current year."*

Today every admin read and write is bound to `app.config.eventYear`, the deploy-time `EVENT_YEAR`. There are
**41 call sites** of it across the admin handlers, and each one becomes "the year the curator selected".

`event_year` already holds the years (`2025`, `2026`, plus a junk `null` row to filter), so the selector has a
source. `photo` currently holds only 2026 rows.

## This reverses a documented decision, so it needs care

PRD 022 §7 made the year deploy-time **on purpose**, and said why: it is the one thing a curator cannot undo by
editing. The tool shows it in large type with its own accent colour for exactly that reason (§5), and task
385's half-page tells a photographer *"if the year is wrong, stop and say so — everything else on the page can
be fixed, this cannot."*

Making it selectable **raises** that risk rather than lowering it: a wrong-year upload becomes a click instead
of a redeploy. So the safety half is part of this task, not a follow-up:

- The badge becomes the selector, staying prominent — not a small control in a corner.
- Working in a year that is **not** `EVENT_YEAR` must be visibly abnormal: a persistent marker, not a toast
  that scrolls away.
- The year belongs in the **URL** (`?year=2025`), not in a cookie or session. Visible, shareable, and it cannot
  silently persist into a wrong-year upload after a reload — which a sticky preference absolutely can.
- Every write path must take the year from the request, and there must be a guard that none of them reads
  `app.config.eventYear` directly any more. With 41 call sites, one missed is a photograph filed in the wrong
  year with no indication.

## Open question, blocking part of this

**Is a past year publishable, or is this archiving only?**

The public site serves exactly one year: `publicRoot()` is `/{EVENT_YEAR}`, so `/2025/album/…` does not exist.
A 2025 album could therefore be built, filled and "published" while remaining publicly unreachable — a
publish button that does nothing observable.

- **Archiving only** (the smaller change): the selector governs the curator's tool. Past years can be
  organised; publication is only meaningful for the current year, and the UI should say so rather than offering
  a button that has no effect.
- **Past years public too** (much larger): the public frontpage, album pages and media route all become
  multi-year, `publicRoot()` stops being a single value, and `PUBLIC_ALBUMS` probably needs per-year meaning.
  That is its own PRD-sized change and should not be smuggled in here.

Not started until this is answered.

## Acceptance Criteria

- [ ] The year can be chosen from the years that exist, defaulting to `EVENT_YEAR`
- [ ] The choice is in the URL and survives a reload; it is never inferred from a cookie
- [ ] Every admin read and write uses the selected year, with a guard that no admin handler reads the
      configured year directly
- [ ] Working outside the current year is visibly and persistently marked
- [ ] The `null` row in `event_year` is filtered out
- [ ] Uploading into a past year is verified live, and the photograph lands in that year and nowhere else
- [ ] PRD 022 §5 and §7 are updated: they currently state the year is immutable, and they will be wrong
- [ ] Task 385's half-page is updated — it tells a photographer the year cannot be changed
- [ ] The publishing question above is answered in this file before implementation
