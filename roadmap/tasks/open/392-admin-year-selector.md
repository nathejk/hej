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

**~~Is a past year publishable, or is this archiving only?~~** **Resolved (2026-09-23, maintainer): past years
public too.**

That is the larger of the two options and it makes this a **PRD-sized change**, not a task. It should be
written up as an amendment to PRD 011/022 (or its own PRD) and approved before implementation, because it
changes a *published* surface rather than an internal one. What follows is the analysis that amendment needs.

### Constraint 1: the set of public years must be explicit and boot-time

`routes.go` records why the prefix is a config literal today:

> *httprouter refuses to have a wildcard segment as a sibling of static ones — `/:year` next to `/api` and
> `/privatliv` **panics** at registration rather than resolving by precedence.*

So `/:year/album/:slug` is not expressible. The options are:

1. **Register a literal prefix per year**, looping over a known set at boot. Works, keeps every path a literal,
   and keeps the AST guards in `publicprivacy_test.go` and `publicvisibility_test.go` working — they parse
   `routes.go` and already resolve `publicRoot + "…"` to a representative year.
2. **A catch-all that parses the year itself.** Rejected. It would make those guards blind: they enumerate
   public routes from the route table, and `parseRegisteredPath` fails loudly on a path it cannot read
   precisely so a gap cannot open silently. A catch-all would hand them an empty list and they would report
   success over nothing — the exact failure mode task 382 was written to prevent.

So: option 1, with the year set from **configuration** rather than from the `event_year` projection. Reading the
projection would make a year public the moment an album in it is published; configuration makes it two
deliberate acts, and "which years does this deployment serve?" becomes answerable from the environment instead
of from the database.

### Constraint 2 — the one that needs a decision: a year prefix is not just albums

This is what makes the change larger than "add a year to the album route". `/{year}` today carries the whole
public site:

| Route | What making 2025 public would do |
|---|---|
| `/{year}` | the frontpage template renders the **glimt strip** and the **patrol search**, not only albums |
| `/{year}/glimt` | last year's public glimt feed |
| `/{year}/patrulje` and `/{year}/patrulje/{number}` | last year's patrol pages — route, scans, distance |
| `/{year}/privatliv` | harmless |
| `/api/public/patrol/{number}/diploma` | last year's diplomas |

**Glimt is safe, by design rather than by luck.** `publicGlimt` passes `app.glimtPublicCutoff()` into
`PublicFeed`, and that cutoff is **time-based** — the 30-day public retention window of PRD 019 §6 (task 310).
So 2025's glimt is filtered out whatever the routing does. It would render an empty section, which is a poor
page rather than a leak.

**Patrol pages are not.** They have no time gate at all — they open when a patrol finishes and stay open. They
carry no person (task 338's `publicpatrol` projection has no name, phone or email, and a structural test holds
that), so this is not a privacy *breach*; it is a decision about republishing last year's event that nobody has
made. It must not happen as a side effect of wanting album pages.

### Recommendation

**A past year gets an albums-only public surface.** Specifically:

- registered for a past year: `/{year}` (an albums-only index), `/{year}/album/{slug}`, `/{year}/privatliv`,
  and the album media and map endpoints scoped to that year;
- **not** registered for a past year: `/{year}/glimt`, `/{year}/patrulje`, `/{year}/patrulje/{number}`, the
  report endpoints, the diploma routes;
- the current year keeps the full site exactly as now.

That needs an albums-only variant of the frontpage template, since today's renders the glimt strip and the
patrol search unconditionally.

**Awaiting confirmation of that scope before implementation.** The alternative — the full site for every
configured year — republishes last year's patrol pages, and that is a decision to take deliberately rather
than to inherit.

### Other work this implies

- `publicRoot()` returns one string and is used in templates, redirects and `@Router` annotations. It becomes
  per-request rather than per-deployment.
- `/api/public/albums` (the map) and `/api/public/albums/{albumId}/media/{ordinal}` take their year from
  config. `album` is keyed `(albumId, year)`, so the media route needs the year rather than resolving an id
  that is only unique within one.
- `PUBLIC_ALBUMS` becomes per-year, or at least needs a stated meaning for a past year.
- Tasks 337, 351 and 382's guards enumerate the public surface from `routes.go` and assume a single
  `publicRoot`. They must be widened to every configured year — and widened *before* the routes are added, so
  a new year's pages are covered by the privacy walk and the visibility walk from the first commit rather than
  retrofitted.

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
- [x] The publishing question above is answered in this file before implementation — *past years public too;
      the scope of "public" still needs confirming, see the recommendation*
- [ ] Split: the curator's year selector and the multi-year **public** surface are separate pieces of work.
      The first is this task. The second needs its own PRD amendment and its own tasks, because it changes a
      published surface and must extend the privacy and visibility walks before it adds routes.
