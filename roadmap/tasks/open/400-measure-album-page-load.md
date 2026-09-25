# 400 — Measure a real album page at the new tile size, and write the answer into PRD 023 §2a

**Status:** open
**Priority:** high
**Created:** 2026-09-25
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 023 §2a.4 and §9. **The deliverable of this task is a written answer in the PRD, not code.**

§2a defers infinite scroll behind a measurement rather than designing it away, and §8's risk list names the
failure mode plainly: *"a deferred decision becomes an invisible one"* — if the measurement is never taken,
the cap is the whole answer and nobody ever revisits it. This task exists so that "deferred" does not
quietly become "forgotten". `.rules`' own framing in §9 is the standard to meet: **"We never checked" is
the failure.**

What to measure: the **2025 import** (task 348), rendered by the album page at task 398's tile size, and
three numbers per page:

- **HTML bytes** — raw *and* gzipped, because §2a's table turns on the gap between them (~160 KB raw,
  ~20 KB gzipped for 400 near-identical tiles).
- **DOM node count** — §2a estimates ~1200 nodes and 400 lazy-load observers for a 400-item album; confirm
  or correct it.
- **Time-to-interactive on a real mid-range Android**, not a throttled laptop. This is the number the whole
  decision rests on, and it is the one a desktop devtools profile cannot fake.

Then record the answer **in PRD 023 §2a** — not here, not only in the progress log — as a dated paragraph
with the numbers and the conclusion. The conclusion is a decision, one of two:

1. The cap is enough, and tasks **412** and **413** are closed as unnecessary; or
2. It is not, and 412/413 are unblocked with this measurement quoted as their justification.

Either way §11's still-open question 2 ("Is the cap 200, and does infinite scroll get built at all?") stops
being open. If the number also says 200 is the wrong cap, say so — the PRD calls it "proposed".

Do this after task 398 has landed, since the tile size is half of what is being measured.

## Acceptance Criteria

- [ ] The 2025 import (task 348) is rendered through the real album page at the shipped tile size, not a
      synthetic fixture
- [ ] HTML bytes (raw and gzipped), DOM node count and time-to-interactive are recorded, with the Android
      device model and connection named
- [ ] PRD 023 §2a gains a dated paragraph with the numbers and an explicit decision on infinite scroll
- [ ] PRD 023 §11's open question 2 is marked answered, in the `~~struck~~` style that section already uses
- [ ] Tasks 412 and 413 are either closed with the reason, or unblocked with this measurement cited

## Progress Log

- 2026-09-25 — Task created from PRD 023.
