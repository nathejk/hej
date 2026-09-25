# 411 — QA pass on the baseline devices, because the Go suite cannot execute any of this

**Status:** open
**Priority:** medium
**Created:** 2026-09-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 023 §6 ("Non-Functional — Baseline") and §9. A manual pass over the shipped viewer on iOS/iPadOS Safari
16.4+ and Chrome 111+ (`.rules`' baseline), plus desktop Firefox for the fallback and one run with JavaScript
disabled.

**Why this matters more than a QA task usually does.** §8 states the testing reality: *"there is no way to
execute the viewer's JavaScript in this test suite."* The cap, the deep link, the per-photograph `href` and the
asset route are behavioural Go tests; everything else — the overlay, the filmstrip, the gestures, the fullscreen
gate, the share sheet, the rendition choice — is guarded only by source-reading tests, which are brittle by
construction and have produced false positives in this repo before. So this pass is **not belt-and-braces; it is
the only test most of this feature gets.** Treat a skipped device as an untested feature.

### The matrix, from §10's last item

| Device / browser | What specifically to confirm |
|---|---|
| **iPhone Safari** | **No fullscreen button at all** (iOS has no element fullscreen — §7.5); the native share sheet opens from the share control; **no filmstrip**, in portrait *and* landscape, since a landscape phone is a short viewport rather than a narrow one; and **the 800 px rendition is fetched, not the 1600 px one — check the network panel** |
| **iPad Safari** | The fullscreen button **is** present and works, including the prefixed path, and the filmstrip **is** shown — the two places iPadOS deliberately differs from iPhone |
| **Chrome desktop** | Fullscreen, filmstrip, keyboard `←`/`→`/`Esc`/`Home`/`End`, native share, and the 1600 px candidate chosen |
| **Desktop Firefox** | `navigator.share` is absent: the control copies and says *"Linket er kopieret"*, which clears itself |
| **JavaScript disabled** | The grid, the "Vis flere" link, one working `href` per photograph, and **opening a shared `?foto=` link** — the right page, scrolled to the right tile |

Also worth walking while there, since they are cheap and specified: an album with one photograph (no arrows, no
filmstrip), a photograph with neither caption nor credit (no panel at all rather than an empty grey box), a
display image that 404s (*"billedet er ikke tilgængeligt"*, and the viewer still moves on), and `Esc` in
fullscreen leaving fullscreen without closing the viewer.

§9's qualitative check belongs here too if the timing allows: a single sitting with the maintainer over a real
hand-in, judging whether a curator can sort 300 photographs without leaving the sheet. And §9's last metric —
**the viewer is used on both desktop and mobile** — is a judgement this pass is in the best position to make: if
the filmstrip decision or the layout makes one of them second-class, §2's instruction was not met and that is a
finding, not a nitpick.

Record what was checked on which device and OS version in the progress log, including the failures found and
where they went. A pass with no notes is indistinguishable from a pass that did not happen.

## Acceptance Criteria

- [ ] All five rows of the matrix are walked, with device models and OS versions recorded
- [ ] An iPhone is confirmed to fetch the 800 px rendition rather than the 1600 px one, from the network panel
- [ ] The iPhone/iPad fullscreen and filmstrip differences are confirmed to be the specified ones rather than
      accidental
- [ ] The no-JavaScript run reaches every photograph, opens one at full size, and opens a shared `?foto=` link on
      the right page
- [ ] The edge cases in PRD 023 §5 are walked and their Danish copy verified in place
- [ ] Every defect found is written up as its own task or fixed and logged here; "looked fine" is not a result

## Progress Log

- 2026-09-25 — Task created from PRD 023.
- 2026-09-25 — Sharpened by task 415, which found two bugs in first use that this pass was meant to catch: the viewer could not be closed at all (a cascade conflict on `dialog`), and a `?foto=` load opened it with no controls (an initialisation order). Every guard written for tasks 402–408 passed throughout, because they assert *decisions* — no surface check, no `files` in a share, the filmstrip hidden by media query — and no source-reading test can see a cascade conflict or an ordering. **Two checks from this pass are cheap enough to run on every change to the viewer, and should be:** open it and close it, and reload the page it leaves in the address bar.
