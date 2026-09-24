# 395 — Migrate the admin tool to HTMX + Alpine + Pico, vendored

**Status:** done
**Priority:** medium
**Created:** 2026-09-23
**Picked up by:** agent
**Started:** 2026-09-24
**Completed:** 2026-09-24
**Completed:**

## Description

Option D of the stack decision (see task 394 for the measurement and the options weighed). The maintainer chose:

> **HTMX + Alpine + Pico, all vendored, no build step anywhere.**

Tailwind was ruled out specifically because it needs a build pipeline, and the only existing Tailwind lives
inside the PWA's Vite build — using it on the website would mean either a second pipeline or coupling the two
things that must stay apart:

> *"It's essential that we do not mix the pwa and the website, it's two different things and should be kept as
> such."*

Pico fills the CSS gap with a single static stylesheet, so the whole stack vendors like Leaflet already does.

## The rules this must respect

Three, stated by the maintainer and load-bearing:

1. **The PWA and the website stay separate.** Nothing here may reach into `vue/`. Note the pre-existing smell:
   the website's Leaflet is served out of `vue/public/vendor/`, i.e. the PWA project's folder. The admin assets
   should be served from the Go binary via `embed` instead, and that boundary is worth tidying while here.
2. **Custom JS stays where it solves a real problem.** *"If we have custom JS that solves a problem not solved
   by any framework, then we just keep that custom JS, even if we might install additional js frameworks."*
3. **OpenAPI is not required for the admin surface.** *"not a problem that that admin interface does not have
   openapi specs."* This unblocks HTML-fragment endpoints, which would be awkward to document as an API — but
   note task 380 wired `/api/admin/` into the OpenAPI guard, so fragment routes need excluding from it
   explicitly rather than by accident.

## What is migrating, and what is not

From task 394's measurement of `page.js` (1,436 lines):

| Section | ~Lines | Fate |
|---|---|---|
| Sheets, forms, fetch-then-render, album list, counts refresh | ~600 | **HTMX fragments.** The target. |
| Contact sheet: selection set, shift-click ranges, keyboard nav | ~250 | **Keep**, Alpine may hold the state |
| Upload queue: 3-at-a-time, per-file rows, drag-and-drop | ~400 | **Keep.** HTMX cannot express a bounded concurrent queue with per-file progress |
| Leaflet position map + tile retry | ~150 | **Keep** |

So this is not a rewrite. It is deleting the ~40% that is boilerplate and leaving the ~40% that is the actual
product.

## The one real risk

**HTMX's swap model versus the contact sheet's selection.** PRD 022 §7's hard constraint is that the selection
survives every action — that is why the sheets are overlays and not routes (task 390). A naive `hx-swap` over
the grid destroys the selection set.

Mitigation is either `hx-preserve` on the grid, or Alpine owning the selection *outside* any swapped region.
Decide this before migrating the contact sheet, and migrate it **last**.

## Order of work

Lowest risk first, so the pattern is established before it meets the hard part:

1. **Vendor** `htmx.min.js`, `alpine.min.js`, `pico.min.css` into `cmd/api/adminui/vendor/`, embedded and
   served from the Go binary. Pin versions in a file next to them, as `scripts/vendor-leaflet.sh` does.
2. **Spike Pico on one page** — check it does not fight the `role="dialog"` sheet stack's z-index or Leaflet's
   own controls. Classless frameworks set broad `button`/`dialog` defaults. ~30 minutes; settles whether Pico
   survives contact with this markup.
3. **The album list** (task 391) as the first fragment: pure fetch-and-render, no selection state, so it is the
   cleanest possible proof of the pattern.
4. The four action sheets, one at a time: add-to-album, credit, patrol, delete.
5. The position sheet — **Leaflet needs `invalidateSize()` after the overlay is visible** (task 390), and a
   fragment swap must not discard the map instance.
6. The contact sheet, last, with the selection decision already made.
7. Replace the source-text guards with fragment tests as each one migrates — see below.

## The payoff worth measuring

**38 tests currently assert on JavaScript source text** (`adminPageSource` + `strings.Contains`). They have
caught real bugs — the map's dict-vs-array lookup (task 389), the delete copy (task 379) — and they are brittle
by construction: three false positives in one session, where a needle matched the comment explaining it.

Every fragment endpoint turns those into ordinary Go HTTP tests against returned HTML. **That is the strongest
argument for this migration, stronger than the line count**, and it should be tracked: each migrated feature
should delete source-text guards and add behavioural ones.

## Progress

**Done.** All seven steps, in five commits.

- **1. Vendored** — htmx 2.0.4, Alpine 3.14.9, Pico 2.0.6 under `cmd/api/adminui/vendor/`, embedded, served at
  `/admin/vendor/:asset` behind `requireAdmin`. Nothing in `vue/`.
- **2. Pico spiked** — it did not fight the sheet stack or Leaflet's controls, but it does scale the root font
  size to 131.25% on wide viewports through a zero-specificity `:where(:root)` rule. Pinned to 100%, and the
  `PICO:` reconciliations are marked in `page.css`. The album cards were condensed afterwards.
- **3. The album list** — the first fragment. ~190 lines left the script.
- **4. The action sheets' pickers** — add-to-album, patrol, delete. The credit sheet had nothing to migrate.
- **5. The position sheet's post picker.** The Leaflet map was not touched.
- **6. The contact sheet's thumbnails.**
- **7. `page.js` split per feature** — nine files, ~1,230 lines, largest 256.

`page.js` went from 1,436 lines to nine files whose largest is 256. Roughly 600 lines of fetch-then-render became
Go templates; ~48 behavioural tests replaced the source-text guards that covered them.

### What was decided along the way, for whoever changes this next

**The one convention.** *A fragment renders what the server knows; the browser keeps what only it knows.* The
selection is the clearest case — a Set in the script that no fragment goes looking for — so a sheet's "42 billeder
bliver tagget" stays client-side while "der er ingen album endnu" does not. Everything else followed from it.

**The shell carries `hx-trigger="load"`; a fragment must not.** A fragment re-stating its own `load` trigger
re-fetches forever and the only symptom is a busy network tab. The guard reads the trigger's *events*, because
`load, albums-changed` sails past a literal needle and loops identically.

**A fragment may render differently and must never decide differently.** Both create paths go through one
`createAdminAlbum`; both library readers go through one `readAdminLibraryPage`. An event log — or a filter — that
disagreed with itself depending on which surface produced it would be the worst outcome of adopting htmx.

**The selection risk in §7 was not where it looked.** See below; this was the task's stated main hazard and the
answer is short.

**A sheet's fragment is fetched from JavaScript when the sheet opens.** Nothing htmx can put on a hidden element
expresses "when this is shown": `load` would fetch five sheets on every page load and `revealed` does not fire for
something unhidden by script. The first page of *thumbnails* is the exception and is declarative, because htmx is
deferred and the script is inline — `window.htmx` does not exist while the script runs.

**Fragments are `/admin/*`, not `/api/admin/*`.** That keeps `requireAdmin` and
`TestAdminRoutesUseOnlyTheAdminWrapper` over them while leaving them outside task 380's OpenAPI guard, which scopes
to `/api/`. There was nothing to exclude, so the reason is recorded in `routes.go` instead.

**A fragment that renders a list it just wrote into has to wait for the fold.** The create's answer *is* the list,
and the first live check returned one without the album the note said had just been created.

## What the selection risk turned out to be

The task called this the one real risk: "a naive `hx-swap` over the grid destroys the selection set", with
`hx-preserve` or Alpine offered as mitigations. Neither was needed.

**The selection was never in the grid.** It is a Set of ids in the script, held outside the DOM deliberately — the
code's own comment already said a DOM-derived selection "would be lost by any re-render, and it would silently
shrink to what is currently loaded". A swap replaces cells; the Set does not notice.

What *is* read back from the DOM is `order`, the display order a shift-click range resolves against. That is not
state: it is the definition of "the cells currently shown". **Selection outside the DOM, display order from it** —
that distinction is the whole answer, and it is why the client's job after a swap is three lines.

## Why nine spliced function declarations, and not ES modules

Modules would give the same split with real imports and no build step. They were rejected because every asset on
this surface answers `no-store` (task 371), sitting behind the shared credential: nine module files would be nine
uncacheable requests on every page load where the injection is zero, and the person waiting is a photographer on a
hotel connection the day after the event.

So each file is one `function init…(ctx)` declaration — a complete, valid program a formatter and a linter can
read, which was task 394's whole reason for making these real files — and `page.html` splices them into one
`<script>` inside one IIFE, so nothing reaches `window`. Splitting mid-IIFE, where one file opens a closure another
closes, was rejected for the same reason task 394 existed: such a file is not a program.

`ctx` is the entire shared surface, and writing it down showed how small it is: the selection, three functions for
opening an overlay, a grid reload, the line an action writes its outcome to, and two notifications.

## Bugs this found

- **The action bar's outcome was unreadable.** It shared `#sheetnote` with the shown-count, and every action ends
  by reloading the sheet — so "42 billeder lagt i 2 album", written by the side that knew how many were already
  there, was overwritten by "42 vist" a moment later. It now has its own `#actionnote`.
- **Creating an album inside the add-to-album sheet silently dropped the boxes already ticked**, because it rebuilt
  the list from scratch. The fragment posts the ticked ids back and echoes them.

## Follow-ups

- **Alpine is vendored, served and loaded but not yet used by anything.** It earned its place in the allowlist for
  local sheet state, and every sheet turned out to need either nothing or a line of delegation. Either find it a
  job or drop it — a loaded library with no callers is a dependency nobody is paying attention to.
- **`page.css` is still one 330-line file** for nine scripts. Splitting it was not part of this task and the
  `PICO:` reconciliation notes are easier to find in one place, but the asymmetry is worth a look.
- The uploader, the selection model, the shift-click ranges, the keyboard navigation, "select all matching this
  filter" and the Leaflet map remain custom, per the maintainer's rule. That is the ~800 lines that are the
  product rather than the boilerplate, and none of it should be migrated later "for consistency".

## Acceptance Criteria

- [x] htmx, Alpine and Pico vendored into the repo with pinned versions, served from the Go binary, no build step
- [x] Nothing reaches into `vue/`; the admin tool's assets do not travel through the PWA's public folder
- [x] Pico verified against the sheet overlays and the Leaflet controls before any migration
- [x] The Nathejk headline font still applies where `.rules` requires it, over Pico's defaults
- [x] The uploader, the selection model and the Leaflet map remain custom, untouched
- [x] The selection survives every migrated action — asserted, since it is PRD 022 §7's hard constraint
- [x] Each migrated feature replaces its source-text guards with behavioural tests on the fragment
- [x] Fragment routes are excluded from task 380's OpenAPI guard deliberately, with the reason recorded
- [x] `page.js` ends up split per feature, which task 394 deferred to this task
- [x] Verified live per feature, not only at the end

## Notes

- Task 394 is the prerequisite and is done: the markup, CSS and JS are already real embedded files, so this can
  proceed one file at a time rather than as one large change.
- Whether this warrants a PRD is worth a decision. `.rules` requires one for significant changes; this is a
  stack change to an internal tool with two or three users, and it adds three dependencies to a service that
  currently has none on the front end. My read is that a PRD is proportionate to the *dependency* decision but
  not to the migration itself — but that is the maintainer's call, and it has not been made.
