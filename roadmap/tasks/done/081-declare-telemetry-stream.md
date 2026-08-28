# 081 — Declare the telemetry stream on the broker

**Status:** done
**Priority:** high
**Created:** 2026-08-26
**Picked up by:** agent session (Zed)
**Started:** 2026-08-28
**Completed:** 2026-08-28

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

- [x] A telemetry stream exists on the shared broker, declared in the repo that owns
      broker topology, with subjects that do not overlap `NATHEJK.>`
- [x] Name and subject pattern agreed and written down here, so 084 and 086 can rely
      on them
- [x] Retention is **indefinite for now** (PRD 002 §11.1) — which is JetStream's default,
      so this is about confirming it rather than configuring it
- [x] The subject pattern is **addressable per person**, so `nats stream purge --subject`
      can later remove one individual's track
- [x] Documented: how to set an age cap later. The stream library's `Create(name)`
      accepts no retention options, so today this is an operator action
      (`nats stream edit`), not a code change — worth stating plainly, since §11.1 calls
      the cap "cheap to change"
- [ ] The dev stack can publish to it and read it back (proven by 084/086, not here)
      *Deliberately left open, as the criterion itself says. Publish and read-back are
      proven with the `nats` CLI (see the log), but "the dev stack" means hej's own BFF,
      which is task 084.*

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

## Progress Log (continued)

- 2026-08-28 — **Maintainer's decision: create the stream, owned by the `nathejk` repo,
  subject shape `TELEMETRY.<year>.track.<personId>.reported`.** That settles the question
  this task was blocked on and unblocks 084.
- 2026-08-28 — Found the `nathejk` repo had **no stream-declaration artifact at all**:
  `NATHEJK` was created ad hoc, so "what streams should exist, with what retention?" could
  only be answered by asking the broker — which only knows what someone last typed. So this
  established the convention rather than following one.
- 2026-08-28 — Landed in `nathejk` as commit `1ca0d20`, a new `streams/` directory:
  * `TELEMETRY.json` and `NATHEJK.json` — declared configs. `NATHEJK.json` was derived from
    the live stream so applying it is a verified no-op, because a topology file set that
    omits the main stream documents nothing.
  * `apply.sh` — creates missing streams, and for existing ones prints a diff and exits
    non-zero **without changing anything**. Editing a stream can shrink limits and silently
    discard messages, which for an append-only log is unrecoverable, so reconciliation stays
    a deliberate operator action.
  * `README.md` — the reasoning: why the volume measurements make this a sibling stream, why
    per-person subjects are the erasure mechanism, and how to add an age cap later.
  * Runs on nothing but docker: the `nats` CLI is absent from every project container, so it
    uses the official `nats-box` image (which ships `jq`) on the broker's own network.
- 2026-08-28 — `TELEMETRY` config, matched deliberately to `NATHEJK` rather than by default:
  subjects `TELEMETRY.>`, `limits` retention, file storage, everything unlimited,
  `max_age: 0` (indefinite, per PRD 002 §11.1), 1 replica, 2-minute duplicate window.
- 2026-08-28 — Verified against the dev broker, not assumed:
  * created cleanly; three publishes landed on two per-person subjects;
  * **`NATHEJK` stayed at 29,136 messages**, which is the real check that the subject
    spaces do not overlap;
  * a message read back byte-intact;
  * **`nats stream purge TELEMETRY --subject '…mock-spejder-1.reported'` removed exactly
    that person's two messages and left the other person's one** — so PRD 002's erasure
    requirement is demonstrated, not merely designed;
  * the test messages were purged afterwards so the dev stream starts empty for 084.
- 2026-08-28 — Checked the drift detection actually detects, because a comparison that can
  only ever say "OK" is worse than none: a deliberately altered `max_age` in the file was
  reported as drift with exit 1, and the live stream was left untouched. Reverting the file
  returned it to OK.
- 2026-08-28 — Recorded for 083's benefit: the broker's **2-minute duplicate window is not
  the deduplication guarantee**. A phone offline for hours ships its backlog long past any
  window, so suppression is the consumer's job — `(person, timestamp)` identifies a point,
  which is what makes a replayed batch idempotent by construction. Task 082 already enforces
  that as the IndexedDB key path; 084 must carry it through to the subject payload.
- 2026-08-28 — **Not done here, on purpose:** the stream exists only on the **dev** broker.
  Production needs `NATS_URL=… NATS_NETWORK=prod ./streams/apply.sh` run on the prod host,
  and `nathejk`'s `1ca0d20` is committed but **not pushed** — that is the maintainer's repo
  and their call.
