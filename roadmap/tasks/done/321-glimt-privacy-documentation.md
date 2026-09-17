# 321 — Document Glimt in PrivacyView

**Status:** done
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:** agent
**Started:** 2026-09-17
**Completed:** 2026-09-17

## Description

PRD 019 §6. `/privatliv` must document Glimt honestly, in Danish, in language a 12-year-old and
their parent can both read:

- **What is stored** — the photos and videos, the caption, which hold posted it, when. Not the
  location: GPS/EXIF is stripped on upload.
- **Who can see it** — the three audiences, in plain words, and that **the Team section can see
  everything posted whatever the audience**. This is the disclosure the PRD insists on: a
  participant choosing "Min gruppe" is entitled to know that is not the same as "only my
  gruppe" (PRD 019 §6). It must be stated, not discoverable.
- **That no names are attached** — a glimt is signed with the patrulje, not a person.
- **How long it lives** — the **real configured retention** from `/api/config` (task 310), not
  a hard-coded sentence. A page that says "90 dage" while the deployment is set to 30 is worse
  than saying nothing.
- **How to get something removed** — the author can delete; anyone can report; reporting hides
  it from the public feed immediately.

## Acceptance Criteria

- [x] `PrivacyView.vue` has a Glimt section covering all five points
- [x] Team-section reach explicitly stated
- [x] Retention figure read from `/api/config`, not hard-coded
- [x] Danish, plain language, no legalese
- [x] `npm run test:unit` and `npm run build` pass

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
- 2026-09-17 — Done. The copy lives in `vue/src/components/glimt/glimtPrivacyCopy.ts`, rendered by
  `PrivacyView.vue`, with 15 tests.

  **Why a module and not paragraphs in the template.** Two of these sentences are not prose:

  - The **Team-section reach** is a disclosure PRD 019 §6 requires to be *stated, not
    discoverable*. A sentence in a template can be dropped by anyone rearranging the page, and
    nothing would fail — the reach is invisible by definition, so no amount of looking at the page
    would reveal it had gone.
  - The **retention window** carries a number that must match the deployment. It is only ever wrong
    on a deployment configured differently from the developer's, which is precisely the case nobody
    looks at.

  The unit suite runs in node and never mounts a component, so copy left in the view could not be
  asserted at all. `audienceChoice.ts` set this precedent for the composer; this is the same
  argument for the page a parent actually reads.

  Placed straight after the "Dit billede" section: both are about photographs, and a reader who has
  just been told their portrait stays inside Nathejk is exactly the reader who needs to know a glimt
  can leave it.

- 2026-09-17 — Copy decisions worth keeping:

  - **The Team sentence says who they are and why.** "Team kan se alle glimt" is true and useless;
    the page has room for "de voksne, der står for løbet", the reason (so a bad photograph can come
    down quickly), and the consequence spelled out — *"Vælger du 'min patrulje', er det altså din
    patrulje og Team, der kan se det."* It has to be unmissable **and** fair at the same time, or it
    reads as surveillance rather than as the thing that makes the safety mechanism work. Given its
    own tinted paragraph, because a sentence folded into a list is a sentence that gets skimmed.
  - **The location sentence is framed as what we do**, not as what we lack: the camera writes the
    place into the file and we cut it out. It is the most reassuring true thing on the page and the
    one a reader would never assume, since every other app they use keeps it.
  - **"Ingen, der ser det igennem først."** With no approval queue in front of the public scope
    (PRD 019 §0) this is the fact a parent most needs and would least expect, so it is said plainly
    rather than left to be inferred from "offentligt".
  - **Hiding and deleting are kept distinct.** The author's delete removes the files; a report
    hides. Calling both "sletning" would be the kind of simplification that surprises somebody
    later, and there is a test asserting the report sentence does not say "slettet".
  - **Second person throughout.** Their photographs, their pronoun. A page in the third person is a
    policy, not an explanation.

- 2026-09-17 — On the retention figure: `glimtRetentionCopy()` reads `glimtRetention` /
  `glimtPublicRetention` from `/api/config` (already wired by task 310) and returns **`''` when
  retention is switched off** — the view then drops the paragraph rather than rendering "0 dage" or
  "for altid". Retention off is a dev or test deployment, and silence beats inventing a window for
  somebody's photographs. When both windows are configured, the public one is stated first, because
  that is the order a parent asks: how long is it on the open web, then how long do you keep it at
  all.

  Two guards beyond the copy itself: a test asserting `PrivacyView.vue` still imports every
  constant (the module is only a guarantee if the view renders it), and one asserting the view
  hard-codes no `\d+ dage` of its own — the exact regression this task was written against, in the
  file most likely to reintroduce it. Also a test that the page's long form and the composer's
  five-word `TEAM_DISCLOSURE` both still name Team and "alle glimt", so softening one cannot leave
  the other standing alone.

  890 Vue tests, `type-check` and `build` clean.

  **Not reviewed by a human for tone**, and it should be: this is the only written account most
  parents will see, and I am not a native Danish speaker. The facts are right; the register is a
  judgement someone else should make. PRD §11 Q1 (the retention defaults) is also still open, so
  the *numbers* this page will show are unconfirmed — though that is now a config question rather
  than a copy one, which was the point.
