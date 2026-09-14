# 223 — Move the verified publish from the `member` subject to `spejder`

**Status:** open
**Priority:** high
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] `go/nathejk/table/person/verified.go` builds `NATHEJK.<year>.spejder.<memberId>.verified`
      and the old `.member.` builder is gone
- [ ] The consumer subscribes to `NATHEJK.*.spejder.*.verified` and receives events
      published by the BFF end to end
- [ ] The year token is taken from the member's own row; no config lookup on the publish path
- [ ] The old subject was checked on the live stream and confirmed empty before the old
      builder was deleted, with the finding recorded in this task's log
- [ ] A comment at the publish and consume ends states that `spejder` is a subject token and
      does not denote the population
- [ ] The member id is still its own subject token, so a per-person purge still reaches the
      event

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-12 — Task created from PRD 015.
