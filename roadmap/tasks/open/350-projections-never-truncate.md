# 350 — Projections never truncate, so stale rows outlive the events that made them

**Status:** open
**Priority:** high
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

**A later, wider run made this much less of an edge case:** rebuilding *every* projection found 15 of 19 tables
coming back smaller, including two that came back empty. See the log.

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
- 2026-09-21 — **The whole dev database rebuilt from the stream, and the result is much stronger than this
  task assumed.** Maintainer: *"reread jetstream"*. Every event-sourced table truncated (all but `deadletter`),
  api restarted, replay left to settle:

  | table | before | after a clean replay | rows with no event behind them |
  |---|---|---|---|
  | `album` | 3 | **0** | 3 |
  | `album_item` | 7 | **0** | 7 |
  | `glimt` | 73 | 60 | 13 |
  | `glimt_media` | 84 | 60 | 24 |
  | `glimt_report` | 2 | **0** | 2 |
  | `kort` | 11 | 5 | 6 |
  | `kortsaet` | 6 | 2 | 4 |
  | `scan` | 7,765 | 7,733 | 32 |
  | `checkpersonnel` | 46 | 41 | 5 |
  | `person_section` | 48 | 43 | 5 |
  | `maphandout` | 888 | 877 | 11 |
  | `vehicle` | 42 | 38 | 4 |
  | `checkgroup` / `checkpoint` | 10 / 15 | 9 / 14 | 1 each |
  | `person`, `public_patrol`, `track_point` | — | unchanged | 0 |

  **Fifteen of nineteen tables came back smaller.** So this is not an edge case about one column on one row
  (task 349's `memberStatusAt`); a material share of what the dev database contained had no event behind it
  any more.

  ### The publish path is fine — checked, because that was my first suspicion

  With `album` at zero I assumed our own published events were being dropped, which would have been a serious
  bug. It is not happening: posting a takedown report put the event in the stream at **seq 46644**, timed and
  intact, and the projection folded it. The `NATHEJK` stream holds 46,643 messages from sequence 1 with
  `MaxAge=0`, `MaxMsgs=-1`, `MaxBytes=-1` — exactly one message deleted in its whole history.

  And yet a full scan of all 46,642 messages, plus `Info(WithSubjectFilter)` across **24,039 distinct
  subjects**, finds **zero** `*.album.*` messages — while the `album` table had three albums, and an earlier
  subject census in this same session *did* list album subjects.

  ### What that means, stated without inventing a mechanism

  The album fixtures were published to a broker, consumed, and projected. Their events are no longer in the
  stream this deployment reads. I cannot tell from here whether the shared dev broker was rebuilt or restored
  from an older snapshot, or whether those fixtures were created against a different instance of it. Either
  way the conclusion for this task is the same, and it is the point:

  > **A projection row is not evidence that an event exists.** The database outlives the log, because nothing
  > truncates it.

  This has now misled me **twice in one day** — first into writing a task asking another team to add a
  timestamp they already send (task 349's correction), then into suspecting our own publisher. Both times the
  data was real and the inference was wrong, and both times a rebuild-then-measure would have settled it in
  four minutes.

  ### Consequences worth deciding about

  - **Verification against dev data must start from a rebuild.** Otherwise a count can reflect a broker that
    no longer exists. That is now the strongest argument for option 1 in the list above — documenting the
    hazard — and a fair argument for option 2, truncating on boot, which would make the invariant true instead
    of aspirational.
  - **In production the same thing is survivable and silent.** If the org's broker is ever restored from a
    snapshot, `hej` keeps serving rows for events that no longer exist, and nothing anywhere says so.
  - **It cost the dev fixtures.** The albums had to be regenerated (`POST /api/dev/album-fixture`) after the
    rebuild, and the glimt, kort, vehicle and scan rows that did not come back are simply gone — they were not
    in the stream to come back from.

  ### And a real bug, found by the replay rather than by a report

  The `deadletter` table had an unresolved row **written during this replay**: a `person` upsert rejected with
  `Error 1406: Data too long for column 'postalCode'`, because upstream sent a 40-character address in a
  `VARCHAR(32)` field. The member survives only because a later event carried the split form. Raised as
  **task 352**, including the part that matters more than the column width: nothing surfaces a dead letter, so
  the one mechanism built to make a dropped write loud is read by nobody.
