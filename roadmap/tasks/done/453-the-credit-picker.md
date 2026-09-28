# 453 — The picker in the "Fotokredit" sheet

**Status:** done
**Priority:** medium
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

## Description

PRD 025 §6 R2 and §7. Needs tasks 449 and 450.

The existing sheet gains a list of crew names — `pr` first, with a way to see all of the year's crew — above the
text field, which stays (PRD 025 §11 Q2: the typed path is a peer, for a guest photographer or one outside
`pr`).

**The copy has to distinguish the two paths.** Picking a colleague from a roster and typing a guest's name are
the same outcome by different routes, and only one of them can be misspelled. The sheet's existing sentence —
which tells the curator they are putting a name on a public page — stays; a curator picking from a list should
know that just as clearly as one typing.

The `localStorage` default (task 393) matters less now but must keep working for the typed path.

The server-rendered admin tool: Pico, htmx, Alpine, no build step. See the `go-server-rendered-pages` skill.

## Acceptance Criteria

- [ ] The list is offered, `pr` by default, all-crew available
- [ ] Picking sets the id; typing sets the text; each clears the other
- [ ] Copy that makes both paths and their consequence clear, in Danish
- [ ] Keyboard-reachable, like the tool's other sheets
- [ ] The typed path and its remembered default still work

## Progress Log

- 2026-09-28 — Task created from PRD 025 §6 R2, §7.

## What changed

**The write path.** `patchAdminPhotosRequest.CreditCrewID` and `setAdminPhotoCrewCredits`, a near-twin of the
typed setter and kept separate for the same reason that one is kept separate from the caption's: they differ in
what they log and in their Danish, and a shared helper taking a field name would be a write path that names its
own column from a parameter.

- **The id is logged, never the resolved name.** "Who was credited on which photographs, and when" is the
  question somebody may have to answer later (PRD 022 §8.2), and an id answers it. A name in the log is a name
  that can never be erased — the one place this feature's whole argument has no answer.
- **No roster check on the way in.** An id this app cannot resolve is not refused: `person.CreditNames` yields
  nothing for anybody who is not crew that year, so an unusable reference renders no credit line — the same
  outcome and the same code path as a crew member who has since been deleted. A check here would be a second
  opinion about who is creditable, and the picker offering one answer while the reader gives another is the
  failure that invites.
- **Refused rather than truncated** above 99 runes, the opposite of the typed credit: a shortened name is still
  recognisably a name, while a shortened id is a *different* id — one that resolves to nobody, or to somebody
  else.

**The sheet**, in the two-choice shape the delete sheet already uses, so a curator can see at a glance that
picking and typing are different acts with the same outcome. Picking comes first. A searchable listbox (filtered
in the browser — a round trip per keystroke would make the search feel worse than scrolling), a "vis hele crewet"
checkbox, and the typed field kept below with its consequence stated.

The roster is fetched **when the sheet opens**, not on page load: most sessions never open it, and `/admin` is
served `no-store` (task 371), so a roster fetched on load would be a request nobody asked for on every page view.
Held between cards, re-fetched when the section changes. A failed fetch is not fatal — it says so and the typed
field still works.

Only a typed credit is remembered in `localStorage`. A picked one needs no default, because the list *is* the
default, and an id in browser storage would be a person reference sitting there for no reason.

## The copy

> **Vælg fotografen** — Listen er dette års crew — PR først, for det er der fotograferne plejer at være. Vælger
> du herfra, staves navnet altid ens, og beder fotografen sig selv slettet, forsvinder navnet fra alle billederne
> på én gang.
>
> **Eller skriv et navn** — Til en gæstefotograf, eller en der ikke er i crewlisten. Skrevne navne bliver
> **ikke** rettet automatisk, hvis fotografen senere beder om det — så vælg fra listen, når du kan.

The second block's second sentence is the one worth defending. The two routes produce an identical public page, so
nothing in the result tells a curator that one of them is unfixable later. That has to be said where the choice is
made, or it is discovered when somebody asks to be removed.

The existing "hvor alle kan læse det" warning was kept and moved up, so it covers both routes rather than only
the typed one.

## A guard matched its own explanation, for the fifth time in this package

`TestACreditNamesAPhotographerAndNobodyElse` failed the moment the crew setter landed: its needle `"Name"` found
`person.CreditNames` in a doc comment saying where resolution belongs. Fixed the documented way — strip comments
before searching — and the slice now covers **both** setters, which is a stronger assertion than it had: neither
way of setting a credit may look anything up.

## Acceptance Criteria

- [x] The list is offered, `pr` by default, all-crew available
- [x] Picking sets the id; typing sets the text; each clears the other (the fold does the clearing — task 450)
- [x] Copy that makes both paths and their consequence clear, in Danish
- [x] Keyboard-reachable: a native `<select>` and a native `<input type="search">`, no custom widget
- [x] The typed path and its remembered default still work

## Progress Log

- 2026-09-28 — Picked up. Wrote the write path first, because the sheet is only useful once there is something to
  send.
- 2026-09-28 — Chose the delete sheet's two-`.choice` shape over one field with a mode toggle: the task asks for
  the routes to be distinguishable, and this tool already has a pattern for "two ways, different consequences".
- 2026-09-28 — Decided against validating the id against the roster on write; see above.
- 2026-09-28 — ✅ Five tests: the event carries the reference and no name, both forms at once is a 400, an
  over-long id is a 400, the sheet offers both routes with the two sentences that distinguish them, and the tool
  sends the id rather than the name. Mutation-checked the mutual-exclusion count.
- 2026-09-28 — `gofmt`, `go vet`, `staticcheck`, `GOWORK=off go test ./...` clean.
