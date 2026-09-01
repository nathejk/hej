# 187 — Readiness view in the profile page

**Status:** done
**Priority:** high
**Created:** 2026-09-01
**Picked up by:** agent session (Zed)
**Started:** 2026-09-01
**Completed:** 2026-09-01

## Description

PRD 009 §7. One place that answers "am I ready to go offline?" — a section in the profile page
(PRD 003, which is `done`, so this is an addition to a shipped page) alongside "På denne
enhed".

Per dataset: what it is, how large, when it last synced, and its state — never synced, synced,
stale, or evicted. Globally: total storage used, whether the phone may remove the data (task
185), and a prominent **"Forbered til offline"** action with a size estimate.

`views/TrackStatusView.vue` already does exactly this for one dataset, including
`navigator.storage.estimate()`. **Generalise it and link to it** rather than duplicating it —
the track's own detail view stays where it is; this section summarises and points there.

**The size estimate is the important detail, not decoration.** The bulk tier is ~324 MB and the
app cannot tell WiFi from cellular on iOS (PRD 009 §8), so the number in front of the user *is*
the consent mechanism. Show it before the download starts, not after.

Danish UI copy. shadcn-vue `Card`, `Progress`, `Badge`, `Button`, `Alert`; Lucide `WifiOff`,
`RefreshCw`, `HardDrive`, `Check`. `badge` is not generated yet — task 189.

## Acceptance Criteria

- [x] A section in `ProfileView.vue` listing every dataset from task 183 with size, last sync
      and state, reading from `offline.store` (task 184).
- [x] Total storage used, from `navigator.storage.estimate()`, plus the persistence state.
- [x] "Forbered til offline" with a size estimate shown **before** any transfer begins, and
      visible progress during it.
- [x] Per-dataset manual sync and clear controls. **Clear is not offered for unrecoverable data
      at all** — see the log; a confirmation dialog was the wrong answer here.
- [x] An evicted or never-synced dataset produces an explanation, never a blank or a zero that
      looks like success.
- [x] Section heading uses `font-nathejk`; body text and controls stay on the system stack.
- [x] No `phoneParent` anywhere near this surface — it lists datasets, not people.

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
- 2026-09-01 — Picked up. `components/profile/OfflineReadiness.vue`, rendered from `ProfileView`
  after "På denne enhed" rather than inside it: those rows report on *permissions* the user
  granted, this reports on *data* the phone holds. Same page, different question.
- 2026-09-01 — **Needed a seam for the per-dataset buttons.** The view cannot import five feature
  stores and guess which has a refresh method, and PRD 009 forbids a registry. Added
  `registerHandlers(id, { sync, clear })` to `offline.store`: functions only, no fetching logic, no
  policy. A dataset with no handler shows no button, which is the right default — the app shell has
  nothing a user could usefully re-fetch and the track has nothing to fetch at all.
- 2026-09-01 — **Clear is refused for unrecoverable data rather than confirmed.** The criterion
  allowed either. A dialog is the wrong answer: the only dataset it applies to is the position
  track, whose local copy may be the sole record of where a team was, and a "free up space" button
  next to that is a foot-gun no amount of copy fixes. Refused in the store *and* not rendered in
  the view — two locks, because the two can be changed independently.
- 2026-09-01 — **`prepareAll` is sequential and cheap-first.** Parallel downloads over rural mobile
  data compete for one thin pipe and make the progress indicator meaningless, and the 324 MB
  dataset has to run last so a user can abandon it without losing the small ones. Declared order
  gives that for free, since tiles already rank last.
- 2026-09-01 — **The size estimate comes from planned budgets, not from the server.** It has to be
  on screen *before* the first request, because on iOS the app cannot tell WiFi from cellular — the
  number is the whole consent mechanism. Partially present datasets are counted in full:
  overstating is the safe direction on a metered connection.
- 2026-09-01 — Copy decisions worth keeping: an evicted dataset says **"Slettet af telefonen"**,
  not "Mangler" — the user had it and the phone took it, which is a different sentence and on iOS
  the normal one. Stale says "Kan være gammel" on the new `warning` badge rather than anything
  alarming. Every state has words, never colour alone: this is read in bright sun and at 04:00.
- 2026-09-01 — ✅ All criteria complete. 11 new store tests; suite 277 across 21 files;
  `type-check` and `npm run build` clean.
- 2026-09-01 — Done. Every row currently reads "Ikke tjekket" and no buttons appear, because
  nothing has registered yet — task 192 is what makes this page say anything. Deliberately shipped
  honest-and-empty rather than seeded with optimistic defaults.
