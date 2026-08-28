# 098 — Frontend: read-only details block (`Mine oplysninger`)

**Status:** done
**Priority:** medium
**Created:** 2026-08-28
**Picked up by:** agent (Zed / Claude Opus 5)
**Started:** 2026-08-28
**Completed:** 2026-08-28

## Description

PRD 003 §6/§7: a labelled, clearly non-editable list of `Navn`, `Adresse`,
`Telefon`, `Forælders telefon`, fed by `profile.store` (task 096), plus a
footnote on how to get a correction made.

Two distinct empty states, matching the BFF (task 094):

- `phone_parent: null` — this role has no guardian number: **hide the row**.
- `phone_parent: ""` — one is expected and missing: show `Ikke registreret`.

Phone numbers are `tel:` links, formatted for Danish numbers (the value stays
normalized in the `href`; only the label is prettified).

Address renders as one block from `address` + `postal_code` + `city`.

## Acceptance Criteria

- [x] Section renders as a definition list with Danish labels.
- [x] `tel:` links on both phone numbers, with Danish formatting on the label.
- [x] Hidden-row vs "Ikke registreret" rules implemented as described.
- [x] A short line telling the user how to get details corrected, naming no
      invented contact channel (PRD 003 §11 "Editability" is still open).
- [x] Loading and error states rendered, not blank.
- [ ] Unit test for the null/empty guardian-number distinction and the phone
      formatter — **not done: this repo has no unit-test runner** (no vitest, no
      `test:unit` script). Behaviour was instead exercised by hand against the
      formatter, and the BFF side of the null/empty distinction *is* covered by a
      Go test (task 094). Adding a frontend test runner is its own task, not a
      side effect of this one.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
- 2026-08-28 — Rendered as a `<dl>`, not a form of disabled inputs: the fields are
  read-only, and disabled inputs invite people to try to edit them.
- 2026-08-28 — `formatPhone` added as `helpers/phone.ts`. It formats the **label
  only** and leaves the `tel:` href on the normalized E.164 value — a prettified
  href is what makes a dialer occasionally refuse a number. Non-Danish and
  unparseable input is returned unchanged rather than forced into 2-2-2-2 groups: a
  wrong-looking number the user recognises beats a plausible-looking wrong one.
- 2026-08-28 — Checked the formatter's cases in the `ui` container:
  `+4530000001`/`30000001`/`+45 30 00 00 01` → `30 00 00 01`; `+46701234567`,
  `abc` and blank pass through untouched.
- 2026-08-28 — Guardian row: hidden when `phoneParent === null` (this population
  has none — showing an empty row would imply something is missing) and shown as
  "Ikke registreret" when it is `''`. Note the template tests `!== null`
  explicitly rather than truthiness, which would have collapsed the two states.
- 2026-08-28 — Address is one block from three fields, with its own `hasAddress`
  guard: crew are seeded without one and the app has no reason to show one, so
  empty is normal here rather than an error.
- 2026-08-28 — ✅ `npm run type-check` clean, `npm run build` succeeds. Moving to
  done with the test criterion explicitly unmet and explained.
