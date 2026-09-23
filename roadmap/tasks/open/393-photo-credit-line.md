# 393 — A credit line on every photograph

**Status:** open
**Priority:** high
**Created:** 2026-09-23
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] A credit can be set on a selection of any size, and cleared
- [ ] It is stored on the photograph, so every album shows the same credit — no per-album copy
- [ ] It renders on the public album page, and a photograph without one renders exactly as now
- [ ] The `uploaded` fold never overwrites it, so a re-upload does not wipe a credit — verified by test
- [ ] `isPersonShaped` allowlists `credit` **by name with reasoning**, and a `CreditPersonID`-shaped field
      still fails
- [ ] A test asserts the credit is never populated from the `person` projection — it is only ever typed
- [ ] The public privacy walk's claim is amended in `publicprivacy_test.go`'s header, not quietly contradicted
- [ ] PRD 022 §6's "no name" and PRD 011's "names no human being" are amended to state the exception and its
      bounds
- [ ] The browser remembers the last credit typed; the server remembers nothing about who typed it
- [ ] Not added to any glimt surface

## Notes

- Deliberately **not** a photographer *record*. No roster, no picker, no ids. If crediting the same six people
  every year becomes tedious, the browser's remembered default is the cheap fix and a roster is a decision to
  take separately — it would mean holding a list of volunteers' names in this service, which is exactly the
  thing §6 is arranged against.
- Worth deciding at review: whether the credit should also reach the **diploma** photograph (task 361) and the
  patrol page, once patrol tags surface publicly (§11 Q1). Out of scope here; noted so it is not re-derived.
