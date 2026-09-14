# 248 — One plate spelling across both apps

**Status:** open
**Priority:** high
**Created:** 2026-09-14
**Picked up by:**
**Started:**
**Completed:**

## Description

The vehicle inventory currently holds licence plates in **two spellings**, so the
duplicate detection PRD 010 built cannot see across them.

Found while verifying task 247 against the dev database:

```
licensePlate: EC16795        ← registered in hq
licensePlate: DK+CA63640     ← registered in hej (this app)
```

`hej` normalises through `internal/plate` before storing or comparing (task 236):
upper-cased, separators stripped, country-prefixed. `hq`'s organiser CRUD
(`go/cmd/api/vehicle.go`) stores `strings.TrimSpace(input.LicensePlate)` — raw.

**The consequence is precisely the failure PRD 010 exists to prevent, arriving from the
other side.** A crew member registering the car an organiser already entered as `EC16795`
normalises to `DK+EC16795`, matches nothing, and gets a second row for one car — which is
the duplicate a coordinator then reconciles by hand. It also means a plate typed into
`hq`'s search may not find a car the participants' app registered.

Nobody has hit it yet only because participant registration shipped today.

## The decision to make

Where does normalisation belong? Three options, and this task should not start until one
is chosen:

1. **In shared-go, on the write path** — `vehicle.Commands.Register`/`Update` normalise,
   so every producer is correct by construction and neither app can get it wrong again.
   The strongest option, and it makes `hej`'s `internal/plate` a duplicate to retire or
   to move into shared-go.
2. **In `hq`'s handler**, mirroring what `hej` does. Cheapest, but leaves the rule stated
   twice in two repos — the same setup that produced the drift.
3. **Normalise on comparison only.** Rejected in advance: it leaves the inventory holding
   values that look different for one car, so every future reader has to remember.

Whichever is chosen, **existing rows need a migration** — and note the trap task 243 hit:
projections are rebuilt by replaying the log, so a SQL backfill of `licensePlate` is undone
at the next boot. The value has to be normalised where the event is applied, or the events
themselves have to be corrected.

## Acceptance Criteria

- [ ] A decision recorded on where normalisation lives (see above), in PRD 010 §11
- [ ] One implementation, reachable from both apps — not one per repo
- [ ] `hq` stores normalised plates for new registrations and edits
- [ ] Existing rows read back normalised, and the fix survives a projection rebuild
- [ ] A plate registered in `hq` is detected as a duplicate by `hej`, and vice versa —
      asserted by a test, with the two spellings from the report as the fixture
- [ ] `hq`'s plate search finds a car registered from the participants' app

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-14 — Found while verifying task 247 end to end: the dev database holds both
  `EC16795` and `DK+CA63640`. Raised rather than fixed on the spot — it is a cross-repo
  modelling decision with a migration attached, not part of that fix.
