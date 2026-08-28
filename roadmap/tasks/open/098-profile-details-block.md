# 098 — Frontend: read-only details block (`Mine oplysninger`)

**Status:** open
**Priority:** medium
**Created:** 2026-08-28

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

- [ ] Section renders as a definition list with Danish labels.
- [ ] `tel:` links on both phone numbers, with Danish formatting on the label.
- [ ] Hidden-row vs "Ikke registreret" rules implemented as described.
- [ ] A short line telling the user how to get details corrected (copy may be
      provisional — PRD 003 §11 "Editability" is still open; do not invent a
      phone number or email).
- [ ] Loading and error states rendered, not blank.
- [ ] Unit test for the null/empty guardian-number distinction and the phone
      formatter.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
