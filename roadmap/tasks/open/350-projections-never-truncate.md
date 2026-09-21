# 350 — Projections never truncate, so stale rows outlive the events that made them

**Status:** open
**Priority:** medium
**Created:** 2026-09-21

## Description

Found while verifying task 349, and **the opposite of what that task's log first claimed**. The original
version of this task asserted that upstream's member-lifecycle events carry no publish time. That was wrong,
and the maintainer said so:

> *"every event has a publish timestamp, this can be used for tracking time"*
> *"not in the message body, but in the message envelope"*

Correct. `jetstreamMessage` (in `stream/jetstream/envelope.go`) carries `"time"`, `createMessage` copies it to
`msg.Time()`, and a sweep of the whole 2026 dev stream found **every event has it** — 15,000+ messages across
~120 subject patterns, `withTime == n` for all of them.

### What actually produced the NULLs

`person` is created with `CREATE TABLE IF NOT EXISTS` and folded with `UPDATE`/upsert. **Nothing ever
truncates it.** So a value written from an earlier state of the stream survives forever, even when the event
that produced it is no longer there — and a column added later stays NULL on such a row, because no current
event re-writes it.

Measured by backing the table up, truncating it, and letting it rebuild:

| 2026 rows | before truncate | after a clean replay |
|---|---|---|
| `racing` | 85 (73 with a time) | **73, all 73 timed** |
| `waiting`, `transit`, `sheltered`, `reunited`, `released` | 5 (0 with a time) | **gone** |
| total rows (all years) | 4,251 | 4,229 |

So the five withdrawn members and twelve of the "racing" ones were **ghosts**: rows whose lifecycle events are
not in the stream any more. `memberStatusAt` is NULL on them for the honest reason that nothing has written
them since the column existed. On real data the stamp is 100%.

### Why this matters beyond one column

The repo's stated model is that projections are *rebuilt from sequence zero on every boot* — and the
idempotent upserts do make a replay converge on the same values. What a replay cannot do is **un-write** a row
whose event has disappeared, which happens whenever a stream is trimmed, re-seeded, or replaced (as the dev
one evidently has been). Consequences worth deciding about rather than discovering again:

- **Verification against dev data can be actively misleading.** This cost task 349 a wrong conclusion and an
  invented upstream problem; the next person may not re-check.
- It applies to **every** projection here, not just `person` — `public_patrol`, `glimt`, `scan`, `album`.
- It is only a dev hazard if production's stream is never trimmed. That is an assumption nobody has stated,
  and PRD 016's retention thinking suggests it may not hold forever.

Options, roughly in order of cost:

1. **Nothing, documented.** State the hazard where the replay is described, and treat "truncate and restart"
   as the standard way to verify a projection against real data.
2. **A boot-time truncate for projections that can afford it.** Correct by construction, and turns every boot
   into a full rebuild — which is what the code already claims happens.
3. **Per-row provenance** (last sequence seen) and a sweep for rows no replay touched. Most precise, most
   machinery.

## Acceptance Criteria

- [ ] A decision between the options above, recorded here with its reasoning.
- [ ] The replay contract is documented where it is relied on — `nathejk/table/*/table.go` or the eventing
      doc — including the sentence this task exists to supply: *a replay converges values, it does not delete
      rows whose events are gone.*
- [ ] The dev-verification implication is written down somewhere a person will find it before trusting a
      count: truncate, replay, then measure.
- [ ] `hej`'s distance fallback ("status with no time → exclude the member's points entirely") **stays**. It
      is now justified by a real case rather than a hypothetical one: a ghost row is exactly a status whose
      time we do not know.
- [ ] `person_backup_349` is dropped from the dev database once nobody wants the old rows (see the log).

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-21 — Task created with the **wrong** premise (upstream omits lifecycle timestamps), corrected the
  same day by the maintainer and rewritten. The measurement above replaces it.
- 2026-09-21 — The dev `person` table was truncated to run that experiment and has rebuilt from the stream
  (4,229 rows). The previous contents are kept in **`person_backup_349`** in the dev database. Five rows with
  lifecycle statuses (`waiting`, `transit`, `sheltered`, `reunited`, `released`, one each) did not come back —
  if they were fixtures somebody relies on for PRD 007's contacts pane, they will need re-creating as real
  events rather than as surviving rows.
