# 310 — Glimt retention purge, configurable, 0 disables

**Status:** done
**Priority:** medium
**Created:** 2026-09-17
**Picked up by:** agent session (Zed)
**Started:** 2026-09-17
**Completed:** 2026-09-17

## Description

PRD 019 §6, §8. Mirror `go/cmd/api/portraitpurge.go`, which already solves this shape.

Two env vars, both `time.Duration`, registered like `portraitRetention` in
`go/cmd/api/env.go` (`flag.DurationVar` + `envDuration`):

| var | controls |
|---|---|
| `GLIMT_RETENTION` | how long a glimt is kept after creation |
| `GLIMT_PUBLIC_RETENTION` | how long it stays on the public feed |

**`0` disables the purge entirely** — required for dev and testing, where a fixture posted
last month must still be there tomorrow. Guard with `<= 0` so a negative value also means
off, and **log it at startup** as `portraitpurge.go:44` does: "off for dev" and "off because
someone fat-fingered the env in prod" look identical otherwise, and this is the one config
value whose failure mode is keeping children's photos forever.

Measured **from creation**, not from a configured event end date — the same simplification
`portraitRetention` makes, and it fails in the safe direction.

The effective value is exposed on `/api/config` (which already carries
`contacts_poll_seconds`) so the composer and `PrivacyView` can state the real number instead
of a hard-coded one.

## Acceptance Criteria

- [x] `glimtRetention` and `glimtPublicRetention` on `config`, via flag + env, documented in `env.go`
- [x] Sweep purges glimt older than the window, publishing `.purged`
- [x] `0` (and negative) disables the sweep and logs that it is disabled
- [x] Public retention can be shorter than the internal one and is applied independently
- [x] Effective retention days served on `/api/config`
- [x] Tests: purges past the cutoff, keeps inside it, does nothing when disabled
- [x] `go test ./...` passes

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.
- 2026-09-17 21:30 — Picked up. Defaults set to the values proposed in PRD 019 §11 Q1 and not yet
  confirmed: **`GLIMT_RETENTION=90d`, `GLIMT_PUBLIC_RETENTION=30d`**. They are now defaults in code
  rather than an open question in prose, which is the right place for them — but the maintainer has
  not signed off on the numbers, so §11 Q1 stays open and changing them is an env edit, not a
  deploy.
- 2026-09-17 21:40 — **Design decision: public retention is a read-time cutoff, not a state
  change.** "Unpublishing" an expired glimt would have been tidier and is wrong twice over: the
  audience is immutable (PRD 019 §6), and hiding is a moderation act with a person's name on it —
  neither should be repurposed by a timer. So `PublicFeed` takes a `notBefore` and older glimt
  simply are not returned to the open web while remaining exactly what they were inside the app.
  It is also **reversible**: lengthening the window brings them back, which a state change could
  not do without inventing an "unexpire" event. A test asserts nothing is published and the row is
  not mutated.
- 2026-09-17 21:45 — **`GLIMT_PUBLIC_RETENTION=0` means "as long as the glimt itself", not
  "immediately".** The other reading is the one that would have quietly emptied the public page in
  every dev environment, where all the retention values are 0. Tested explicitly, including the
  negative case.
- 2026-09-17 21:55 — **Ordering is the opposite of the portrait purge's, and this took some
  thought.** `portraitpurge.go` argues convincingly for bytes-first: the row keeps pointing at a
  missing object, reads degrade to "no photo", and the next run retries the event — self-healing,
  and nothing else can be harmed because a portrait's objects belong to exactly one person.
  A glimt's objects may be **shared** (task 306). Bytes-first plus a failed publish would mean the
  retry comes back to a row whose refs are already deleted, and any surviving glimt sharing them is
  blank permanently with no record of why. So this publishes first: a failure leaves the glimt whole
  and the next tick tries again, and the worst case is a late purge — which is the failure a
  retention job should prefer. Both orders are now documented with their reasoning, in their own
  files, because the next person will notice they disagree.
- 2026-09-18 — Reuses `purgeGlimtBlobs` from task 306, so the shared-object check cannot be present
  in the user-initiated delete and missing here. A test proves the sweep leaves media a surviving
  glimt still references — the same hazard, but in bulk, at 03:00, with nobody watching.
- 2026-09-18 — `/api/config` now carries `glimt_retention_days` and
  `glimt_public_retention_days`, rounded **down** so the number a member reads is never longer than
  the truth: "2 dage" for a 36-hour window would be a promise the service does not keep. Two
  separate numbers because the composer's `public` option should be able to say how long the open
  web keeps it, which is the part a member is most entitled to know before tapping "Offentligt".
- 2026-09-18 — Startup logs both the enabled and disabled cases, with the consequence spelled out
  when disabled ("glimt and their media are kept indefinitely"). "Off for dev" and "off because
  somebody fat-fingered the env in production" are indistinguishable unless the service says which
  it thinks it is doing.
- 2026-09-18 — ✅ All criteria met. `gofmt` clean, `go build ./...`, full `go test ./...` green.
  Moving to done.

### Notes

- **PRD 019 §11 Q1 stays open.** The numbers are now defaults rather than a proposal, but they are
  the ones I suggested, not ones the maintainer has confirmed. Worth a decision before the first
  event.
- Task 317 (composer) and task 321 (privacy page) must read the days from `/api/config` rather than
  hard-coding them — that is the whole reason they are served.
- Task 323 (public page) must pass `app.glimtPublicCutoff()` into `PublicFeed`. Passing a zero time
  there would silently serve the whole archive to the open web.
