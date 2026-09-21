# 338 — Public patrol read model

**Status:** done
**Priority:** high
**Created:** 2026-09-19
**Picked up by:** agent session (Zed)
**Started:** 2026-09-19
**Completed:** 2026-09-19

## Description

PRD 011 §8. A projection carrying **exactly** what the public patrol page shows, and nothing else:

- patrol number, patrol name, gruppe, korps
- finish time — **nullable**, because a page opened by the backstop has no finish (task 330)
- the computed distance (task 339)
- **why** the gate is open, not merely whether

**Why the gate reason and not a boolean.** A page opened by the finish scan shows a diploma; a page
opened by the last-checkpoint-closed backstop does not (PRD 011 §0b.3, task 346). The two states are
not interchangeable, so collapsing them loses the distinction the template needs.

**Source: shared-go's `tables/patrulje`**, which carries `teamNumber`, `name`, `groupName` and `korps`
— confirmed 2026-09-19, so no upstream change is needed.

Two details that will otherwise be got wrong:

- **`korps` is a slug, not a label.** `shared-go/types/corps.go` maps `dds` → *Det Danske Spejderkorps*,
  `kfuk` → *De grønne pigespejdere*, and so on via `CorpsSlug.Label()`. Render the label. `andet`
  ("Andet") and empty are both "unspecified" and must be **omitted** rather than printed — a header line
  reading "Andet" tells a visitor nothing and looks like a bug.
- **That table also carries `contactName`** — a personal name, right next to the fields we want. This is
  precisely why the public surface reads a purpose-built projection instead of querying `patrulje`
  directly: the invariant "this page names no person" should be impossible to break by adding a field to
  a template. See task 337.

## Acceptance Criteria

- [x] A projection owned by its own consumer (PRD 008 §8), carrying only the fields listed above.
- [x] **No name column of any kind**, and `contactName` demonstrably not projected.
- [x] Extend task 337's structural check (`publicprivacy_test.go`) to cover the patrol read model and
      its view types, the way it already covers the album ones.
- [x] `korps` stored as the slug, rendered via `types.CorpsSlug.Label()`; `andet` and empty omitted from
      the header rather than printed.
- [ ] Finish time is nullable and null for a backstop-opened patrol. *— moved to 341; see the log.*
- [ ] The gate reason is modelled as a distinguishable state (finish scan / backstop / override), not a
      boolean. *— already exists as `publicgate.Reason` from task 330; see the log.*
- [x] Reads are keyed by year + patrol number, matching how the public URL addresses a patrol.
- [x] An unknown patrol yields "not found", not an error.
- [x] Table SQL carries the *why* commentary this repo's projections use (see `checkgroup/table.sql`,
      `scan/table.sql`) — in particular, why there is no name column.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §8 / §10 (Phase 2).
- 2026-09-19 — Picked up. First question was whether this needs a new projection at all, since shared-go
  already projects `patrulje` with `groupName` and `korps` on it. **It does, and the reason is sharper than
  the PRD stated.**
- 2026-09-19 — shared-go's `Patrulje` struct carries `ContactName`, `ContactPhone`, `ContactEmail` and
  `ContactRole`, and its `GetAll` selects all of them. Reading the public page through that read would put
  a leader's name and mobile number in memory *inside the handler that renders an unauthenticated page* —
  one field access from a template, on the surface whose entire claim is that it names no person. So this
  folds the **same upstream events** (`NathejkTeamUpdated`, `NathejkPatrolNumberAssigned`) and writes four
  columns, dropping the contact block on the way through.
- 2026-09-19 — The distinction worth keeping: **"it is not here" is a property; "we do not select it" is a
  habit.** Written into both `table.sql` and the package doc, including an explicit "do not fix this by
  reading shared-go's table instead — that is not a refactor, it is a removal of the boundary".
- 2026-09-19 — Cost stated rather than hidden: a second consumer of events shared-go already consumes.
  Two things keep it small — the fold is four columns wide, and it couples to the published *messages*
  rather than to shared-go's schema.
- 2026-09-19 — ✅ **A test caught a wrong comment, which is the best thing a test can do to prose.** I had
  written that the upstream colon spelling (`NATHEJK:*.patrulje.*.updated`) was load-bearing and that a
  dot "would match nothing". `TestConsumesUsesTheUpstreamColonSpelling` failed — and the reason is that
  `subject.FromStr` **replaces the first colon with a dot**, so the two spellings are identical at the
  library boundary. Corrected the comment to say the colon is cosmetic (kept only so grepping for how
  these events are spelled gives one answer, and so a reader comparing this file to shared-go's finds no
  unexplained difference), and replaced the test with one that asserts the *normalisation* — because a
  comment claiming a library behaviour goes stale on the next upgrade.
- 2026-09-19 — The other failing test was also my premise, not the code: a subject that matches no verb is
  a **no-op, not an error**, which is correct — this projection subscribes to two verbs and the stream
  carries many. Replaced with `TestUnmatchedSubjectsAreIgnored`, which pins that down for `started`,
  `signedup`, a klan subject and a too-short one.
- 2026-09-19 — `teamNumber` has exactly one writer (`numberassigned`), asserted. Two writers is how a
  replayed update reverts an assigned number — and a patrol whose number is reverted has no reachable
  page, since the number *is* the URL.
- 2026-09-19 — Klans are ignored, because PRD 011 §11 Q10 has not decided whether they get a page. A klan
  in this table would be a page nobody decided to publish. An *absent* type is still folded, since the
  early signup events do not always carry one.
- 2026-09-19 — `ByNumber` uses `LIMIT 1` with a stable `ORDER BY teamId` rather than a unique index on
  (year, number). Nothing upstream guarantees one number per year, and a unique index would make the
  **projection** fail on replay — taking down every public page over a data problem affecting one patrol.
- 2026-09-19 — Extended task 337's structural check to `publicpatrol.Patrol`, and made it assert in both
  directions: no person-shaped field, *and* the five expected fields are still present — so "no person"
  cannot be achieved by deleting the feature. **Verified by adding `ContactName` to the struct**: it fails
  twice, once naming the field as person-shaped and once noting that a new field on this type belongs in
  the test's list deliberately. Restored and green.
- 2026-09-19 — Verified against real 2026 data in the dev stack: **380 patrols projected**, names, group
  names and korps slugs populated, 194 with both group and korps. Slugs seen: `dds`, `fdf`, `kfuk`,
  `kfum`, `andet` and empty — the last two are exactly the ones `KorpsLabel` omits.
- 2026-09-19 — One finding from that check is stronger than the criterion asked for: shared-go's
  `patrulje` table **does not exist in hej's database at all**, because this service never registered that
  projection. So the leaders' contact details are not merely absent from the public table — they are
  absent from this service entirely, on this event path. Worth knowing before somebody "simplifies" by
  registering the upstream projection.
- 2026-09-19 — **Two criteria moved rather than ticked.** The *finish time* needs the last-checkgroup scan
  lookup, which belongs with the page that renders it (341) — and it is a derived value, not something a
  team event carries, so projecting it here would mean this consumer reading the scan table. The *gate
  reason* already exists as `publicgate.Reason` from task 330, with the four-valued shape this criterion
  asked for; it is composed at the handler rather than stored, because it depends on the clock. Both
  noted here rather than silently dropped.
- 2026-09-19 — `gofmt`, `go vet ./...` and `go test ./...` clean. Moving to done.
