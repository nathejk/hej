# 148 — Correcting the guardian number in the confirmation step

**Status:** open
**Priority:** high
**Created:** 2026-08-30
**Picked up by:**
**Started:**
**Completed:**

## Description

New requirement from the maintainer, 2026-08-30:

> if they can't complete the contact phone number and the field should open up and they have fill
> in a full number, after filling in the full number they should confirm it to be ok.

So the confirmation step gains a second path. Today a member who cannot recognise the registered
number can only *report* that (task 128) and move on, which leaves the record broken and the
organizer with work to do. Now they can fix it: the masked number becomes an **editable field**,
they type the full number, and they confirm **that** number as reachable.

This is a better outcome than a flag for the obvious reason — the member standing there is the
person most likely to know the right number — and it turns the step from "verify our data" into
"make sure we can reach an adult", which is what it was always for.

## Flow

1. Masked number shown, two digits requested (unchanged).
2. **"Nummeret er forkert" / "Jeg kender ikke nummeret"** opens the field: an empty phone input,
   prefilled with nothing (deliberately — see below), plus the same *"Dette nummer kan kontaktes i
   løbet af Nathejk"* acknowledgement.
3. The member types a full number and confirms. Both the number and the tick are required.
4. If they cannot supply one either, they **skip the step** and continue into the app. Nothing is
   recorded and nothing blocks; `confirmation_required` stays true, so they are asked again next
   time. *(Revised 2026-08-30: there is no longer a report event — the domain settles on
   `member.verified` alone, see task 147.)*

**The field starts empty rather than prefilled with the registered number.** Prefilling would
invite editing one digit of a number the member has just said they do not recognise, and would make
"corrected" indistinguishable from "retyped what we already had". An empty field asks the real
question: what number should we call?

## Data model — the part worth getting right

The guardian number is **owned upstream**: `person.phoneParent` is projected from
`NathejkScoutUpdated`. A member-supplied correction must not pretend otherwise, or the next upstream
publish silently reimposes the old number and nobody knows which value was believed when.

So the correction is recorded as **what the member acknowledged**, and the register keeps its own
value:

| column | meaning |
|---|---|
| `phoneParent` | what the register holds. Still projected from upstream, untouched by the app. |
| `acknowledgedPhone` | the number the member says can be reached — the registered one, or the one they typed. **Authoritative for contacting a guardian during the event.** |
| `verifiedAgainstPhone` | *(new)* what `phoneParent` was at the moment of acknowledgement. |

Two different questions then stay answerable, and they call for opposite responses:

- **Stale** — `phoneParent != verifiedAgainstPhone`: the register changed since the member
  acknowledged, so the acknowledgement is about a number that is no longer on file → **ask again**.
- **Corrected** — `acknowledgedPhone != verifiedAgainstPhone`: the member told us the register is
  wrong → **fix the register**, and leave the member alone.

Collapsing these into one comparison (which is what `Person.IsVerified()` does today) makes a
correction look exactly like a stale acknowledgement, so a member who corrected us would be asked
again forever while the register stayed wrong.

### Consequent changes

- `Person.IsVerified()` compares `phoneParent` against **`verifiedAgainstPhone`**, not against
  `acknowledgedPhone`.
- New `Person.GuardianCorrected()` — `acknowledgedPhone != verifiedAgainstPhone` — which is the
  organizer's actionable signal, and is *not* a failure state for the member.
- `invalidateVerification` (task 076) compares `verifiedAgainstPhone`, so an upstream change still
  clears a stale verification but a correction survives it.
- `GET /api/me/profile` keeps returning `phone_parent`; the client shows the acknowledged number
  when one exists, since that is the number that will actually be called.

## Endpoint

A separate endpoint rather than an extra branch on `confirm`:

```
POST /api/me/profile/guardian   { "phone": "...", "acknowledged": true }
  204 / 400 (unparseable or missing acknowledgement) / 401 / 429 / 503
```

The two are different acts — one agrees with what we hold, the other replaces it — with different
validation and different meaning in the log. Folding them into one endpoint would mean a body where
`digits` and `phone` are mutually exclusive, which is the shape that produces "which one did the
client mean?" bugs.

Both publish `member.verified`; this one with `AcknowledgedPhone` = the typed number and
`RegisteredPhone` = whatever the register held. Normalize with `internal/phone` before publishing —
the login lookup depends on normalized numbers and an unnormalized one here would read as a
different number to every comparison above.

There is no `report-incorrect` endpoint any more (removed with the `GuardianReported` message,
task 147). A member who can supply no number simply skips, and the absence of a verification is the
state. Worth naming what that gives up: an organizer can no longer tell "tried and could not
confirm" from "has not opened the app". This flow is what makes that acceptable — it turns almost
every would-be report into a verification — so it is the piece that has to work well.

## Acceptance Criteria

- [ ] The masked number can be replaced by a full number the member types, from within the step
- [ ] The field starts empty, and the acknowledgement tick is required alongside it
- [ ] The number is normalized server-side and rejected if unparseable, with non-punitive copy
- [ ] A correction publishes `member.verified` with both the acknowledged and the registered number
- [ ] `verifiedAgainstPhone` is projected, and `IsVerified()` compares against it
- [ ] `GuardianCorrected()` exposes "the register is wrong" separately from "the acknowledgement is
      stale"
- [ ] An upstream guardian-number change still invalidates a stale verification, and does **not**
      invalidate a correction
- [ ] The member can still decline entirely and continue; nothing blocks
- [ ] OpenAPI annotations on the new endpoint
- [ ] Tests: correction path, normalization, stale-vs-corrected, and that a correction survives an
      upstream re-publish
- [ ] `vue-tsc`, `npm test`, `go build ./...`, `go test ./...` clean

## Depends on

- **Task 147** — `RegisteredPhone` lands on the shared-go message. **This task is what that field
  is for**, so agree the shape here before lifting: adding a field to a message nothing consumes is
  free; reshaping one after `hq` subscribes is not.
- PRD 005 §12's correction-channel question becomes much smaller: for most members the channel is
  now "type the right number". It still needs an answer for the ones who cannot — and with the
  report event gone, a human is the only remaining route for them, so the copy must name one.

## Progress Log

- 2026-08-30 — Task created from the maintainer's requirement. Held for task 147's dependency bump,
  since the event needs the second phone number and that shape should be settled once.
