# 357 — Copy hq's `year` projection: the event's two cities

**Status:** done
**Priority:** medium
**Created:** 2026-09-21
**Picked up by:** agent session (Zed)
**Started:** 2026-09-21
**Completed:** 2026-09-21

## Description

Maintainer instruction, 2026-09-21, immediately after the 2026 diploma artwork landed:

> copy 'year' projection from hq-repo here, it carries to and from cities

It answers the open item on the diploma: the route line, *"fra Lundby til Glumsø"*, which `internal/diploma`
renders only when configured and which nothing in this repo could configure. The sibling `diplom` service had
2024's two villages hardcoded in Go — the arrangement that moving the diploma here (task 345) was meant to end.

### What was copied, and what was not

hq's `go/nathejk/table/year` is six files: `commands.go` (the write side), `consumer.go`, `querier.go`,
`filter.go` (paging and sorting for the admin list), `table.go`, `table.sql` — seven columns.

Copied: the **fold and the read**, narrowed to `cityDeparture` and `cityDestination`.

Left behind, each for its own reason:

- **`commands.go`** — hq owns the write side. An organizer edits the year there; nothing in this app publishes a
  year event and nothing here should.
- **`filter.go`** — paging, sorting and a validator for an admin list this app does not have. It also imports
  hq's `internal/validator`, so it could not have come across as-is: `nathejk/table/*` must not import
  `nathejk.dk/internal/...` (these packages are bound for shared-go).
- **`headline`, `description`, `dateStart`, `dateEnd`** — nothing on this surface renders them. Not a privacy
  argument like `public_patrol`'s dropped contact block; just that a column nobody reads is a column somebody
  eventually renders. Adding one back is a line in the fold and a line in the schema.

The table is named `event_year` rather than hq's `years`, because `year` is a column name throughout this
schema and a table called `years` next to a `year` column on every other table reads as a mistake.

### The two-source route line

`EVENT_ROUTE` already existed as config, with a comment claiming *"why configuration and not a projection:
because nothing upstream carries it"*. **That was wrong**, and the maintainer's one-line instruction is what
disproved it. The comment is corrected rather than deleted, because a confident false justification is worse
than none.

The projection is now the source; `EVENT_ROUTE` survives as an override and wins when set, since it is the only
one of the two that can fix a wrong line without waiting for upstream data and a replay. Every failure —
no projection, a database error, an unknown year, one city without the other — omits the line, which is exactly
what shipped before.

### Measured against the real stream

A replay filled the table on the first boot:

| slug | cityDeparture | cityDestination |
|---|---|---|
| 2025 | Tølløse | Køge |
| **2026** | **Frederikssund** | **Fløng** |
| null | | |

So this year's diploma can now say *"fra Frederikssund til Fløng"* with nobody typing it into a config file.

**The `null` row is upstream junk**, not a bug here: something published a year event whose slug is the
four-character string `null`. Harmless — no read asks for that year, and the fold is keyed by slug — and hq will
have the same row. Recorded rather than filtered, because a projection that silently drops rows it finds
implausible is a projection nobody can debug.

## Acceptance Criteria

- [x] The fold writes both cities and ignores the editorial fields
- [x] A nil field in the message does **not** overwrite a stored city with the empty string (an unrelated edit
      in hq must not erase the route)
- [x] The subscription's three-token patterns cannot match an entity event — asserted, since the whole safety of
      `NATHEJK.*.updated` rests on it
- [x] The diploma prints the projection's route, the operator override beats it, and half a route prints nothing
- [x] A failing read costs the line, not the diploma
- [x] Verified against the real stream, not only fixtures
