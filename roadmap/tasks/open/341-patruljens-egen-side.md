# 341 — Patruljens egen side

**Status:** open
**Priority:** high
**Created:** 2026-09-19
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 011 §6, §7. The page itself, at `/offentligt/patrulje/{number}`, server-rendered and complete without
JavaScript. The map is a separate task (342) and a progressive enhancement on top of this.

Layout, from the maintainer's brief:

```
┌──────────────────────────────────────────────┬──────────┐
│ Patrulje 42 · Ørnene                         │ ┌──────┐ │
│ 1. Søllerød Gruppe · Det Danske Spejderkorps │ │diplom│ │
│ mindst ~24 km                                │ └──────┘ │
├──────────────────────────────────────────────┴──────────┤
│   map of the race area (task 342, enhancement)          │
├─────────────────────────────────────────────────────────┤
│   the scans, in order, as a list                        │
└─────────────────────────────────────────────────────────┘
```

Patrol name in `font-nathejk` and large; gruppe and korps beneath it, smaller — a patrol is known by its
name, and the korps is context. The distance is the one number on the page, so it gets room. The diploma
thumbnail is the visual anchor and the thing a parent clicks.

**The scan list is not the map's fallback, it is a requirement.** Every registration the patrol collected,
in race order, with its kind and time — **including the ones with no position**, which are listable but not
plottable (a post can register a patrol manually). A member whose patrol's scans have no coordinates at all
must still get a usable page.

**The page must state what the track covers** — that it records where a phone had the app open, not where
the patrol walked — in one sentence a parent will read. Without it, a gap reads as "they stood still here".
This sentence is the honesty requirement of the whole feature; do not shorten it into meaninglessness.

**The not-yet page** (gate closed, task 330) is a **first-class screen, not an error path**: it is what
every link to a patrol page shows for most of the year. It says what the page will contain and when it
appears, carries **no patrol data at all**, and is **indistinguishable from the answer for a patrol number
that does not exist** — otherwise the URL space becomes a live finish-order feed.

Depends on 330 (gate), 338 (read model), 339 (distance). The diploma thumbnail's source is task 345; until
then leave the slot structurally present but empty rather than faking it.

## Acceptance Criteria

- [ ] `GET /offentligt/patrulje/{number}` renders the header (name, gruppe, korps), the distance, the
      diploma slot and the ordered scan list. OpenAPI annotated.
- [ ] Complete and useful with **JavaScript disabled**; the map's absence degrades to nothing worse than a
      missing section.
- [ ] Scans listed in race order with kind and time, including un-positioned ones.
- [ ] The track-coverage sentence is present and readable by a parent.
- [ ] Headline uses `font-nathejk`; no font family set anywhere per-component (`.rules`).
- [ ] `korps` rendered as its label; `andet`/empty omitted (task 338).
- [ ] Not-yet page implemented, carrying no patrol data, identical to the unknown-patrol answer.
- [ ] A patrol with no track, or no positioned scans, still reads as a page about a patrol that was there.
- [ ] Unauthenticated, ignores the session cookie, `noindex, nofollow`.
- [ ] Renders on the oldest device available and with CSS disabled.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §6 / §7 / §10 (Phase 2). Depends on 330, 338, 339.
