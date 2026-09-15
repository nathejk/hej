# The `kort` events — the printed map sheets, over the stream

> **This is a copy, taken 2026-09-15, of `roadmap/api/kort-events.md` in the `hq`
> repo.** It is vendored here on purpose: the only thing this repo shares with hq
> is the **stream**, and a contract we consume must be readable without a second
> checkout on disk. Nothing here is imported from hq — see PRD 016 §8.
>
> **hq owns the contract.** If something below needs to change, it changes in hq
> first (against its PRD 010) and this copy is refreshed afterwards. Do not edit
> this file to describe what hej does — that belongs in PRD 016.
>
> Refresh by copying the file again and noting the date above. The text below is
> verbatim, including its own header, which is written from hq's point of view.

---

**For:** the hej-app team (separate repo)
**Owner:** hq (`go/nathejk/table/kort`)
**Introduced by:** PRD 010
**Last updated:** 2026-09-03

HQ now records the sheets we print and hand out during the event: what each is
called, which set it belongs to, what ground it shows, and — the part you want —
**which checkpoints are drawn on it**.

You get this by **subscribing to the stream and building your own projection.**
Cross-service communication is over JetStream; HQ's REST endpoints serve HQ's own
SPA and are not an integration point. That is also the right answer for you on a
race night: a read model you own keeps answering while HQ is restarting,
redeploying or unreachable, whereas polling would make every reveal depend on HQ
being up at that moment.

> **Prerequisite:** the message types still live inside the hq repo
> (`go/nathejk/table/kort/messages.go`, `kortsaet_messages.go`). They are being
> lifted to `nathejk/shared-go` under **task 138** — you cannot decode these events
> until that lands. The shapes below are what will be lifted, unchanged.

---

## Subjects

```
NATHEJK.{year}.kort.{kortId}.created
NATHEJK.{year}.kort.{kortId}.updated
NATHEJK.{year}.kort.{kortId}.deleted
NATHEJK.{year}.kort.sorted

NATHEJK.{year}.kortsaet.{kortsaetId}.created
NATHEJK.{year}.kortsaet.{kortsaetId}.updated
NATHEJK.{year}.kortsaet.{kortsaetId}.deleted
NATHEJK.{year}.kortsaet.sorted
```

`{year}` is the year slug. The entity id is in the subject and is authoritative:
bodies also carry it, but where they disagree the subject wins — that is what the
stream routed on.

The two `…sorted` subjects carry **no id**, because they are about the collection.

---

## Payloads

### `kort.{id}.created`

```json
{ "kortId": "kort-…", "kortsaetId": "kortsaet-…", "name": "Kort 1" }
```

Only the set and the name. A sheet is *described* after it exists — an operator
adds "Kort 3" before knowing its format or drawing its area — so do not expect a
complete sheet here, and do not wait for one.

The set may not exist yet. Events arrive in stream order and a sheet can precede
its set, so tolerate an unknown `kortsaetId` rather than dropping the row.

### `kort.{id}.updated` — **a patch, not a snapshot**

```json
{ "kortId": "kort-…", "name": "Kort 1 — Start til Post 2" }
```

```json
{ "kortId": "kort-…", "checkpointIds": ["cp-1", "cp-2"] }
```

Every field is optional, and **an absent field means "unchanged"** — not "empty".
Fields: `kortsaetId`, `name`, `format`, `note`, `sortOrder`, `checkpointIds`,
`extents`, `handoutCheckgroupId`.

This is the thing most likely to be got wrong: treating the body as a full record
will blank the fields it does not mention. The checkpoint picker and the sheet's
description are separate screens that save separately, so partial updates are the
normal traffic, not an edge case.

An explicitly empty array **is** a change: `"checkpointIds": []` clears the sheet,
and `"extents": []` makes it a skitse. So is an explicitly empty **string** in
`handoutCheckgroupId` — see §1.1.

### `kort.{id}.deleted`

```json
{ "kortId": "kort-…" }
```

The sheet goes; its checkpoints do not — they exist independently and are almost
certainly on another sheet too.

### `kort.sorted`

```json
{ "kortIds": ["kort-a", "kort-b", "kort-c"] }
```

Position in the list is the sheet's `sortOrder` — handout order along the route,
meaningful within a set. **Ids not named keep their current order**, so this is not
a full ordering of the year.

### `kortsaet.{id}.created` and `kortsaet.{id}.updated` — **whole record**

```json
{ "kortsaetId": "kortsaet-…", "name": "Patruljer", "teamType": "patrulje" }
```

```json
{ "kortsaetId": "kortsaet-…", "name": "Crew" }
```

Unlike a sheet's `updated`, a set's carries its **whole** editable state: name and
team type together. So an **absent or null `teamType` means the set has none** — it
does not mean "unchanged". The asymmetry is deliberate: under patch semantics,
clearing a team type and not mentioning it would be the same event, and un-marking
the spejder set would be inexpressible.

### `kortsaet.{id}.deleted`

```json
{ "kortsaetId": "kortsaet-…" }
```

Only ever published for a set with **no sheets** — HQ refuses to delete a set that
still holds any — so there is no cascade to apply.

### `kortsaet.sorted`

```json
{ "kortsaetIds": ["kortsaet-a", "kortsaet-b"] }
```

Same semantics as `kort.sorted`.

---

## Semantics you cannot infer from the payloads

### 1. There are two reveal units, not one

The most important thing here, and the thing most likely to be implemented as one
rule.

- **A sheet's QR is linked or scanned** → that sheet's `checkpointIds` become
  visible.
- **A checkpoint is scanned** → its whole **checkgroup** becomes visible.

They do not nest. A skitse shows a subset of one checkgroup; a double-sided A3
spans two. Neither rule can be derived from the other, and HQ implements neither —
it records the facts and leaves the policy to you.

**A skitse has no QR code at all.** It is a hand-drawn slip handed over at a post
and is never scanned, so its checkpoints are revealed off the *previous*
checkpoint's scan. Its `checkpointIds` are the only trace of it in the system,
which is why sheets with no area and no QR still matter. Its `format` is
`"skitse"`.

### 1.1 `handoutCheckgroupId` tells you *which* rule applies to a sheet

Added after the two rules above were written, and it is what removes the guesswork:
the organizer now records where each sheet is handed out, because they plan it.

```json
{ "kortId": "kort-…", "handoutCheckgroupId": "cg-…" }
```

- **A checkgroup id** → the sheet is handed out at that group's post, so its
  `checkpointIds` become visible when that checkgroup is reached. This is how a
  skitse works, and it is now stated rather than inferred from "the previous
  checkpoint".
- **`""` (empty string)** → the QR rule: the sheet's checkpoints become visible when
  its own QR code is linked to or scanned by the team.

Three things to get right:

1. **Empty is a value, not a missing field.** It means the QR rule, which is also
   the behaviour that existed before this field, so a sheet nobody has configured
   behaves as it always did. Never render it as "unknown" and never fall back to
   some other policy.
2. **An explicit `""` in an `updated` event is an edit**, not an absence: it is how
   an organizer switches a sheet back from a post to the QR rule. Absence of the
   key still means "unchanged".
3. **Resolve the id against your own checkgroup projection.** HQ does not validate
   it on write, and nothing re-publishes the sheet if the checkgroup is later
   deleted. Treat an id that does not resolve as `""` — the QR rule — because the
   alternative is a reveal keyed to a post that will never be reached, and those
   checkpoints would never appear at all. HQ's own read path does exactly this, and
   like §4 the fix does not travel over the stream.

### 2. Find the patrol sheets by `teamType`, never by name

Set names are Danish free text an organizer may rename mid-season — "Patruljer",
"Patruljekort", "Patruljer nord". Matching on them will break.

### 3. `teamType` is a filter, not a key — and three consequences

Read it as *"this set is specifically for this team type"*, **not** *"only this
team type uses it"*. Values: `patrulje`, `klan`, `crew`, `gøgler`, or absent.

1. **Absent is the ordinary case, not missing data.** The crew set covers gøglere,
   banditter and crew, who are not one team type.
2. **It is not unique.** More than one set may carry the same value — a year that
   splits its patrol maps into two sets is legitimate. Collect *all* matching sets;
   there is no "the" patrol set.
3. **Filtering by `klan` will usually match nothing, and that is not an error.**
   Klaner normally draw from the unmarked crew set. Concluding "this klan has no
   maps" is wrong — **fall back to the unmarked set(s)**.

### 4. You must resolve `checkpointIds` yourself

A `kort` event carries the ids that were saved, and nothing re-publishes them when
a checkpoint later disappears. In particular **deleting a checkgroup emits no
per-checkpoint event** — HQ's own read path copes by resolving ids against the
checkpoint table and ignoring what is missing, and that fix does not travel over
the stream.

So resolve `checkpointIds` against your own checkpoint projection and ignore ids
that do not resolve. You already consume `NATHEJK.*.checkpoint.*` and
`NATHEJK.*.checkgroup.*`, so this costs nothing — but it will not happen by
accident, and the symptom would be a reveal naming a post that no longer exists.

### 5. A checkpoint may be on any number of sheets

Including several in one set: adjacent sheets overlap by design, and every patrol
sheet's ground is also on the crew map. Seeing an id twice is normal, not a bug.

Order within `checkpointIds` carries no meaning — a sheet's checkpoints are a set.
It is stable between saves, but read nothing into it.

### 6. `extents` is zero, one or two rectangles

- **Zero** — normal. A skitse has no area worth recording, and a sheet may be
  described before anyone draws it.
- **One** — an ordinary sheet.
- **Two** — a double-sided sheet, which is **one** `kort` with two areas: one
  sheet, one QR, one handover, one reveal.

The two are simply two areas. Nothing says which is the front, and the checkpoints
are **not** split per side, because both sides are handed over at once. Do not try
to infer sides.

```json
{ "northWest": { "latitude": 56, "longitude": 9 },
  "southEast": { "latitude": 55, "longitude": 9.4 } }
```

Always a true north-west/south-east pair — normalised before publishing — so no
min/max dance is needed before handing them to a map library.

### 7. Maps are per year and never carried forward

The event is in a different area each year, so last year's sheets and coordinates
have no meaning this year. A new year starts empty.

---

## What is *not* published yet

- **Which QR code sits on which sheet.** Every printed sheet carries a unique QR,
  and scanning an unknown one prompts HQ to link it to a patrulje. Linking it to a
  *sheet* is the natural next step and is not built. Until it is, these events give
  you the **candidate** sheets for a team type; matching a specific scan to a
  specific sheet has to come from elsewhere.
- **Where a sheet is handed out.** Now recorded as a *plan* — see `handoutCheckgroupId`
  in §1.1. What actually happened is still not recorded, and is not planned: it
  varies and is not reliably known.

## Questions

Raise them against PRD 010 (`roadmap/prd/done/010-kort-definitions.md` **in the
hq repo**), which carries the reasoning behind all of the above. These event
shapes are a contract with your repo: if one needs to change, it changes there
first.
