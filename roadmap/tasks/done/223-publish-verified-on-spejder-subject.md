# 223 — Move the verified publish from the `member` subject to `spejder`

**Status:** done
**Priority:** high
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

## Description

PRD 015 §6 puts the verification event on the same prefix as the member-lifecycle events
instead of on a subject of its own:

```
NATHEJK.<year>.member.<personId>.verified     → NATHEJK.<year>.spejder.<memberId>.verified
                                                (consumed as NATHEJK.*.spejder.*.verified)
```

The subject builder lives in `go/nathejk/table/person/verified.go`; the consumer's
subscription moves with it. Per-person purgeability is preserved because the member id
remains its own token — that property is a privacy requirement (PRD 015 §6
Non-Functional), so it must survive the move rather than be re-derived afterwards.

The `<year>` token must come from **the member's own row** — the year of the record being
verified — not from a separately resolved "current event year" out of config. Those are the
same value today, and a publish that reads config rather than the member is precisely how
they stop being the same value.

The maintainer has confirmed nothing was ever published on the old `.member.` subject, so
there is no migration and no dual subscription. Still: **verify against the live stream
before deleting the old builder** (`nats stream view` on the old subject). The cost of that
confirmation being wrong is a verification that is silently never projected, which nothing
in the system will complain about.

Note for whoever writes the consumer side: `spejder` in the subject is a **token, not a
population**. Bandits publish their own-phone verification on it too (PRD 015 §6), so a
consumer must not read the token as "this member is a spejder". The member-lifecycle events
on the same prefix already work this way, so the convention is inherited rather than
invented — but it belongs in a comment, because role is what a reader will otherwise infer.

Depends on task 222 (the reshaped message lands upstream first).

## Acceptance Criteria

- [x] `go/nathejk/table/person/verified.go` builds `NATHEJK.<year>.spejder.<memberId>.verified`
      and the old `.member.` builder is gone
- [x] The consumer subscribes to `NATHEJK.*.spejder.*.verified` and receives events
      published by the BFF end to end
- [x] The year token is taken from the member's own row; no config lookup on the publish path
- [x] The old subject was checked on the live stream and confirmed empty before the old
      builder was deleted, with the finding recorded in this task's log
- [x] A comment at the publish and consume ends states that `spejder` is a subject token and
      does not denote the population
- [x] The member id is still its own subject token, so a per-person purge still reaches the
      event

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
- 2026-09-12 — Picked up. Plan: rename the builder, move the subscription and the routing
  case, and thread the member's own `Person.Year` into the publish instead of
  `app.config.eventYear`.
- 2026-09-12 — **Old subject confirmed empty on the live stream**, as the maintainer said:
  `nats stream subjects NATHEJK "NATHEJK.*.member.*.verified"` → "No subjects found" (run
  from a nats-box container on the shared `jetstream` network; the NATHEJK stream holds
  29,474 messages, so the answer is about this subject, not an empty stream). The new
  subject is likewise empty, which is the expected before-state. So: no dual subscription,
  and the old builder is deleted rather than deprecated. ✅
- 2026-09-12 — Subject moved in `verified.go`, subscription and routing case moved in
  `consumer.go`. The routing case sits immediately before `spejder.*.updated`, with a
  comment saying why: it is a four-part subject, so it must be matched before that pattern
  and must stay out of the five-part lifecycle group above it, which resolves every match to
  a member *status* — and a verification is not a status. Added a unit assertion that the
  built subject does not match a lifecycle pattern, so that ordering trap is caught by a
  test rather than by a comment.
- 2026-09-12 — `storeVerification` now takes a `person.Person` instead of a person id, so
  the subject's year comes from `p.Year`. Documented why: the two values agree today, and
  reading config here is exactly how they would stop agreeing unnoticed — the subject would
  claim a year the row does not have, and the projection's UPDATE keys on both, so it would
  silently match no rows.
- 2026-09-12 — That change made every `cmd/api` fixture without a `Year` publish to a
  malformed subject and answer 500. Set `Year` on the confirm fixtures and said in a comment
  that the field is now load-bearing, since a 500 is an unhelpful way to rediscover it.
- 2026-09-12 — Added an end-to-end subject assertion in `confirm_test.go` (exact string plus
  a `Match` against the consumed pattern). Worth the duplication with the person-package
  test: publish and consume are two strings in two files, and a mismatch raises no error
  anywhere — the projection just never writes.
- 2026-09-12 — `gofmt -l`, `go vet ./...` and `go test ./...` clean. All criteria met.
