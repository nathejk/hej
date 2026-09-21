# 338 — Public patrol read model

**Status:** open
**Priority:** high
**Created:** 2026-09-19
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] A projection owned by its own consumer (PRD 008 §8), carrying only the fields listed above.
- [ ] **No name column of any kind**, and `contactName` demonstrably not projected.
- [ ] Extend task 337's structural check (`publicprivacy_test.go`) to cover the patrol read model and
      its view types, the way it already covers the album ones. Task 337 left this for 338 because the
      types did not exist yet — the behavioural route walk already covers
      `/offentligt/patrulje/:number`, so a leak will fail without a line being added, but the
      person-shaped-field check needs the types named.
- [ ] `korps` stored as the slug, rendered via `types.CorpsSlug.Label()`; `andet` and empty omitted from
      the header rather than printed.
- [ ] Finish time is nullable and null for a backstop-opened patrol.
- [ ] The gate reason is modelled as a distinguishable state (finish scan / backstop / override), not a
      boolean.
- [ ] Reads are keyed by year + patrol number, matching how the public URL addresses a patrol.
- [ ] An unknown patrol yields "not found", not an error.
- [ ] Table SQL carries the *why* commentary this repo's projections use (see `checkgroup/table.sql`,
      `scan/table.sql`) — in particular, why there is no name column.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §8 / §10 (Phase 2).
