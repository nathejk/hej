# 412 — DEFERRED: the album items fragment endpoint

**Status:** open
**Priority:** low
**Created:** 2026-09-25
**Picked up by:**
**Started:**
**Completed:**

## Description

**Do not start this task until task 400 reports a number that justifies it.** PRD 023 §2a.4 defers infinite
scroll and its fragment endpoint behind a measurement, and §10 keeps both written down rather than deleting
them — *"decided against for now, with a number to revisit it at"* is worth more than silence. If task 400 finds
that a 200-item page is fine on a real mid-range Android, this task is **closed as unnecessary**, not shelved.

The reason for the block is the honest accounting in §2a: paging buys bounded server work, a bounded DOM and a
bounded viewer list, and **approximately nothing on transferred bytes** — `loading="lazy"` already does that.
Against those three it costs a fragment endpoint, an observer, a no-script path, a second rendering of the item
list to keep in step, and the `?foto=` page derivation. That may be a good trade. It is not obviously one, which
is why it waits for a measurement.

### If it is built

`GET /{year}/album/{slug}/side/{n}` (§8's endpoint table), rendered from **the same `{{define "album-items"}}`
the page uses** — task 403 introduces that define precisely so this endpoint is not a second rendering that can
drift. The guard that matters is therefore a test asserting **the fragment and the page render identical items**
for the same `side`; without it, the two renderings disagree eventually and the symptom is a photograph that
appears twice or not at all, days after the change that caused it.

Per §8 it is **not a JSON API**: it carries an OpenAPI annotation with `@Produce html`, like the public site's
other handlers, and documents that it answers the same 404/503/429 set as the page it belongs to.

Task 413 is the observer that consumes it and depends on this task.

## Acceptance Criteria

- [ ] **Blocked: not started until task 400 has recorded a measurement in PRD 023 §2a that justifies infinite
      scroll.** If the measurement says the cap is enough, this task is closed instead of implemented
- [ ] `GET /{year}/album/{slug}/side/{n}` returns the items for that page and nothing else — no page chrome
- [ ] It renders from the same `{{define "album-items"}}` as the page, with no second copy of the item markup
- [ ] A test asserts the fragment and the page render identical items for the same `side`
- [ ] It carries an OpenAPI annotation with `@Produce html`, documenting the same 404/503/429 set as the page
- [ ] It is excluded from the JSON API guards deliberately, with the reason recorded, as task 395's fragment
      routes were

## Progress Log

- 2026-09-25 — Task created from PRD 023.
