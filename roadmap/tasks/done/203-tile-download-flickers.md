# 203 — "Hent nu" on Kortbilleder flickers and does nothing

**Status:** done
**Priority:** high
**Created:** 2026-09-02
**Picked up by:** agent session (Zed)
**Started:** 2026-09-02
**Completed:** 2026-09-02

## Description

Reported from a phone, 2026-09-02: tapping **Hent nu** on *Kortbilleder* changes the badge to "Hentes…"
for about half a second, then back to "Klar". The size does not grow.

Half a second is a request round trip, which narrows it to two candidates — and **both were silent**, which
is the actual defect. This is the third time the same shape of bug has appeared (tasks 197, 202, now this),
and each time the cost was not the failure but the app's inability to say it had failed.

### Candidate 1: no race area

`GET /api/race-area` answers **404** when no checkpoint has coordinates yet — deliberately, so a client
cannot mistake "nothing" for "cache everything" and try to download the whole country. `fetchRaceArea`
collapsed that into `null`, and the handler returned with a comment claiming it was "not an error the user
can act on". Wrong: the user had just pressed a button.

The local database has 13 checkpoints with 9 positioned, so this is **not** reproducible here — which is
itself worth knowing, since the deployed host is a different dataset.

### Candidate 2: an exception nobody saw

`offline.store.sync()` caught every exception and restored the previous state without a word. Anything
throwing inside the download — most plausibly `tileUrlSource`, which builds a **detached** Leaflet map to
generate URLs — produces exactly the same flicker.

That detached map is a real hazard, not a hypothetical one: a detached element has `clientWidth` and
`clientHeight` of zero, and both `setView` and the tile layer's `onAdd` compute a pixel bounds from the
container. A zero or NaN extent is the sort of thing that throws deep inside a mapping library.

### The second bug in the same screenshot

Every row read **"aldrig"** — beside `26,5 MB · 252 stk.` for the tiles. Both cannot be true. The Cache API
does not record when an entry was written, so for tiles, portraits and the shell there is no honest
timestamp; the code printed "never" for "unknown". And the contacts directory, which *does* know when it
synced, was having its timestamp dropped on the way to the store.

## Acceptance Criteria

- [x] A 404 from `/api/race-area` is reported as its own cause and explained calmly, paired with the fact
      that browsing the map still saves what you look at.
- [x] A network failure is distinguished from a 404.
- [x] An exception in any sync handler is surfaced to the user **and** written to the diagnostic log, so
      the next occurrence names itself.
- [x] `tileUrlSource`'s Leaflet map is sized and attached off-screen, then removed, so the zero-extent
      hazard is gone.
- [x] No row says "aldrig" when it holds data: the timestamp is shown only when it is known, and the
      directory's own timestamp is no longer dropped.
- [x] Tests for both silent paths.

## Progress Log

- 2026-09-02 — Task created from the report.
- 2026-09-02 — **Fixed the silence before the cause**, deliberately. Both candidates produce an identical
  symptom, I cannot reproduce either locally, and the deployed host has a different dataset — so the
  useful move is to make the app name whichever it is. `fetchRaceArea` now returns a three-way result
  (`area` / `none` / `offline`), and `sync()` reports `problem: 'error'` plus a `syncfail` entry in the
  diagnostic log when a handler throws.
- 2026-09-02 — Added `syncfail` to the track log's event kinds rather than starting a second log. That log
  is the one channel that survives the app being killed, and it is where anyone already looks.
- 2026-09-02 — **Hardened the detached map anyway**, since it is the most likely candidate and the fix is
  three lines: the container is now 256×256 and attached off-screen, and removed on dispose. Nothing is
  painted; this exists purely so Leaflet is never asked to reason about a zero-size viewport.
- 2026-09-02 — **The "aldrig" bug is the more embarrassing of the two.** A row reading `26,5 MB · 252 stk. ·
  aldrig` is self-contradictory, and on a page whose entire purpose is to be believed about what the phone
  holds, that is worse than an absent field. The Cache API genuinely cannot say when an entry was written,
  so the honest rendering is silence \u2014 the size and count already prove the data is there. The directory's
  timestamp *was* known and was simply being dropped in `reportDirectory`.
- 2026-09-02 — ✅ Green: 2 new tests, suite 393 across 32 files, `type-check` and `build` clean.
- 2026-09-02 — **Not resolved: which candidate it actually was.** The next tap will say \u2014 either
  "Løbsområdet er ikke lagt ind endnu" (the 404), "Forbindelsen holdt ikke" (network), or "Der gik noget galt
  i appen" with a `syncfail` line in `/sporing` naming the exception.
