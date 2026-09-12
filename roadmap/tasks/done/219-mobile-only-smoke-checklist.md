# 219 — The mobile-only smoke checklist

**Status:** done
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:** agent session (Zed)
**Started:** 2026-09-12
**Completed:** 2026-09-12

## Description

PRD 014, phase 5, §7.4. Write down what the simulation layer **cannot** cover, so it
does not quietly become a substitute for testing on a phone.

This is the counterweight to the whole PRD. A simulated iPhone is not an iPhone, and
the failure mode of a good simulation is false confidence — shipping something that
works on a laptop and not in a forest.

Rows to cover, each with what to establish:

- iOS Add to Home Screen, and the resulting splash / status-bar chrome.
- **iOS Web Push** (16.4+, home-screen only) — the platform where push matters most
  and the one thing a laptop cannot approximate at all.
- Android `beforeinstallprompt` accept path and the richer install dialog.
- Real GPS: drift, indoor loss, accuracy variance, backgrounding, and iOS killing a
  backgrounded standalone app (`location.store.ts:90`).
- Real camera framing and portrait quality.
- Touch ergonomics, one-handed reach, sunlight legibility.

Note the overlap with task 139's device matrix, which covers device *classification*.
This list is about capability and feel, not classification — cross-reference rather
than duplicate, and say which of the two a given failure belongs to.

## Acceptance Criteria

- [x] A checklist committed under `roadmap/` — `roadmap/mobile-only-checklist.md`
- [x] Each row states what to establish, not merely what to open
- [x] Cross-references task 139's matrix and says how the two differ, with a rule for
      deciding which list a given failure belongs to
- [x] Referenced from `README.md` (task 218) and from PRD 014 §7.4

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 5.
- 2026-09-12 10:18 — Written. The 139 boundary is stated as a **decision rule** rather than a
  description, because that is what a reader actually needs in the moment: if the app decided you
  were the wrong kind of device it is 139; if it knew what you were and the feature still failed it
  is this list.
- 2026-09-12 10:19 — Added two rows the PRD's §7.4 did not list, both because the simulation layer
  makes them *easier to forget*, not harder:
  * **Offline for real** — the panel's force-offline fails requests instantly; a real radio fails
    them slowly, which is a different thing to survive.
  * **Patchy coverage** — one bar and high latency, the exact state the offline design exists for
    and the one nothing here can fake. Noted that a train or a rural drive is how to get it.
- 2026-09-12 10:19 — Added a "when to run it" rule by area, since a checklist that claims to be
  needed on every release gets skipped entirely rather than selectively.
