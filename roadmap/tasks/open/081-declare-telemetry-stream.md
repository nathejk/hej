# 081 — Declare the telemetry stream on the broker

**Status:** open
**Priority:** high
**Created:** 2026-08-26
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 002 §11.1. The position track is published to a JetStream stream that is a
**sibling of `NATHEJK`**, not to `NATHEJK` itself. That stream has to exist before
anything can publish to it — JetStream routes by subject, so a publish to a subject no
stream claims is silently accepted by nobody.

**This is a cross-repo prerequisite, and it blocks 084.** `hej` does not create
streams: nothing in the repo calls the stream library's `Create`, and `NATHEJK` is owned
by the `nathejk` repo, which owns the broker. Declaring a stream here would put two
repos in charge of broker topology, which is how a stream ends up with different config
depending on who booted last.

## Why a separate stream

Measured, so the decision is reviewable (PRD 002 §11.1). One 12-hour race, 827
participants, batched every 2 minutes:

| sampling | MB/event | vs. all of `NATHEJK` |
|---|---|---|
| 10 s | 330 | **18×** |
| 30 s | 157 | **9×** |

`NATHEJK` today is 18 MiB / 29,102 messages — the event's entire domain history.
Projections replay it **from sequence zero on every boot**, so telemetry in `NATHEJK`
would mean every future restart dragging hundreds of megabytes past every projector to
rebuild read models that do not want it.

## Acceptance Criteria

- [ ] A telemetry stream exists on the shared broker, declared in the repo that owns
      broker topology, with subjects that do not overlap `NATHEJK.>`
- [ ] Name and subject pattern agreed and written down here, so 084 and 086 can rely
      on them
- [ ] Retention is **indefinite for now** (PRD 002 §11.1) — which is JetStream's default,
      so this is about confirming it rather than configuring it
- [ ] The subject pattern is **addressable per person**, so `nats stream purge --subject`
      can later remove one individual's track
- [ ] Documented: how to set an age cap later. The stream library's `Create(name)`
      accepts no retention options, so today this is an operator action
      (`nats stream edit`), not a code change — worth stating plainly, since §11.1 calls
      the cap "cheap to change"
- [ ] The dev stack can publish to it and read it back (proven by 084/086, not here)

## Notes

Subject shape needs deciding as part of this task. It must carry the year (every other
subject does) and the person, and must not collide with `NATHEJK.>`. Something of the
shape `TELEMETRY.<year>.track.<personId>.reported` satisfies both, but the naming is
this task's call in agreement with the `nathejk` repo.

Do **not** key only by team: a team-keyed subject makes per-person erasure impossible
without rewriting the stream, and the team is resolvable from the person via the
directory anyway.

## Progress Log

- 2026-08-26 — Task created from PRD 002 §11.1.
