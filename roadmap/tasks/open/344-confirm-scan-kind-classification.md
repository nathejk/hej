# 344 — Confirm scan-kind classification against 2025

**Status:** open
**Priority:** medium
**Created:** 2026-09-19
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 011 §8. The patrol page's scan list labels each registration by **kind** — a checkpoint, a bandit catch,
or something else. `internal/scans` models `KindCheckpoint` and `KindBandit`, but whether the kind is
reliably *derivable* for every real scan has never been confirmed against real data.

The residual doubt, recorded across PRD 011's drafts: the scan event carries `scannerId` and `scannerPhone`
but **no kind**, so "checkpoint or bandit" has to come from who scanned. An earlier spot check of a 2025
`scannerId` found no match in the person projection, so the join is not obvious.

Related and already known: **a scan carries no checkpoint of its own.** It is attributed by asking which
post its scanner was on shift at (`checkpersonnel.sql`), and an **unattributed scan is a normal outcome**,
not an error — the same fact that makes task 330's trigger 1 miss occasionally.

**Why this is its own task rather than part of 341:** mislabelling a bandit catch as a checkpoint is worse
than showing neither, and it would be worse on a public page than it was in the app. Better to find out
what fraction of 2025's scans can be classified *before* the list claims a kind for all of them.

2025 is complete on the stream, so this is answerable now and does not need the 2026 event.

## Acceptance Criteria

- [ ] For a representative sample of 2025 scans, the proportion that can be classified as checkpoint vs
      bandit vs unknown is measured and recorded in this task's log.
- [ ] The derivation path is documented: which join, which table, and what happens when the scanner does not
      resolve.
- [ ] If a material share cannot be classified, the page's treatment of "unknown kind" is decided and
      recorded here — an honest "registrering" beats a guessed label.
- [ ] Unattributed scans remain **listed**, never dropped.
- [ ] Whatever is decided is expressed behind the existing `scans.Source` interface so handlers do not
      change.
- [ ] A test covers the unknown-kind path, not only the two happy ones.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §8 / §10 (Phase 2). Carries forward the unresolved part of the
  original PRD 011 draft's scan-classification question.
