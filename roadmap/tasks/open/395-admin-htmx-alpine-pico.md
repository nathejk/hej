# 395 — Migrate the admin tool to HTMX + Alpine + Pico, vendored

**Status:** open
**Priority:** medium
**Created:** 2026-09-23
**Picked up by:**
**Started:**
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

## Acceptance Criteria

- [ ] htmx, Alpine and Pico vendored into the repo with pinned versions, served from the Go binary, no build step
- [ ] Nothing reaches into `vue/`; the admin tool's assets do not travel through the PWA's public folder
- [ ] Pico verified against the sheet overlays and the Leaflet controls before any migration
- [ ] The Nathejk headline font still applies where `.rules` requires it, over Pico's defaults
- [ ] The uploader, the selection model and the Leaflet map remain custom, untouched
- [ ] The selection survives every migrated action — asserted, since it is PRD 022 §7's hard constraint
- [ ] Each migrated feature replaces its source-text guards with behavioural tests on the fragment
- [ ] Fragment routes are excluded from task 380's OpenAPI guard deliberately, with the reason recorded
- [ ] `page.js` ends up split per feature, which task 394 deferred to this task
- [ ] Verified live per feature, not only at the end

## Notes

- Task 394 is the prerequisite and is done: the markup, CSS and JS are already real embedded files, so this can
  proceed one file at a time rather than as one large change.
- Whether this warrants a PRD is worth a decision. `.rules` requires one for significant changes; this is a
  stack change to an internal tool with two or three users, and it adds three dependencies to a service that
  currently has none on the front end. My read is that a PRD is proportionate to the *dependency* decision but
  not to the migration itself — but that is the maintainer's call, and it has not been made.
