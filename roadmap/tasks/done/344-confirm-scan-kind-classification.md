# 344 — Confirm scan-kind classification against 2025

**Status:** done
**Priority:** medium
**Created:** 2026-09-19
**Picked up by:** agent
**Started:** 2026-09-21
**Completed:** 2026-09-21

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

- [x] For a representative sample of 2025 scans, the proportion that can be classified as checkpoint vs
      bandit vs unknown is measured and recorded in this task's log. **All 3,588 of them, not a sample.**
- [x] The derivation path is documented: which join, which table, and what happens when the scanner does not
      resolve.
- [x] If a material share cannot be classified, the page's treatment of "unknown kind" is decided and
      recorded here — an honest "registrering" beats a guessed label.
- [x] Unattributed scans remain **listed**, never dropped.
- [x] Whatever is decided is expressed behind the existing `scans.Source` interface so handlers do not
      change.
- [x] A test covers the unknown-kind path, not only the two happy ones.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §8 / §10 (Phase 2). Carries forward the unresolved part of the
  original PRD 011 draft's scan-classification question.
- 2026-09-21 — **Done. The doubt was justified and the answer is worse than the task feared.**

  The task worried about *missing* a bandit catch. The real failure runs the other way: applying today's rule
  to 2025 would have told patrols they were **caught by bandits at every post they visited**.

  ### The derivation path, as it actually is

  A scan event carries `scannerId` and no kind. `internal/scans.kindFor` calls it a bandit catch when that id
  is in the set from `banditter.BanditIDs` → `person.ListByAppRoles(year, ["bandit"])`. And `appRole` comes
  from `person.Classify`, which maps **`PopulationSenior` → `RoleBandit` unconditionally**.

  That is the whole bug in one line: *senior* means "bandit" only in a year where the people staffing the
  event signed up as **crew** instead.

  ### 2025, measured — every scan, not a sample

  | | scans | share |
  |---|---|---|
  | scanner id absent | 0 | 0% |
  | scanner resolves to nobody in `person` | 1,280 | **35.7%** |
  | scanner resolves, `appRole=bandit` | 2,308 | **64.3%** |
  | scanner resolves, any other role | **0** | 0% |

  Every scanner who resolves is a "bandit". The population behind it: 2,465 people in 2025 — 1,366 bandit,
  974 spejder, 125 gøgler, and **zero** crew, postmandskab, guide or samarit. Zero section slugs.

  For one real patrol, number 112: **30 of its 36 registrations** would have read *"Fanget af en bandit"* on
  its public page.

  ### There is no corroborating signal to fall back on

  I looked for one before deciding, because "drop the feature" is a big answer:

  - **`armNumber`** — PRD 007 keeps it as the bandits' field identifier. **Empty for all 2,465 people in
    2025, and all 1,764 in 2026.** The assigning event has never been projected here.
  - **Post rota** ("was this scanner on a shift?") — `checkpersonnel` holds **2 rows for the whole of 2025**.
    So checkpoint attribution is absent too, which is why every 2025 label is "Registrering": not a
    regression, just nothing to name the post with.
  - **Section slug** — empty for every 2025 person, and for every *bandit* in both years (they come from the
    senior population, not from a crew section), so it cannot identify a bandit directly.

  ### The decision: trust the role only where the year separates staff from seniors

  Not "show no kind" (that discards a real part of the night for years where the data is fine) and not "trust
  the role" (2025). The capability question belongs to the **year**, not the scan: if a year has **any**
  crew-role person, its senior population has been narrowed to something meaningful and the classification
  holds; if it has none, everyone who helped is a senior and nothing may be called a catch.

  Implemented as a guard inside `banditter.BanditIDs` — one extra `ListByAppRoles` for the crew roles, using
  the method that already exists. **No interface change, no schema change, no handler change**, and
  `internal/scans` already treats an empty bandit set as "no banditter known" and labels everything a
  checkpoint visit. The `scans.Source` contract is untouched, as the task required.

  Measured effect: 2025 has 0 crew-role people → no bandit labels at all. 2026 has 160 → classifies exactly
  as before.

  ### Unknown kind: no copy change needed

  35.7% of 2025's scanners resolve to nobody, and those scans already read *"Registrering"* with a time —
  which is true, and is the honest way to say "this happened, we cannot say where". Every one stays
  **listed**; none is dropped. The map still omits a scan only when it carries no position, which is a
  separate fact the page already explains.

  ### Verified end to end, on the year in question

  Not only in unit tests: I ran a **read-only** instance pinned to `EVENT_YEAR=2025` with no broker — so it
  could not touch the projections, two processes folding the same subjects being the hazard
  `docker-compose.prod.yml` documents — and fetched patrol 112's page:

  - **36 registrations, all "Registrering", zero bandit labels.** Before: 30 would have claimed a catch.
  - The dev stack on 2026 still labels its 5 bandit scans on patrol 2, so classification survives where it is
    trustworthy.

  Also verified by breaking it: with the guard disabled, the 2025-shaped test reports three banditter.

  ### Two things this leaves for somebody else

  1. **The app has the same rule and the same exposure.** For 2026 it is fine (crew exists); for any
     re-import of an older year it would mislabel in the app too. Not changed here — outside PRD 011's
     surface and deserving its own decision — but the guard protects both, since both read through
     `banditter`.
  2. **`Classify`'s senior → bandit mapping is the underlying weakness.** The honest fix is upstream: a
     bandit should be identifiable as one (a section, a team type, or the arm number actually being
     projected), rather than inferred from what somebody did *not* sign up as.

  `gofmt`, `go vet`, `go test ./...` clean.
