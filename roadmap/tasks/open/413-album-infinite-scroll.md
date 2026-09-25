# 413 — DEFERRED: the IntersectionObserver that appends the next page

**Status:** open
**Priority:** low
**Created:** 2026-09-25
**Picked up by:**
**Started:**
**Completed:**

## Description

**Do not start this task until task 400 reports a number that justifies it — the same blocking condition as
task 412 — and it depends on task 412 for the fragment it appends.** PRD 023 §2a.4.

An `IntersectionObserver` near the end of the grid fetches the next `side` from the items fragment endpoint and
appends its tiles, so a long album scrolls continuously instead of being pressed through a link. The viewer's
list grows with it, which also resolves the edge case §5 records: today the viewer's "next" is **disabled at the
end of what is loaded**, and with the observer it becomes "next loads the following page".

### The floor stays in the markup

§6 is explicit and it is the requirement most likely to be quietly dropped as redundant: **if the observer is
built, the "Vis flere" link stays in the markup.** *An observer that never fires must not be the only way to
reach photograph 201.* It can fail to fire for reasons we do not control — a script that did not load, a browser
extension, a zero-height sentinel after a layout change, JavaScript switched off entirely — and every one of
those turns a public album into a page that silently ends at 200 photographs with no indication there are more.
A visible link costs one line of markup and removes that whole class of failure, so it is not duplication: it is
the server-rendered floor the enhancement stands on, exactly as PRD 011 §8 and PRD 023 §3 require.

The obvious care points: the link is hidden or repurposed only once the observer has actually taken over,
appending must not disturb the scroll position or the viewer's current index, and a failed fetch has to leave the
link usable rather than leaving the reader on a grid that has stopped growing for no visible reason.

## Acceptance Criteria

- [ ] **Blocked: not started until task 400's measurement in PRD 023 §2a justifies infinite scroll, and task 412
      has landed.** If the measurement says the cap is enough, this task is closed instead of implemented
- [ ] Scrolling to the end of the grid appends the next page's items, and the viewer's list grows with them so
      "next" reaches the appended photographs
- [ ] The `<a href="?side=N">Vis flere</a>` link remains in the server-rendered markup, and with JavaScript
      disabled it is still the way to reach photograph 201
- [ ] A failed or empty fragment fetch leaves the "Vis flere" link usable and says nothing misleading
- [ ] Appending does not move the scroll position or reset the viewer's current index

## Progress Log

- 2026-09-25 — Task created from PRD 023.
