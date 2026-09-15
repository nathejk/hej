# 271 — Classify bandit catches in the real scan source

**Status:** open
**Priority:** medium
**Created:** 2026-09-15

## Description

Follow-up from task 254, which replaced the seeded scans mock with the real projection.

`internal/scans` has two kinds — `KindCheckpoint` and `KindBandit` — and they are part of the API
contract (`/api/patrol/scans` returns `kind`, and `ScanList.vue` renders a red skull for a bandit
catch). The mock produces both. **The real source produces only `KindCheckpoint`.**

The reason is that nothing available says a scan was a bandit catch. `qr.scanned` carries the
*scanner's* user id and nothing about their role, and the two cases are physically identical: someone
scans the patrol's code. Distinguishing them needs one of:

- a projection of bandit/klan teams, so a scanner can be recognised as a bandit; or
- a role lookup for the scanner id (the person projection may already be enough — a bandit is a
  `RoleBandit` user); or
- an upstream distinction, if one exists that we have not found.

The third is worth checking first, because a classification the organizers already make is better than
one we infer.

Task 254 deliberately did **not** guess. Labelling a post visit "Bandit taget" in front of a patrol
that was not caught would be worse than labelling it plainly, and the failure would be invisible to us
— it looks like data, not like a bug.

Until this lands, every real registration reads as a checkpoint scan. The bandit styling in the drawer
is therefore currently unreachable outside the dev fixture.

## Acceptance Criteria

- [ ] Establish whether the stream already distinguishes a bandit catch; record the answer either way.
- [ ] If it does not: classify by resolving the scanner to a role or team, in the BFF.
- [ ] A scan whose scanner cannot be classified stays `KindCheckpoint` rather than becoming a bandit
      catch — the failure must not invent a catch.
- [ ] Tests cover: post personnel scan → checkpoint; bandit scan → bandit; unknown scanner → checkpoint.
- [ ] Remove the note in `internal/scans/projection.go` that points here.

## Progress Log

- 2026-09-15 — Task created from task 254. Recorded rather than guessed: the classification is not
  derivable from the events this repo consumes today.
