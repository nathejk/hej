# 084 — BFF `POST /api/track` publishing to the telemetry stream

**Status:** open
**Priority:** high
**Created:** 2026-08-26
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 002 §11.1. Accept a batch of position points from the signed-in user and **publish**
it to the telemetry stream. No SQL: the BFF publishes, exactly as it does for every other
state change (PRD 008 §8).

**Blocked by task 081** — the stream must exist first. JetStream routes by subject, so
publishing to a subject no stream claims fails quietly in the least useful way: the
request succeeds and the data is nowhere.

## Security properties

Two, and both are the kind that are easy to leave out and hard to notice missing:

- **The person is resolved from the session, never from the request body.** Otherwise a
  member can report a track as somebody else — which in a race where position is
  competitively meaningful is not a theoretical concern.
- **The endpoint is rate limited.** A 2-minute cadence needs nothing like the login
  limiter's headroom, and an unbounded ingest endpoint is the one place where a client-side
  bug becomes a broker-capacity problem. Note the existing PIN limiter is per-IP; per-user
  is the more meaningful axis here, since participants will share networks.

## Subjects

Per person, per task 081's agreed pattern, so a later retention or erasure policy can be
applied to one individual. The year comes from the same config the directory uses
(`EVENT_YEAR`) rather than the clock, for the reasons in PRD 006 §11 Q7.

## Acceptance Criteria

- [ ] `POST /api/track` behind `requireAuth`, with OpenAPI annotations (mandatory per
      `.rules`, matching the style in `go/cmd/api/auth.go`)
- [ ] The person id comes from the session; a body that names a different person is
      ignored or rejected, and there is a test proving a member cannot report as another
- [ ] The batch is published to the telemetry stream on a per-person subject
- [ ] Writes no SQL
- [ ] Rate limited, per user
- [ ] Batch size is bounded, with a clear `413`/`400` rather than an unbounded read into
      memory
- [ ] Malformed points (missing timestamp, out-of-range coordinates, absurd accuracy) are
      rejected or dropped deliberately, with the choice recorded — this stream is retained
      indefinitely, so junk in it is permanent
- [ ] `202` on accept, `401` unauthenticated
- [ ] Behaves sanely when the broker is unreachable: the endpoint must fail in a way the
      client can retry (see task 083), **not** accept the batch and lose it. Note
      `commands.ErrNoPublisher` already exists for exactly this state
- [ ] Verified end to end against the dev broker: publish a batch, read it back off the
      stream

## Progress Log

- 2026-08-26 — Task created from PRD 002 §11.1.
