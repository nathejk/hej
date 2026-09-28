# PRD 025 — Photo credit by crew member

**Status:** doing
**Author:** agent session (2026-09-28)
**Created:** 2026-09-28
**Last updated:** 2026-09-28 (449–453 and 455 done; only 454, the credit filter, remains — low priority)
**Approved:** 2026-09-28
**Shipped:**
**Target users:** organizer (the photographers and curators who use `/admin`)

---

## 1. Summary

A photograph's credit becomes **either a reference to a crew member or a typed string**. The curator picks a
photographer from the crew roster — defaulting to the `pr` section, where most of them are — instead of typing
a name onto hundreds of photographs.

## 2. Problem & Motivation

**What problem does this solve?** Photographers are insistent about being credited for their work, and the
current credit is free text. It is already a bulk action with a `localStorage` default, so nobody types a name
three hundred times — but nothing makes it *consistent*. "Anne Sørensen" on Saturday's card and "Anne Sorensen"
on Sunday's, from a different laptop or a different curator, are two photographers as far as the public page is
concerned. And today a misspelling is nearly unfixable: there is no way to **find** photographs by credit.

**Why now.** PRD 024 has just made the library's bulk workflow good enough that a card becomes an album in one
sitting. The credit is the remaining hand-typed field in that flow, and it is the one that is published.

**Evidence.** The maintainer, 2026-09-28: *"adding creditlines for this many photos increases the risk of
misspelling a name"*, and *"photographers are very insisting on being credited for their work"*.

## 3. Goals

- A credit can be **picked**, not typed, so the same photographer is spelled the same way on every photograph.
- A photographer who is not in the roster can still be credited.
- A crew member who asks to be removed is removed **in one place**, and disappears from every photograph.
- No participant's name can be published by this mechanism, however it is misused.

## 4. Non-Goals

- **A credit for anyone but a photographer.** Not a curator, not an uploader. PRD 022 §8.2 stands: the
  credential is shared, so the tool cannot attribute its own actions to a person.
- **Crediting several people per photograph.** One credit, as today. If two photographers want joint billing,
  typed text says so.
- **Changing what the public page shows.** It renders a name under a photograph, exactly as now.
- **Exposing the roster publicly.** The picker is behind the admin credential.

## 5. User Stories & Scenarios

- As a **curator**, I want to pick the photographer from a list so that their name is right without my having to
  know how it is spelled.
- As a **photographer**, I want my name under my photographs, spelled the same way every time.
- As a **crew member**, I want one place to be deleted from, and to be gone from the public site when I am.

**Happy path.** The curator selects 120 photographs from a card, opens "Fotokredit", and picks *Anne Sørensen*
from a list of this year's `pr` crew. The public album page credits *Foto: Anne Sørensen* under each one.

**Not in the roster.** A guest photographer, or a section that is not `pr`. The sheet still takes typed text, and
the list offers all of the year's crew rather than only `pr` — `pr` is the default filter, not the boundary.

**Erasure.** A crew member asks to be removed. Their person record is deleted in the one place that holds it.
Every photograph they took stops carrying a name within the public page's 60-second cache window. Nothing has to
be found, rewritten or re-published, and the photographs themselves are untouched.

## 6. Requirements

### Functional

- [x] **R1 — Two ways to credit, one in force.** `photo.creditPersonId` (nullable) alongside the existing
      `credit` text. Setting one **clears** the other: a photograph has one credit, and two fields that could
      both be set is a photograph with two answers about who took it.
- [x] **R2 — The picker.** An admin-only read of the year's crew, defaulting to section `pr` and able to show
      all crew. Names only — no phone number, no email, no portrait, nothing else the person row holds.
- [x] **R3 — Resolution is narrow, and this is the load-bearing requirement.** A credit id resolves to a name
      **only** when the person classifies as `person.RoleCrew`. A spejder's or a bandit's id resolves to
      nothing, so a mistyped, stale or malicious id cannot publish a participant's name. Only the name field is
      ever read.

**Widened on 2026-09-28**: `person.CrewRoles` — crew, guide, samarit and postmandskab — not `RoleCrew`
      alone. A photographer registered under `guider` would otherwise have been invisible to the picker, which is
      the case this feature exists to remove. Gøglere, banditter and spejdere stay out, so no participant's name
      can be published under either reading.

      **Within the photograph's own year** (§11 Q1), with no fallback to another. That is the same rule every
      other read in this service follows — nothing crosses a year — and it makes the resolver a two-key lookup
      with no "which row wins" question in it. The consequence is worth stating: somebody who was crew in 2026
      and is not in 2027 still has their 2026 photographs credited, because those resolve against 2026; and an
      id that is not crew *in that year* renders no credit, by R5.
- [x] **R4 — Erasure works by deletion.** Deleting the person record removes the name from every photograph and
      every public page. Nothing in the library or on the event log has to be rewritten — which is the whole
      reason this is a reference and not a copied string (§8 D1).
- [x] **R5 — Absence renders as no credit.** An id that resolves to nothing produces no credit line: no
      placeholder, no empty label, no gap. A photograph with no credit already renders that way.
- [x] **R6 — The public response carries the name, never the id.** The id is a handle to a person record and
      has no business on the open web, even though it names nobody by itself.
- [ ] **R7 — A credit filter in the library** (lower priority; §11 Q3). Its original justification is gone: the
      feature has not been used yet, so there are no misspelled credits to repair. It stays because "which
      photographs are credited to X" is a question that will be asked — to re-credit a batch, or to check a
      photographer's own request — and today there is no way to ask it. Not a blocker for the rest.

### Non-Functional

- **This reverses a written decision, and the reversal has to be recorded, not just implemented.** PRD 022 §6
  admits the credit line as the one field naming a human being *because* it is "free text a curator typed.
  Never derived, never looked up, never joined to the `person` projection", and says "emphatically not a
  `creditPersonId`". `TestACreditIsOnlyEverTypedNeverDerived` enforces it, and `isPersonShaped` deliberately
  still rejects `CreditPersonID`.

  The maintainer's argument for reversing it is **erasure**, and it is stronger than the argument for the rule:
  a name copied into `photo.credit` is also copied onto the append-only event log, so a deletion request could
  never be fully honoured. A reference can be. PRD 022 §6 must be rewritten to say this, and the guard
  re-scoped from "the credit is never derived" to R3's narrower and still meaningful rule.

  The two premises the old rule rested on were also corrected: crew are **consenting adults**, not
  participants; and crew names are **not** purged after the event — the same `crewMemberId` is reissued the
  following year.

- **Privacy.** The tool displays crew names; it already holds them. No participant name becomes reachable, by
  R3. No new personal data enters the library: the id is pseudonymous and the name stays in the one projection
  that owns it.
- **Performance.** The public album page resolves credits in the read that already fetches its photographs, and
  that page is cached for 60 seconds. The picker is one indexed read — `KEY year_section (year, sectionSlug)`
  already exists.
- **OpenAPI** annotations for the new roster read and the changed credit write.

## 7. UX / UI Notes

The admin tool only (`go/cmd/api/adminui/`) — server-rendered, Pico/htmx/Alpine, no build step. Nothing in
`vue/`.

The existing "Fotokredit" sheet gains a list of crew names above the text field: `pr` first, with a way to see
all of the year's crew. The text field stays, and the copy has to make the two paths distinguishable — picking a
colleague from a roster and typing a guest's name are the same outcome by different routes, and only one of them
can be misspelled.

The sheet's current copy already tells the curator they are putting a name on a public page; that stays. The
`localStorage` default becomes less important but should keep working for the typed path.

## 8. Technical Considerations

**D1 — Why a reference and not a resolved string.** This was the one real disagreement, and it is worth
recording because the losing argument was mine. Storing the resolved *name* would have been purge-safe and
would have kept the public read free of any join. It fails on erasure: the event log is append-only and is
never rewritten, so a name copied onto it at upload time is permanent, and "delete me" could not be honoured.
A reference keeps the name in exactly one place. The cost is that the public page now depends on a join, which
R3 bounds and R5 makes safe when it finds nothing.

**Data.** `photo.creditPersonId VARCHAR(99) NOT NULL DEFAULT ""`, plus the existing `credit`. An event field on
`photo.Updated` to match, with the same pointer semantics as the rest of that event — and a rule that setting
one clears the other, enforced in the fold as well as the handler.

**Reads.** The public album read and the admin library read resolve the id. One resolver, in one place, so R3
is one function to audit: *given an id and a year, return a crew member's name or "".*

**Which year's row resolves a credit** is the open question — see §11 Q1.

**Guards to change, deliberately and by name:**
- `TestACreditIsOnlyEverTypedNeverDerived` → becomes a test of R3: the resolver reads only `name`, only for
  `RoleCrew`, and yields "" otherwise.
- `isPersonShaped`'s rejection of `creditpersonid` → excepted **for the library scope only**, exactly as
  `filename` was in task 448, so a *public* response carrying the id still fails (R6).
- `libraryPersonShapedExceptions` gains a third entry with its own written argument.

**Risks.** The public site starts rendering something derived from the person projection. R3 is what keeps that
from becoming "the public site can render any person's name": it must be a single narrow function with its own
tests, not a general-purpose lookup that later grows a second caller.

## 9. Success Metrics

- Every 2026 photograph's credit is either a picked crew member or a deliberately typed string.
- No photographer's name appears in two spellings in one year's library.
- A test proves a participant id cannot produce a public name.

## 10. Rollout / Task Breakdown

Created on the board 2026-09-28:

- [x] Task 449: the crew roster read for the picker (`pr` default, all-crew option), OpenAPI
- [x] Task 450: `photo.creditPersonId` — column, event field, fold, and the one-in-force rule
- [x] Task 451: the resolver — crew-only, name-only, within the year, "" when absent — and the rewrite of
      `TestACreditIsOnlyEverTypedNeverDerived`. **The load-bearing one**
- [x] Task 452: the public and admin reads use it; the public response carries the name and never the id
- [x] Task 453: the picker in the "Fotokredit" sheet, with copy for the two paths
- [x] Task 455: rewrite PRD 022 §6's credit bullet to record the reversal and its three reasons
- [ ] Task 454: a credit filter in the library — **low priority**, since there is nothing to repair yet

## 11. Open Questions

All three answered by the maintainer on 2026-09-28.

1. **Which year's person row resolves a credit?** ✅ *"the year of the photo resolves the year."* No cross-year
   fallback — see R3. Tighter than the alternatives, and consistent with everything else here being
   year-scoped.
2. **Should the typed path stay, or be a fallback only?** ✅ Stays a peer of the picker. A guest photographer, or
   one outside `pr`, is credited by typing.
3. **What about the credits already in the library?** ✅ *"we have not started using the feature yet, no names to
   recover."* No migration, no matching pass. R7 demoted accordingly.

---

## 12. What shipping it taught us

Added on completion.

1. **The rule written to protect people was the weaker option, in the case that mattered most to them.** PRD 022
   §6 forbade a `creditPersonId` so that no name would be *derived*; it also meant every credit was a name copied
   onto an append-only log, where an erasure request has no answer. The maintainer saw that and I did not. It is
   worth remembering as a shape: a privacy rule phrased as "never do X" can be worse than a bounded X, and the
   test for which is usually "what happens when somebody asks to be removed".

2. **Scope an exception to the narrowest thing that makes it true.** `creditCrewId` is excepted for the library,
   not for the word — so a public response carrying it still fails. `album.Item` is excepted, not the field name —
   so `publicAlbumItem`, the type a template renders, still fails. Both exceptions left the guards *stronger* than
   they were, because each one forced a boundary to be named and tested that had previously been implicit.

3. **A guard that asserts the opposite of the feature must be rewritten, not deleted.**
   `TestACreditIsOnlyEverTypedNeverDerived` became `TestACreditNamesAPhotographerAndNobodyElse`. Deleting it would
   have discarded three things that are still true; leaving it would have blocked a decision the maintainer had
   made. Neither is a good outcome, and rewriting it took ten minutes.

4. **`RoleCrew` is not "crew".** It is one of four roles that all belong to crew members, and the picker was
   initially built against the narrow reading — which would have made a photographer registered under `guider`
   invisible to the very feature meant to stop names being typed by hand. `person.CrewRoles` and `IsCrew` now name
   the union once, replacing a copy in `scansource.go` and a paragraph of prose in `internal/users`.

5. **A dangling test name in a comment is a privacy claim nobody can verify.** Four comments named the old guard
   after it was renamed. The next reader looks for the guarantee, cannot find it, and concludes there isn't one.
