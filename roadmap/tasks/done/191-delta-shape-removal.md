# 191 — Removal semantics for cached datasets

**Status:** done
**Priority:** high
**Created:** 2026-09-01
**Picked up by:** agent session (Zed)
**Started:** 2026-09-01
**Completed:** 2026-09-01

> **Outcome, 2026-09-01: no delta protocol was built, and none should be.** The premise — "a delta
> that carries only present fields cannot say a value is gone" — is true of a delta and there is no
> delta: the manifest is a **full replacement**, and replacement expresses removal exactly. Both
> halves were already implemented and tested. What this task actually contributed is the *rule*,
> written down, plus the two gaps the existing tests left. See the log.

## Description

PRD 009 §6. The one part of the freshness contract that was **not** built with tasks 155/162,
and the reason task 171 stays open.

A delta that carries only present fields cannot say "this value is gone". So a member who
withdraws has their phone number purged on the server (PRD 007 §6, `portraitpurge.go`) while
every device that synced earlier keeps showing it — and the purge becomes a server-side
gesture. Row-level removal has the same problem in a different shape: a person who leaves the
caller's permitted set must *disappear* from the cached index, not merely stop being updated.

So the delta needs to express three things, not one:

- a **row added or changed**;
- a **field unset** on a row that otherwise remains (a withdrawn member keeps their name and
  status until the end of the race, but loses their number — PRD 007);
- a **row removed** entirely (permission narrowed, or the person left the permitted set).

**The server decides, always.** A client that filters a broader payload is not enforcing a rule
it evaluated — PRD 009 §6, and the same principle that keeps `phoneParent` out of responses
rather than merely unrendered.

## Acceptance Criteria

- [x] Removal is expressible — **by replacement, not by a delta**. `contactsManifestHandler` sends
      the whole permitted set and `contacts.store.fetch` replaces what it holds. Documented as PRD
      009 §6's convention, with the condition attached: a dataset that outgrows one payload may use a
      delta, and then it **must** carry explicit tombstones.
- [x] Applying an update can clear a field to absent — not to `""`, which would render as a
      callable-looking empty row.
- [x] **The test that makes this real:** sync, withdraw a member server-side, re-sync, assert the
      number is gone from device storage. *Already existed on both sides; generalised to "no omitted
      field survives a refetch", covering `crewFunction` and `portraitVersion` too.*
- [x] A narrowed permission removes rows on the next sync, verified the same way — **and moves the
      version**, which is the half that was missing.
- [x] Task 171 closed on merge.

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval; carries the one unbuilt point from task 171.
- 2026-09-01 — Picked up. First step: check what the shipped manifest actually does about removal before designing a delta for it.
- 2026-09-01 — **Stopped before designing anything.** The manifest is a full replacement and the
  client replaces rather than merges — both deliberate, both commented, both tested
  (`TestContactsManifest_ClearedFieldDisappears`, and "replaces rather than merges, so a purged phone
  number disappears"). So field-level removal already works. Building a delta with tombstones to
  satisfy a requirement written on the assumption of a delta would have added a protocol, a merge
  path and a class of bug, to solve a problem replacement already solves at under a megabyte.
- 2026-09-01 — **Recorded the rule instead**, in PRD 009 §6: replace while a dataset fits one
  payload; a delta only when it does not, and then tombstones are mandatory rather than optional. The
  point of writing it down is that "send only what changed" is the obvious optimisation for the next
  dataset, and it is the one that quietly makes a purge decorative.
- 2026-09-01 — **Gap 1, client: the existing test proves it for one field.** The next sensitive field
  will not be `phone` — `crewFunction` says where someone is posted, `portraitVersion` is a handle on
  their photograph. Generalised to "no omitted field survives a refetch", asserting against the
  *stored* copy as well as state, since the stored copy is what survives a reload and what an
  inspection of the device would find.
- 2026-09-01 — **Gap 2, server: removal without a version change is not a removal.** A row vanishing
  from the payload is not the same as a row that stops being updated — a device holding the earlier
  manifest keeps that person and their number until something tells it to replace what it has. So
  narrowing access has to move the version too, or every already-synced phone keeps the wider
  directory indefinitely. Added
  `TestContactsManifest_NarrowedAccessRemovesThePersonAndMovesTheVersion`; it passes, because the
  version is a hash of the payload — but nothing said so, and a future switch to an incrementing
  counter or a per-person mtime would break it silently.
- 2026-09-01 — No OpenAPI changes: no endpoint's shape changed. The manifest's annotation already
  documents `If-None-Match` and the version.
- 2026-09-01 — ✅ All criteria complete. 1 new Go test (`go test ./cmd/api/` green), 1 new client
  test; suite 299 across 24 files. **Task 171 closed.**
- 2026-09-01 — Picked up. First step: check what the shipped manifest actually does about removal before designing a delta for it.
