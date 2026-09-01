# 187 — Readiness view in the profile page

**Status:** open
**Priority:** high
**Created:** 2026-09-01

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

- [ ] A section in `ProfileView.vue` listing every dataset from task 183 with size, last sync
      and state, reading from `offline.store` (task 184).
- [ ] Total storage used, from `navigator.storage.estimate()`, plus the persistence state.
- [ ] "Forbered til offline" with a size estimate shown **before** any transfer begins, and
      visible progress during it.
- [ ] Per-dataset manual sync and clear controls. Clearing anything `unrecoverable` either is
      not offered or requires an explicit confirmation that says the data cannot be recovered.
- [ ] An evicted or never-synced dataset produces an explanation, never a blank or a zero that
      looks like success.
- [ ] Section heading uses `font-nathejk`; body text and controls stay on the system stack.
- [ ] No `phoneParent` anywhere near this surface — it lists datasets, not people, and per
      `.rules` a guardian's number never enters the PWA outside a user's own confirmation.

## Progress Log

- 2026-09-01 — Task created on PRD 009's approval.
