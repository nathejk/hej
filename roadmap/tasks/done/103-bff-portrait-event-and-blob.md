# 103 — BFF: portrait event, projection column and blob write path

**Status:** done
**Priority:** medium
**Created:** 2026-08-28
**Picked up by:** agent (Zed / Claude Opus 5)
**Started:** 2026-08-28
**Completed:** 2026-08-28

## Description

Unblocked 2026-08-28 (task 102: consent held from sign-up, safety basis, purged
after the event).

Write side for the portrait: a domain event published through
`internal/commands`, consumed by the `person` projection to set `portraitRef`
(the column already exists on `person.Person`), with the bytes stored via
`internal/blob` content-addressed. No direct SQL write from a handler.

The read side is exposed through `internal/data.Models`, per the BFF
conventions.

## Acceptance Criteria

- [x] Portrait command + event; nothing writes `person` outside its consumer.
- [x] `person` consumer sets `portraitRef` from the event; replay-safe
      (idempotent).
- [x] Blob written content-addressed through `internal/blob`; stored objects are
      not publicly enumerable.
- [x] Consumer test covering set, replace and replay.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
- 2026-08-28 — Unblocked by task 102 and picked up.
- 2026-08-28 — **Decision: the event type lives in `nathejk/table/person`, not in
  `internal/`.** This is the first event *this* app publishes (every other one comes
  from shared-go), so somebody has to own the shape. The projection that consumes it
  owns it, and `cmd/api` — which already imports the package — publishes it. The
  alternative was a struct in `internal/portrait` **plus** a private copy in
  `person` (that package may not import `internal/`, or the eventual shared-go lift
  is blocked), i.e. two structs that must agree on JSON tags with nothing to catch
  them drifting.
- 2026-08-28 — **Decision worth a maintainer's eye: the subject is
  `NATHEJK.<year>.portrait.<personId>.captured`, i.e. on the existing `NATHEJK`
  stream**, not a sibling. Reasoning: it is a small, low-frequency domain fact, and
  PRD 008 §8's table already lists portrait metadata as a projection of the domain
  log. Contrast task 081, where a sibling stream was forced by *volume* (the track is
  9–18× the entire domain history, replayed on every boot) — that argument does not
  apply to one small message per member. Practical consequence: `NATHEJK.>` already
  claims the subject, so **no broker topology change and no cross-repo prerequisite**
  — which is what blocked 084 for two days. Provenance is preserved by the
  metatagger's `producer: hej-api`.
- 2026-08-28 — Added `portraitCapturedAt` (table.sql + `EnsureColumn` for existing
  deployments + querier). Task 109 needs an age on the row to purge from, and the
  message's delivery time is unusable for that because it changes on every replay.
  A missing timestamp writes NULL rather than `time.Now()` for the same reason;
  the purge treats NULL as purgeable rather than immortal.
- 2026-08-28 — The handler **refuses** a ref that is not 64 lowercase hex chars
  rather than writing it. Two reasons: it is untrusted input that ends up in SQL and
  later in a URL path ("../../etc/passwd" is a Ref-shaped string), and a ref no blob
  can satisfy would leave the row claiming a portrait while every read degraded to
  "no photo" — a disagreement nobody would notice. Dead-lettering keeps it visible.
- 2026-08-28 — `storePortrait` in `cmd/api` writes **bytes first, event second**, and
  the code says why: an event without bytes is permanently unserveable, while bytes
  without an event are an unreferenced object the retention sweep collects. The
  recoverable failure is the one we risk. A failed publish fails the call — reporting
  success would stop nudging a member for a photo nobody can look up, and this is a
  safety feature.
- 2026-08-28 — UPDATE, not upsert, matching the delete/team/start handlers: a
  portrait event must not invent a person.
- 2026-08-28 — One test pins something that is otherwise only true by coincidence:
  the subject `PortraitSubject` *publishes* must match the pattern `Consumes()`
  *subscribes to*. They are two strings in two files, and a mismatch is completely
  silent — the projection would just stay empty.
- 2026-08-28 — Reused `cqrstest.Publisher` after first hand-rolling a fake and
  discovering it could not satisfy `cqrs.Publisher`; the shared fake also decodes
  bodies through real JSON, so the assertions can catch a wrong field tag.
- 2026-08-28 — Two shared test fixtures had to move with the schema: `newTestApp`
  gained an in-memory blob store, and the person querier's sqlmock rows gained the
  new column.
- 2026-08-28 — ✅ All criteria met. `gofmt -l`, `go test ./...`, `go vet ./...` and
  `staticcheck ./...` green. **Not** verified against a live broker — that happens
  when the endpoint exists (task 105), where a publish can be driven over HTTP.
  Moving to done.
