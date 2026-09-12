# 219 — The mobile-only smoke checklist

**Status:** open
**Priority:** medium
**Created:** 2026-09-12
**Picked up by:**
**Started:**
**Completed:**

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

- [ ] A checklist committed under `roadmap/`
- [ ] Each row states what to establish, not merely what to open
- [ ] Cross-references task 139's matrix and says how the two differ
- [ ] Referenced from `README.md` (task 218) and from PRD 014 §7.4

## Progress Log

- 2026-09-12 — Task created from PRD 014, phase 5.
