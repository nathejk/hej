# 345 — The diploma

**Status:** done
**Priority:** medium
**Created:** 2026-09-19
**Picked up by:** agent
**Started:** 2026-09-21
**Completed:** 2026-09-21

## Description

PRD 011 §8, §11 Q6. The diploma thumbnail on the patrol page, and the full diploma behind it.

**The constraint that shapes this:** the org's architecture forbids a service calling another service's HTTP
API, so `hej`'s BFF **may not** call `diplom`. Three options, and PRD 011 §8 now leans (a):

- **(a) Link the visitor's browser to `diplom`.** A link is not a service-to-service call — the browser
  fetches it — so the rule holds. Cheapest, and reuses artwork and layout that already work. **The public
  frontpage makes this much better than it was:** the original objection was that bouncing a 12-year-old to
  another domain to log in again would kill the feature, and with no login anywhere on this surface that
  objection is gone. Remaining costs: a second origin, `diplom` is hardcoded to 2024 and needs the current
  year's data, and it still needs a thumbnail from somewhere.
- **(b) Render the diploma in `hej`** from its own projections. One origin, one deployment. Costs:
  reimplementing PDF/image generation (`go-pdf/fpdf` is small) and re-creating the artwork pipeline.
- **(c) Move generation into `hej` and retire `diplom`.** Cleanest end state, largest change, touches a repo
  outside this one.

**The finish time must use the same definition `diplom` already uses** — the scan at the last checkpoint —
or the page and the diploma will disagree about when a patrol finished. Conveniently that is also task 330's
trigger 1, so there is one definition of "finished" across the gate, the page and the diploma. Keep it that
way.

**Not in scope:** printing, posting or emailing the diploma (PRD 011 §4). In-page display, whatever the
browser's own share/save affords, and nothing else. No mail pipeline.

Open sub-questions to settle as part of this task: does the current year's artwork exist, who makes it, and
who renders the thumbnail.

## Acceptance Criteria

- [x] An option from §11 Q6 is chosen and the reasoning recorded in this task's log and back in the PRD.
      **(c), by the maintainer.**
- [x] A diploma thumbnail appears in the patrol page's slot (task 341) for a patrol that finished.
- [x] The full diploma is reachable from the thumbnail without a login.
- [x] The finish time matches `diplom`'s definition — the last-checkpoint scan — and is the same value the
      gate and the page use.
- [x] A backstop-opened patrol (no finish) shows **no diploma slot at all**, not an empty frame — see task
      346.
- [x] No service-to-service HTTP call from `hej` to `diplom`. **None: generation moved here, so there is
      nothing to call.**
- [x] Whether the current year's artwork exists is confirmed, and if it does not, who is making it is
      recorded here. **It does not; the 2024 poster is the mock. See the log.**

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §8 / §11 Q6 / §10 (Phase 3).
- 2026-09-21 — **Done, as option (c): generation moved here.**

  > *"a sibling repo 'diplom' can be used as guideline to how we create a diploma — the diploma generation
  > logic should be moved here, use an old graphic as mock, we will replace before launch"*

  §8 had leaned on (a), linking the browser out. The maintainer's call overrides it, and the argument that
  settles it turns out not to be architectural but editorial: the diploma is one element on a page this repo
  already renders, gated by a verdict it already computes, from a finish time it already has. Linking out
  would have meant a second origin, a second deployment and a year hardcoded in another codebase — for a PDF
  that is a background image and four lines of text.

  ### What was ported, and the one thing that was not

  `diplom`'s `pdf.go` is ~90 lines: A4 via `go-pdf/fpdf`, a full-bleed background, the patrol name, three
  sentences, and the finish time from the last checkpoint's scan. All of that came across into
  `internal/diploma`, a pure renderer with no HTTP and no database — the same shape as `internal/distance`.

  **The patrol photograph did not come.** `diplom` fetches one from `natpas` and places it mid-page. That
  cannot cross onto this surface, and the reason is worth more than the feature:

  - These pages are **unauthenticated**, addressed by a patrol number anybody can type (§11 Q2).
  - §0b.2 settled that photographs are publishable only where a **curator obtained consent** as the
    photograph entered an album. A `natpas` portrait has been through no such gate.
  - A photograph of eight children's faces is a stronger identifier than any of the names task 337 goes to
    trouble over. Publishing one would make that work pointless.

  So the mock has an empty middle, and the note to whoever draws 2026's artwork is: **design it for a diploma
  without a photograph**, not around a hole where one used to be.

  ### The mock, and what must happen before launch

  The 2024 poster (`Diplom2024_patrulje.png`, 2480×3508) downscaled to 150 dpi JPEG, 620 kB, embedded with
  `go:embed`. It says **2024** on it, visibly and deliberately, so nobody mistakes it for finished work.

  Recorded in code as `diploma.ReplaceBeforeLaunch` rather than only here, so it shows up in `go doc` and in a
  grep for "launch":

  1. **The artwork** — 2026's design, 300 dpi, A4 portrait, no photo area. **Does not exist yet; nobody is
     named as making it.** That is the one open dependency this task leaves.
  2. **The headline font** — `diplom` embeds `impact.ttf`. Impact is a Microsoft core font whose
     redistribution is restricted, so this renders in fpdf's built-in Helvetica. If the real artwork wants
     Impact *in the text* (its own headline is part of the image, so it does not), the licence question has to
     be answered rather than inherited.
  3. **The route line** — `diplom` hardcoded "fra Lundby til Glumsø" for 2024, which is how a diploma ends up
     naming last year's villages. Now `EVENT_ROUTE`, and **empty omits the line**: a diploma with no route
     reads fine, one with the wrong places is a certificate somebody frames with a mistake in it.

  ### Two bugs that only a rendered sample could have caught

  I rendered one and looked at it. Both of these passed every unit test I had written at the time:

  - **Mojibake.** "Ørnene" printed as "Ã˜rnene". fpdf's core fonts are single-byte; `diplom` re-encodes to
    Latin-1 and I had dropped that line as incidental. It is not incidental — it is the difference between a
    patrol's name and rubbish. Now `latin1()`, with a test pinning it.
  - **Text on top of the artwork.** `diplom`'s coordinates put the name at y=210mm, where its 2024 layout had
    clear paper; on this poster that lands on "Vi ses i mørket!". Moved into the clear middle, with a comment
    saying the constants belong to the artwork rather than to any layout logic.

  A third, found while fixing the first: the Latin-1 encoder substitutes **0x1A**, a control character, for
  anything unmappable — so a patrol called “Rævene” would have printed wearing two boxes. Typographic
  punctuation is now folded to ASCII first.

  The sample renderer survives as `TestWriteSample`, skipped unless `DIPLOMA_SAMPLE` is set, precisely so the
  person who replaces the artwork looks at the output rather than trusting it.

  ### The gate is two conditions, and a test proves the second one bites

  `openPatrol` says whether the page exists; `verdict.Finished()` says whether there is a diploma. A
  backstop-opened patrol has a page and **no** diploma (task 346) — without the second check a patrol that
  did not finish would be handed a certificate saying *har gennemført*. Verified by disabling it and watching
  both routes start answering 200.

  The finish time is the **gate's** value, rendered in Europe/Copenhagen as `diplom` does, so the gate, the
  page and the certificate cannot disagree about when a patrol finished.

  ### What the dev stack could and could not verify

  - Backstop-opened patrol 71: both routes answer **404**, and the page renders no slot (only the CSS rules
    mention `diploma`). ✓
  - A **finished** patrol does not exist anywhere in the dev data — not in 2026 (no scan attributes to the
    *Mål* checkgroup; `checkpersonnel` covers Start and Postlinje 1–2 only) and not in 2025 (one checkgroup
    row, two personnel rows — see task 344). So the 200 path is covered by the handler tests and by the
    sample I rendered and read, not by the dev stack. **Task 348 should look at a real diploma if production
    has a finished patrol.**

  ### Also

  A structural test asserts `diploma.Diploma` has no fields beyond the five it needs, with a message pointing
  at the package doc — because the obvious "improvement" to a diploma is a picture on it, and a year from now
  the reasoning will not be in anyone's head.

  New dependency: `github.com/go-pdf/fpdf` v0.9.0 — the same version `diplom` uses, pure Go, no cgo.

  `gofmt`, `go vet`, `go test ./...` clean.
