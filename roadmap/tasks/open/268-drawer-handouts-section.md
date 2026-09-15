# 268 — "Kort udleveret" section in the drawer

**Status:** open
**Priority:** medium
**Created:** 2026-09-15

## Description

PRD 016 phase 4. Add the handout list to `ScanList.vue` as its own section, and show the
bottom handle when the patrol has **either** registrations **or** handouts (today: only
registrations).

- Section heading "Kort udleveret"; empty state "Ingen kort udleveret" (hq's wording).
- Per row: sheet name (or "Ukendt kort"), the **QR sticker number**, and when it was handed
  over. The sticker number is printed on the physical sheet, so it is the one identifier a
  patrol can read aloud down a phone when nothing else matches — which is the original
  problem this feature exists to solve.
- A sheet the patrol no longer holds is de-emphasised as **"afleveret"**, and **names
  nobody**. hq's organizer view says "Flyttet til {team}"; a participant may not see that.
- A synthesised handout (a skitse, task 257) has no QR number — the row must not leave an
  empty gap where it would be.
- Store + offline cache like the other datasets (replace, do not merge).
- Handle label covers both kinds now, so it can no longer say "Registreringer (n)" alone.
- Lucide `Map` icon; shadcn-vue primitives only.

## Acceptance Criteria

- [ ] "Kort udleveret" section renders below the registrations.
- [ ] Handle appears for a patrol with handouts but no scans (test) — the case today's
      `v-if="scans.hasAny"` gets wrong.
- [ ] "afleveret" state renders without naming another team (test).
- [ ] QR-less rows render cleanly — carried over from task 257, whose BFF work marks such rows
      `synthesised` so there is something to branch on. A skitse has no sticker number and the row
      must not leave a gap where one would go.
- [ ] Empty state string present.
- [ ] `handouts.store.ts` cached offline; replaces rather than merges (test).

## Progress Log

- 2026-09-15 — Task created from PRD 016 phase 4.
