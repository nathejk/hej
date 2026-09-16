# 279 — Restyle the edge arrows: bigger, thicker, red, no label or disc

**Status:** done
**Priority:** medium
**Created:** 2026-09-15
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-15
**Completed:** 2026-09-15

## Description

Product-owner styling change to the edge arrows: double the chevron's size, thicken it, make it red, and
drop the distance text and the white circle background.

Changes in `EdgeArrows.vue`:

- Chevron `h-4 w-4` → `h-8 w-8` (doubled), `stroke-width` 3 (thicker), `text-red-600`.
- Removed the distance `<span>` from the visual.
- Removed the white disc (`rounded-full bg-white/95 shadow-md ring-1`).

Two things kept deliberately, not oversights:

- **The 48 px button stays** as the tap target, transparent, with the smaller arrow centred in it. Tapping
  an arrow pans to the post, and shrinking the touch area to the visible 32 px arrow would drop it below the
  ≥ 44 px guideline.
- **Distance stays in the `aria-label`.** It is removed from the *visual*, as asked, but a screen-reader
  user has no other source for it, so the accessible label still reads "Post 1A, 3,4 km mod nordøst".

Added a thin white `drop-shadow` halo. It is not a re-introduced background: without the disc, a red chevron
would vanish against the pink roads on the topo layer and darken into the aerial, so the halo is the minimum
that keeps it legible on both. Called out in a comment so it is not "cleaned up" as a leftover.

## Acceptance Criteria

- [x] Chevron doubled in size and thicker.
- [x] Chevron is red.
- [x] Distance text removed from the visual.
- [x] White circle background removed.
- [x] Tap target and accessible label preserved.
- [x] Frontend suite, type-check and build clean.

## Progress Log

- 2026-09-15 — Applied the four visual changes. Kept the tap target and the aria-label, and added a white
  drop-shadow halo for legibility on both base layers now that the disc is gone — noted in the component so
  the reason survives.
- 2026-09-15 — The overlay's structural tests (button, `aria-label`, `aria-hidden`, pointer-events) still
  hold, since none of that markup changed. ✅ 611 frontend tests, type-check and build clean.
- 2026-09-15 — Open question for the device pass: whether a red chevron on the topo layer's pink roads is
  distinct enough even with the halo. It is the one combination I would look at first on a screen. If it
  reads poorly, the halo can thicken or the fill can gain a dark outline — one line either way.
- 2026-09-15 — Done.
