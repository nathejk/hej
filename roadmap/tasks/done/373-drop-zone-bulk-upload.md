# 373 — The drop zone: three hundred files, three at a time, one bad file fails alone

**Status:** done
**Priority:** high
**Created:** 2026-09-22
**Picked up by:** agent session (Zed)
**Started:** 2026-09-23
**Completed:** 2026-09-23

## Description

The uploader half of `GET /admin`: a `multiple` file input and a drag-and-drop zone that accepts a
folder's worth, uploading **one request per file with a concurrency of three**, a row per file with a
thumbnail once it lands and a red row with a reason if it does not, and total/remaining counts.

Built as an `html/template` in `go/cmd/api` with inline CSS and a small amount of hand-written vanilla
JavaScript — **not in `vue/`**. PRD 022 §7 and §8.10: the PWA is not touched, no route is added to
`vue/src/router/index.ts`, and the device gate is not modified. The reason curation cannot live in the
app is §8.1: `vue/src/router/index.ts` runs a device-class gate *ahead* of the auth gate and
`window.location.replace`s a desktop visitor to the website, and its own comment says revisiting that
gate is a new PRD. Bulk upload from an SD card is desktop access by definition. The absence of a build
step here is the reason not to start a second frontend for it either.

**One request per file** is the whole design, not an implementation detail. One bad file cannot fail a
batch; a dead connection — a closed laptop, a train tunnel — loses one file rather than three hundred;
and because upload is idempotent (task 372) the batch is resumable by re-dragging the same folder, with
already-uploaded files recognised and skipped. Failures never stop the batch. The rejected alternative,
one multipart request with 300 parts, fails all of them on the last byte and gives the photographer
nothing to retry but the whole card.

The page states the **event year in large type**, because it is the one thing that cannot be undone by
editing (PRD 022 §7). Copy is Danish. Headlines use the Nathejk face. Icons, where needed, are inline
SVG copied from Lucide — this page has no build step, so not the Vue package. The page may use modern
JavaScript freely: it is behind auth, on a laptop, and needs `File`, `fetch` and `FormData`; it does
not inherit the public site's deliberately lower floor. Keyboard operability and real `<label>`s are
required (PRD 022 §6) — two or three people use this for hours.

## What landed

The uploader half of `GET /admin`: a labelled `multiple` file input, a drop zone that accepts a dropped
folder, a worker pool of three, a row per file with a preview and an outcome, and live total/remaining counts.
No npm dependency, no bundler, nothing added under `vue/`.

Three decisions worth recording:

**The row preview is the local `File`, not a fetch of the stored rendition.** No round trip, instant, and it
shows the photographer what they actually selected rather than what the server made of it. The object URL is
revoked on load *and* on error — without that a 300-file batch holds 300 decoded bitmaps and the tab dies
partway through exactly the batch it was built for. The server's own rendition is the contact sheet's job
(task 374), which is why this task needed no media endpoint.

**Hidden files are skipped client-side; everything else is sent.** `.DS_Store`, `._originals` and `Thumbs.db`
come off every card, and the server would refuse them correctly — as three hundred red rows nobody can read
past. Anything that might be a photograph still goes to the server, because the decode is the validation.

**`readEntries` is called in a loop.** It returns at most a hundred entries per call, so a single read would
silently upload the first hundred photographs of a card and drop the rest — the worst failure shape available
here, because it looks like it worked.

## A gap in the testing, stated rather than papered over

There is no JavaScript runner that reaches this page — `vue/`'s Vitest covers the PWA, which this deliberately
is not part of — so **there is no honest way to assert from Go that three requests run concurrently.** The
tests cover what is checkable: that the page renders to completion (a failing template action is a blank screen
for the curator), that the properties the criteria name are in the markup, and that the numbers the design
balances on have not drifted. A browser-level test of the batching would need a runner, and that is a decision
rather than an omission.

What stands in for it is a live check: four concurrent uploads against the running stack, one of them junk.
The junk got `400` with a Danish reason while the other three succeeded — **one bad file failed alone**, which
is the property the whole one-request-per-file design exists for.

## What the live run taught us, and it is worth keeping

Two files reported `already` although the rows had been deleted from the database earlier. The cause is that
the projection **replays from the event log on every boot**, so the API rebuild recreated them — and therefore
the `DELETE FROM photo` used to tidy up after task 372 was never a deletion at all, nor was the `deleted=1`
flag set by hand.

That is correct behaviour rather than a bug, and it is a useful reminder with teeth: **the log is
authoritative, so a real takedown has to be an event** (task 379). Anybody tidying this surface with SQL is
not tidying anything.

## Acceptance Criteria

- [x] Each file is its own request, with at most three in flight — the pool is in the script and the constant
      is asserted; the concurrency itself is verified live rather than in Go, for the reason above
- [x] Per-file outcome and a plain-language Danish reason on failure, for every status the endpoint can
      return; failures advance the queue through a `finally` so they cannot stall the batch
- [x] Re-dragging the same folder skips what landed — the server's three outcomes are rendered distinctly, so
      "already" is visible as such rather than shown as a fresh success
- [x] A row shows the position distinguishably for inside / outside / unknown, with `unknown` styled as a
      caveat rather than a rejection — it is a statement about us, not about the photograph
- [x] The event year is stated unmistakably: in the headline, in the title, and as a data attribute for the
      script rather than interpolated into JavaScript
- [x] No `npm` dependency, no bundler, no framework, and no file added under `vue/` — asserted against the
      script section and against `git status` for `vue/`
- [x] The drop zone collapses to a thin bar when idle and expands during a batch, driven by a class
- [x] Keyboard-operable: a real `<label for>`, an input hidden with `opacity` rather than `display:none` so it
      keeps its place in the tab order, and a `:focus-within` ring on the label so focus is visible

## Not done here

- **Per-file byte progress.** Rows show queued → uploading → outcome, and the batch bar shows files completed,
      but not bytes within one file: `fetch` exposes no upload progress event. Doing it properly means
      `XMLHttpRequest` for the upload path alone, which is a real trade and worth its own decision rather than
      a quiet swap. Per-file *outcome* and batch progress are what the criteria asked for and are present.
