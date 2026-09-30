# 473 — The credit editor showed nothing, and the credit showed in the wrong places

**Status:** done
**Priority:** high
**Created:** 2026-09-30
**Picked up by:** agent
**Started:** 2026-09-30
**Completed:** 2026-09-30

**PRD:** 025

## Description

Reported by the maintainer, with a screenshot of the credit sheet open on a credited photograph:

> here i have opened an image with photocredit but the existing credit does not show in editor
>
> also when clicking "entire crew" selector is still empty
>
> in the public album view photocredit should be removed from thumbnails, and in single image view credit should
> always be prefixed with "Foto: "

Four things, of which one was mine to apologise for and one turned out to be a different bug than the one reported.

## 1. The empty crew list was my fault, not the tool's

`/api/admin/crew` answered **500** at the time of the screenshot. Not a bug in the endpoint: the dev container was
sitting in `==> build failed; waiting for changes` after a **test-gate timeout I caused**, running mutation checks
against `patroltrack.go` while validating task 471. `loadCrew`'s catch then leaves the listbox empty, which is what
the screenshot shows.

Verified after restoring the container: `section=pr` returns 9 people, `section=all` returns 147. The picker and both
its sections are fine.

**Worth changing about how I work, not about the code:** mutation-testing in a repo whose dev loop rebuilds on every
save leaves the maintainer's environment broken for as long as the mutation is on disk. Restoring the file is not
enough — the container keeps the failed verdict until something changes again.

## 2. The credit sheet now says what the credit is

It showed nothing about the current credit. The only text was the **placeholder** of the last name typed on this
machine, which reads exactly like a filled field and belongs to a different photograph.

The sheet now states it: *"Nuværende fotokredit: Anna (valgt fra crewlisten)"*, or *"De valgte billeder har
forskellige fotokreditter"*, or *"Billedet har ingen fotokredit endnu"*.

**Stating it rather than prefilling it**, which was the obvious fix and is wrong twice over:

- with several photographs selected, a prefill is a guess that silently overwrites the ones it guessed wrong about —
  the reason this sheet never prefilled;
- on a photograph credited from the **roster** it would replace a reference with a typed name the moment the curator
  pressed the button.

## 3. The bug behind the bug: the viewer's editor was converting references to text

Found while fixing (2). `vieweredit.js` prefills its field from `data-credit` — which is the **resolved name** — and
saving writes that name into `photo.credit`. So editing a roster-credited photograph in the viewer, or merely opening
it and pressing Gem, silently replaced the erasable reference with a permanent string on the append-only log.

That is PRD 025 §8 D1 running backwards, and it was the decisive argument for the whole feature:

> A name copied into `credit` is also on the append-only event log, so "delete me" could never be fully honoured. A
> reference can be.

The editor no longer prefills a roster credit. It shows who it is in the hint — *"Krediteret Anna, valgt fra
crewlisten. Skriv kun et navn her, hvis krediteringen skal skiftes til fri tekst"* — so changing the kind stays
possible and becomes deliberate.

### What made this possible: the kind on the wire, not the reference

The reference deliberately never reaches a response (PRD 025 §6 R6), and that is right. The consequence nobody had
noticed is that an editor cannot then tell the two kinds apart. So `adminLibraryPhoto` gained
`CreditIsCrew bool` — one bit, neither of whose values is a person — rendered as `data-credit-crew="1"`.

It needed **two** privacy exceptions, which is a consequence of there being two guards with different scopes rather
than of the field being doubtful: `libraryPersonShapedExceptions` for the struct walk, and a named exception in
`TestAdminLibraryPayloadHasNowhereToPutAPerson`, which deliberately holds the admin payload to the *public* standard
so that `creditCrewId` cannot reach the wire. That test now also asserts `CreditCrewID` is **still** refused, so "add
the kind" cannot become "add the id" by following the same paragraph.

## 4. The grid's credit plate is gone; the viewer always says "Foto: "

**The plate** (task 422) is removed from the album grid, and its CSS with it — rules for markup nothing emits are dead
code nobody can identify as dead, and these were subtle enough (absolute positioning, `pointer-events: none`) to look
deliberate for years.

This **reverses** task 403's reasoning, which is recorded in the test rather than deleted: *an attribution that renders
only once a script has run is an attribution we stop making for everybody whose script did not run.* That cost is real
and is now paid. What it buys is a grid that can be scanned — the plate sat over the bottom-left of a 150px tile, on
some tiles and not others. The credit stays in `data-credit`, so it is in the page a reader or a scraper sees, and it
reaches everybody who opens a photograph.

**The prefix** is `creditLine()` in `viewer.js`: `"Foto: "` plus the name, with any existing prefix stripped first.
That last part is not defensive programming, it is required by the data — and the live dev album shows both forms side
by side:

	data-credit="Anna"                 a roster credit, resolved to a bare name
	data-credit="Foto: Vibeke Krogh"   a typed credit, because until now the stored string *was* the rendered line

Adding the prefix unconditionally would render "Foto: Foto: Vibeke Krogh" for every credit set before today. So the
rule is idempotent, and both kinds now render identically — which is the point of the instruction.

`window.hejViewer.creditLine` is exposed so the admin editor's preview renders through it. A preview that applied the
rule differently would be a preview of something else.

## Acceptance Criteria

- [x] The credit sheet shows the current credit, and which kind it is
- [x] No editor can turn a roster reference into typed text by accident
- [x] The reference still never reaches a response; only the kind does
- [x] No credit plate on the public grid, and no orphaned CSS
- [x] The viewer prefixes every credit with "Foto: ", exactly once
- [x] The editor's preview and the public page render the same sentence
- [x] Full gate clean: `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...`

## Progress Log

- 2026-09-30 — Diagnosed the empty crew list as self-inflicted before changing any code, which is why no "fix" was
  written for it.
- 2026-09-30 — Verified end to end against the dev container: set a roster credit through the API, confirmed
  `creditIsCrew: true`, `data-credit-crew="1"` on that cell only, and the bare name on the public page — then removed
  the test credit so the dev data is as it was.
- 2026-09-30 — Backtick trap in `publicsite.go` for the **seventh** recorded time, while deleting the very CSS whose
  comment carried the warning.
