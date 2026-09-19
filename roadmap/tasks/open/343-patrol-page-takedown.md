# 343 — Takedown affordance on the patrol page

**Status:** open
**Priority:** medium
**Created:** 2026-09-19
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 011 §6. A visible route to "take this down" on a patrol's public page.

**Not because the design is in doubt** — PRD 011 §0b.1 settled that a patrol's merged route may be public,
and the right to publish the tracks has been obtained. This exists because a page can be *wrong* (a scan
attributed to the wrong patrol, a track that is not theirs, a name that changed), and because a patrol may
have a reason nobody anticipated. A public page about a group of children with no way to write to anyone is
the kind of thing that turns a small problem into an angry one.

The public glimt page already has this affordance (`glimtpublic.go`'s footer: *"Er der et billede, der ikke
skal ligge her? Skriv til os, så tager vi det ned."*). Match its tone and mechanism rather than inventing a
second reporting path — and note PRD 019 already built an anonymous report endpoint with a by-IP rate limit
and a `publicReporterSentinel` for the audit trail. Reuse that thinking; a patrol-page report is the same
shape of thing.

Two details worth carrying from PRD 019's reasoning:

- **Do not record the reporter's IP** as an identifier. PRD 019 chose a sentinel instead, accepting that
  several anonymous reports collapse into one row, because recording an IP would put a personal identifier
  of someone outside the app into an append-only audit table to solve a duplicate-counting problem that does
  not matter.
- **The cost of a spurious report is a moderator's glance; the cost of a throttled one is a page somebody
  objected to staying up.** So be generous with the limit.

Whether a report auto-hides a patrol page (as it does a glimt) or only notifies is a judgement for this
task: a patrol page is not a photograph, and hiding one on a single anonymous report is easier to abuse.
Recommend notify-only, and record the reasoning either way.

## Acceptance Criteria

- [ ] A visible, plainly-worded takedown route on every patrol page, matching the glimt page's tone.
- [ ] Reuses PRD 019's anonymous-report shape rather than a second mechanism.
- [ ] No reporter IP stored as an identifier; sentinel used, following `publicReporterSentinel`.
- [ ] By-IP rate limited, generously (see task 347).
- [ ] The auto-hide-vs-notify decision is made and its reasoning recorded in this task's log.
- [ ] Reachable and usable with JavaScript disabled.
- [ ] Consistent with the copy written in task 331, which also names a route to ask for removal.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §6 / §10 (Phase 2).
