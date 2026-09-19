# 345 — The diploma

**Status:** open
**Priority:** medium
**Created:** 2026-09-19
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 011 §8, §11 Q6. The diploma thumbnail on the patrol page, and the full diploma behind it.

**The constraint that shapes this:** the org's architecture forbids a service calling another service's HTTP
API, so `hej`'s BFF **may not** call `diplom`. Three options, and PRD 011 §8 now leans (a):

- **(a) Link the visitor's browser to `diplom`.** A link is not a service-to-service call — the browser
  fetches it — so the rule holds. Cheapest, and reuses artwork and layout that already work. **The public
  frontpage makes this much better than it was:** the original objection was that bouncing a 12-year-old to
  another domain to log in again would kill the feature, and with no login anywhere on this surface that
  objection is gone. Remaining costs: a second origin, `diplom` is hardcoded to 2024 and needs the current
  year's data, and it still needs a thumbnail from somewhere.
- **(b) Render the diploma in `hej`** from its own projections. One origin, one deployment. Costs:
  reimplementing PDF/image generation (`go-pdf/fpdf` is small) and re-creating the artwork pipeline.
- **(c) Move generation into `hej` and retire `diplom`.** Cleanest end state, largest change, touches a repo
  outside this one.

**The finish time must use the same definition `diplom` already uses** — the scan at the last checkpoint —
or the page and the diploma will disagree about when a patrol finished. Conveniently that is also task 330's
trigger 1, so there is one definition of "finished" across the gate, the page and the diploma. Keep it that
way.

**Not in scope:** printing, posting or emailing the diploma (PRD 011 §4). In-page display, whatever the
browser's own share/save affords, and nothing else. No mail pipeline.

Open sub-questions to settle as part of this task: does the current year's artwork exist, who makes it, and
who renders the thumbnail.

## Acceptance Criteria

- [ ] An option from §11 Q6 is chosen and the reasoning recorded in this task's log and back in the PRD.
- [ ] A diploma thumbnail appears in the patrol page's slot (task 341) for a patrol that finished.
- [ ] The full diploma is reachable from the thumbnail without a login.
- [ ] The finish time matches `diplom`'s definition — the last-checkpoint scan — and is the same value the
      gate and the page use.
- [ ] A backstop-opened patrol (no finish) shows **no diploma slot at all**, not an empty frame — see task
      346.
- [ ] No service-to-service HTTP call from `hej` to `diplom`.
- [ ] Whether the current year's artwork exists is confirmed, and if it does not, who is making it is
      recorded here.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §8 / §11 Q6 / §10 (Phase 3).
