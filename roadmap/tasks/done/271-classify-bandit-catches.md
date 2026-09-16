# 271 — Classify bandit catches in the real scan source

**Status:** done
**Priority:** medium
**Created:** 2026-09-15
**Completed:** 2026-09-16

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

- [x] Establish whether the stream already distinguishes a bandit catch; record the answer either way.
- [x] If it does not: classify by resolving the scanner to a role or team, in the BFF.
- [x] A scan whose scanner cannot be classified stays `KindCheckpoint` rather than becoming a bandit
      catch — the failure must not invent a catch.
- [x] Tests cover: post personnel scan → checkpoint; bandit scan → bandit; unknown scanner → checkpoint.
- [x] Remove the note in `internal/scans/projection.go` that points here.

## Progress Log

- 2026-09-15 — Task created from task 254. Recorded rather than guessed: the classification is not
  derivable from the events this repo consumes today.
- 2026-09-16 — **Upstream answer: no.** Checked `shared-go`'s contract directly. `NathejkQrScanned` carries
  `qrId`, `teamId`, `teamNumber`, `scannerId`, `scannerPhone`, `remark` and `location` — no role, no kind,
  no flag. Nor is there a sibling "caught" event: the only bandit-specific subject upstream is
  `NATHEJK.*.bandit.*.armNumber.assigned`, which assigns arm numbers and says nothing about catches. So the
  organizers do **not** already make this classification and we have to infer it. Recorded in the
  `scans.Banditter` doc comment so the next reader does not re-run the search.
- 2026-09-16 — Classified by resolving the scanner to a **role**, not a team: a bandit is a person in this
  repo's model (`person.Classify` maps the senior population to `RoleBandit`) and the scan event hands us a
  person id, so going via teams would need a second projection to answer what the person one already
  answers. Added a `scans.Banditter` seam (declared in `internal/scans`, so it still depends on nothing
  under `nathejk/table`) returning the year's bandit ids as a **set** — one `ListByAppRoles` read per
  request rather than one lookup per registration, the same query contacts already uses. Adapter
  `banditterOrNil` in `cmd/api/scansource.go` guards the nil-interface trap; `ProjectedScan` gained
  `ScannerID`; `scanSourceFor` now takes the person querier.
- 2026-09-16 — Every uncertain case resolves to `KindCheckpoint`: unknown scanner, empty scanner id, no
  person projection, and a failed classification read (reported, never swallowed). The asymmetry is the
  whole design — missing a catch understates what happened, while inventing one tells a patrol they were
  caught when they were not, and they would act on it. Also gave a bandit catch with no post name the label
  "Bandit" rather than the generic "Registrering", which reads better beside the drawer's skull; it names no
  individual, because who caught them is not this app's business. Tests in `internal/scans/classify_test.go`
  cover all three required cases plus no-classifier, read-failure, single-read and no-patrol-no-read. The
  bandit styling in the drawer is now reachable from real data. All Go gates green, incl. `GOWORK=off`.
