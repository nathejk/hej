# 393 — A credit line on every photograph

**Status:** done
**Priority:** high
**Created:** 2026-09-23
**Picked up by:** agent
**Started:** 2026-09-23
**Completed:** 2026-09-23

## Description

Maintainer:

> *"all photos should have the possibility of a credit line"*

A photographer's attribution — *"Foto: Anne Sørensen"* — settable on any photograph and shown with it on the
public album page. Optional ("the possibility of"), so a photograph without one renders as it does today.

The mechanics are small: `credit` mirrors `caption` exactly, which already runs the whole length of this
feature. The reason this has a task rather than being a two-line change is that **it amends a documented
privacy claim**, and that has to be deliberate.

## What it changes, and why it is still safe

`cmd/api/publicprivacy_test.go` opens by stating the claim the whole public surface rests on:

> *The entire privacy claim of the public frontpage is that **it names no human being**. A patrol is a patrol —
> a number, a name, a gruppe, a korps — and its route is the patrol's route. That is what makes publishing a
> merged track defensible (PRD 011 §0b.1), and it is why per-member tracks are forbidden rather than deferred.*

And PRD 022 §6: *"The library row has no uploader, no curator, no person id, no name, no phone number, and
emphatically no `phoneParent`."*

A credit line is a human being's name, on a public page, in the library row. So the claim becomes narrower
rather than absolute, and the narrowing is the whole design:

| | |
|---|---|
| **Who it names** | a consenting adult volunteer, in their professional capacity, because they asked to be credited. Not a participant, not a minor, not somebody who never agreed to be in this app. |
| **Where it comes from** | free text a curator **typed**. Never derived, never looked up, never joined to the `person` projection. |
| **What it is not** | a `personId`, a link, a lookup key, or anything a second feature could resolve into a person record. |

That last row is the safety property that matters. The hazard was never "a name appears on a page" — it is *a
system that starts deriving names from its person records and putting them on public pages.* A string somebody
typed cannot do that, and a test should assert it stays one.

**The shared credential makes this cleaner, not harder.** PRD 022 §8.2 says the tool cannot honestly attribute
anything to a person, which is why there is no uploader column. A credit line does not contradict that: it is
**editorial text about the photograph**, not a claim about who operated the tool. A curator can credit a
photographer who never touched this app.

## The guard must document the exception, not miss it

Checked: `isPersonShaped` would let `Credit` and `CreditLine` through **by luck** — neither matches any needle,
and neither ends in "By". That is the worst available outcome: the one field in the whole projection that
intentionally carries a person's name would be the one field the guard says nothing about.

So:

- add `credit` to `isPersonShaped`'s **allowlist by name**, with the reasoning, exactly as `photoId`, `photos`,
  `photoDeleted` and `coverPhotoId` were (tasks 381, 391). The guard then *documents* the exception rather than
  being silent about it, and a future `CreditPersonID` still fails.
- the field is named `Credit`, not `Photographer` — which the needle *would* catch, and which would then need
  excepting for a worse reason. `Credit` is also the honest word: it is a line of text, not an identity.

## Design

Mirrors `caption` at every layer, because that is a field with exactly the same shape — curator-authored free
text, on the photograph rather than on an album membership (PRD 022 §8.3), so one place to edit and every album
shows the same thing.

1. **Schema.** `credit VARCHAR(160) NOT NULL DEFAULT ""` on `photo`.
2. **Event.** `Credit *string` on `photo.Updated`, a pointer so "not mentioned" and "cleared" stay different —
   the same reason `Caption` is one. The `uploaded` fold must **not** touch it, like `caption` and `deleted`.
3. **Curator API.** `Credit *string` on `patchAdminPhotosRequest`, so it is settable over a **selection**. This
   matters more than it looks: a card is one photographer, so three hundred photographs take one action.
4. **UI.** Part of the existing bulk edit. A remembered default in the browser's `localStorage` so a
   photographer does not retype it per batch — in the **browser**, deliberately, because the credential is
   shared and a server-side "my credit" would be an attribution the tool cannot honestly make.
5. **Public.** Rendered under the photograph on the album page. Not on the frontpage cards.
6. **Not on glimt.** Those are participants' photographs, attributed to a patrol, and PRD 019 forbids naming.

### Length

160 characters. Long enough for *"Foto: Anne Sørensen / Nathejk"*, short enough that it cannot become a
paragraph — a credit line that can hold prose will eventually hold prose, and the caption is where prose goes.

## Acceptance Criteria

- [x] A credit can be set on a selection of any size, and cleared
- [x] It is stored on the photograph, so every album shows the same credit — no per-album copy
- [x] It renders on the public album page, and a photograph without one renders exactly as now
- [x] The `uploaded` fold never overwrites it, so a re-upload does not wipe a credit — verified by test
- [x] `isPersonShaped` allowlists `credit` **by name with reasoning**, and a `CreditPersonID`-shaped field
      still fails
- [x] A test asserts the credit is never populated from the `person` projection — it is only ever typed
- [x] The public privacy walk's claim is amended in `publicprivacy_test.go`'s header, not quietly contradicted
- [x] PRD 022 §6's "no name" and PRD 011's "names no human being" are amended to state the exception and its
      bounds
- [x] The browser remembers the last credit typed; the server remembers nothing about who typed it
- [x] Not added to any glimt surface

## What landed

`credit` mirrors `caption` at every layer — schema, `photo.Updated`, the fold, the curator's bulk patch, the
curator's read, the album join, the public template — because it is a field of exactly the same shape:
curator-authored free text on the **photograph** rather than on an album membership (PRD 022 §8.3), so one place
to edit and every album shows the same thing.

Settable over a **selection**, which is the requirement rather than a convenience: a memory card is one
photographer, so three hundred photographs take one action. Clearing is its own button rather than "save an
empty field", so removing somebody's attribution is a deliberate act and not something a stray select-all does
on the way past.

On the public page the credit is its own quieter line **under** the caption, not appended to it: the caption is
what the photograph is *of* and the credit is who took it, and one sentence containing both reads as though the
photographer were part of the scene. A photograph with a credit and **no** caption still gets a `figcaption` —
the template condition is `or .Caption .Credit`, and that case is why.

### The privacy claim, narrowed on the record

`publicprivacy_test.go`'s header used to say the surface *names no human being*, full stop. It now states the
one exception and, more importantly, what the claim really was all along:

> What this file has always really defended is not the absence of characters that spell a name — it is that
> **this service does not take names out of its person records and put them on public pages.**

A curator typing an attribution does not do that. A lookup would, and is the thing to keep failing. PRD 022 §6
and PRD 011's privacy bullet carry the same amendment rather than being quietly contradicted.

### The guard had to be *given* the needle

The important detail. `Credit` matched no existing needle and does not end in "By", so it would have passed the
structural walk **in silence** — the one field in the service that intentionally names a person would have been
the one field the guard said nothing about.

So `credit` was added to the needles *and* the exact name excepted. The pair is the point: the exception is now
written down in the guard, and relatives still fail. Proven by break-test — with the needle removed a
`CreditName` field passes unnoticed; with it present, it fails.

`CreditPersonID` fails on `person` regardless, and the field is named `Credit` rather than `Photographer`
precisely because the latter trips the `photo` needle and would then need excepting for a worse reason.

### Why the remembered default is in the browser

A card is one photographer, so without something remembered every batch means retyping the same line. It lives
in `localStorage` and **not** on the server, deliberately: the credential is shared (§8.2), so a server-side "my
credit" would be an attribution the tool cannot honestly make. A browser remembering what *this laptop* last
typed claims nothing about who is using it.

It is prefilled from that, **not** from the selection — the photographs may carry different credits, and showing
one of them would be a guess that silently overwrites the others when the button is pressed.

### The credit is logged, where the caption is not

`setAdminPhotoCaptions` keeps its text out of the log on purpose: curator prose, nothing an operator would use.
The credit is the opposite. It is the one field here that names a person, so *"who was credited on which
photographs, and when"* is exactly the question somebody may have to answer later — a photographer asking to be
uncredited, or a mis-typed attribution on a public page. With a shared credential the log is the only record
there is.

### Bounds

`VARCHAR(160)`, refused at the edge with a Danish reason and **truncated by rune** in the fold as a safety net.
Not duplication: a curator who pasted a paragraph should be told, while a credit that somehow got past the API
and overflowed its column would deadletter on every replay — the failure task 352 records on `postalCode`.
Truncation is by rune because a Danish name is not ASCII and slicing UTF-8 by byte can leave half a character.

Tight where the caption's limit is generous: a field that can hold prose will eventually hold prose, and the
caption is where prose goes.

## Verified by breaking it

| Sabotage | Caught |
|---|---|
| the `credit` needle removed | a `CreditName` field then passed **unnoticed** — 0 failures with it gone, 1 with it present |
| a `CreditPersonID` field added | flagged on `person`, as it must be |
| the credit derived from `app.models.People` | *"a credit must never be looked up in the person projection"* |
| `credit=VALUES(credit)` added to the upload fold | *"would blank every attribution"* |
| rune truncation replaced with a byte slice | the 160-rune assertion failed |

## Verified live

- One request set one credit on a **two-photograph selection**: `Fotokredit sat på 2 billeder.`
- The public album page renders both cases: `<figcaption>Ved posten, lige før midnat<span
  class="credit">Foto: Vibeke Krogh</span></figcaption>`, and a credit-only `figcaption` on the uncaptioned
  photograph.
- **Re-uploading a credited photograph** — the documented recovery procedure — answered `Allerede uploadet.`
  and left the credit intact.
- Both credits survived a **full replay** after a container restart.
- The credit reaches the curator's library read, and the sheet carries `data-act="credit"`, the panel, and the
  `hej.admin.lastCredit` key.

## Notes

- **A migration was needed and the suite could not tell me.** `table.sql` is `CREATE TABLE IF NOT EXISTS`, so
  adding the column there does nothing to a database that has already booted — and every test in `cmd/api` runs
  against stubs that never touch a real schema. The suite was green and the dev database simply had no `credit`
  column. Fixed with `cqrs.EnsureColumn` in `photo/table.go`, following `checkpoint` and `person`, and found
  only because the live check looked at `describe photo`. **Any new column on an existing projection needs both
  halves**, and only the live check will say so.
- **I lost work with `git checkout`.** Two break-tests reverted their sabotage with `git checkout <file>` on
  files whose credit changes were **uncommitted**, discarding the real work along with the sabotage —
  `adminlibrary.go` and `adminposition.go` both had to be rewritten. Use a `/tmp` copy to revert a sabotage,
  never `git checkout`, unless the file is committed. The earlier break-tests in this session did it correctly;
  these two did not.
- **Deliberately not built:** a photographer roster or picker. That would mean holding a list of volunteers'
  names in this service, which is what §6 is arranged against. The remembered default is the cheap fix.
- Still open: whether the credit should reach the **diploma** photograph (task 361) and the patrol page once
  patrol tags surface publicly (§11 Q1). Noted so it is not re-derived.
