# 410 — The viewer picks its rendition with `srcset`, not `matchMedia`

**Status:** done (pending device verification, task 411)
**Priority:** medium
**Created:** 2026-09-25
**Picked up by:** agent
**Started:** 2026-09-26
**Completed:** 2026-09-26

## Description

PRD 023 §7.9 and §6 ("Functional — the viewer"). **Depends on task 409**, which produces the 800 px rendition
this task learns to ask for.

The viewer's `<img>` gets both URLs with their widths and a `sizes` hint, and the browser chooses:

```html
<img srcset="…/media/12?variant=medium 800w, …/media/12 1600w"
     sizes="(max-width: 40rem) 400px, 100vw" …>
```

**Not `matchMedia` in `viewer.js`**, for two reasons §7.9 gives:

- **Device pixel ratio is part of the decision and we do not have to think about it.** A 390 pt phone at 3× and
  an iPad at 2× want different answers to "is this a small screen", and `srcset` already knows both numbers.
- **Rotation and window resizing are free.** A JS branch taken once at open is wrong the moment a phone turns
  sideways — and this is a feature explicitly for both desktop and mobile (§2).

### The `sizes` value is a deliberate lie, and it needs a comment saying so

A phone's slot really is ~100vw, and declaring that on a 3× display would ask for ~1170 px and therefore pick
the **1600 w** candidate — re-introducing exactly what this whole change removes. Declaring `400px` caps a
narrow viewport at the 800 px rendition, which is still a genuine 2× image on a 390 pt screen. Only a 3×
flagship gives up anything, and at 800 px over 390 pt it is not perceptible.

§7.9 is explicit: **this is a known technique and it is a lie about layout, so it gets a comment** in
`viewer.css` / `viewer.js` saying so — otherwise the next person reads a wrong number, "fixes" it to `100vw`,
and the regression is invisible because the pictures still look fine.

**Prefetching follows the same choice** (§6): two ahead and one back, **of the variant this viewport uses**.
Prefetching 1600 px images to a phone that will then display 800 px ones would be this task's own bug arriving
by the back door, so derive the prefetch URL from what the `<img>` actually resolved (`currentSrc`) rather than
from a second, independent guess.

Photographs uploaded before task 409 have no medium rendition and fall back to the display image — same rule and
same reason as the existing missing-thumbnail fallback. A rendition is an optimisation, and losing one costs
bandwidth rather than the photograph.

§9's clearest single signal that this works: **a phone fetches the 800 px rendition, not the 1600 px one**,
checkable in a network panel. Task 411 checks it on real devices.

## Acceptance Criteria

- [x] The viewer's `<img>` carries `srcset` with both candidates and their real widths, plus the `sizes` hint;
      no `matchMedia` or window measurement decides the rendition
- [x] A comment in `viewer.js`/`viewer.css` states that `sizes` deliberately under-declares the narrow-viewport
      slot, and why, so it is not "fixed" to `100vw`
- [ ] A narrow high-DPR viewport fetches the 800 w candidate and a laptop fetches the 1600 w one, verified in a
      network panel — *manual, belongs to task 411*
- [x] Prefetch — two ahead, one back — requests the same variant the current photograph resolved to
- [x] A photograph with no medium rendition renders from the display image with no gap and no extra request

## Progress Log

- 2026-09-25 — Task created from PRD 023.
- 2026-09-26 — Implemented in `viewer.js`. `pictureFor` keeps its job as the one plain-URL decision (the
  `srcset`-less fallback and the filmstrip use it); a new `applyPicture(img, item)` sets
  `srcset="…?variant=medium 800w, … 1600w"` and `sizes="(max-width: 40rem) 400px, 100vw"` together with `src`,
  and *removes* both attributes when there is no medium rendition — the overlay reuses one `<img>`, so leaving
  them would carry the previous photograph's candidates over. The under-declared `sizes` carries the comment
  §7.9 asks for.
  Prefetch moved from `show` to the `<img>`'s `load` handler, because that is the first moment `currentSrc`
  reports which rendition this viewport resolved to; the neighbour's variant is matched against the current
  item's own `data-medium` (a suffix match — the shared viewer still knows no endpoint, PRD 023 §7.7). Still
  two ahead, one back. Four guards added to `go/cmd/api/viewer_test.go`; `go test ./cmd/api/` green.
