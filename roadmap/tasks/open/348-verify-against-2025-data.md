# 348 — Verify the whole public surface against 2025 data

**Status:** open
**Priority:** high
**Created:** 2026-09-19
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 011 §9. The acceptance pass for the whole feature, done against **2025** — which is complete on the
stream, while 2026 is empty. This is a real advantage and should be used rather than waited out: every part
of this feature can be seen working on real data before the event.

Three things to verify, and they fail in different ways:

**1. It works on old and small devices.** The entire justification for server-rendered Go templates is reach
— so a parent on a ten-year-old browser can see what the weekend looked like. Verify on the oldest device
available (iPad mini 2, iOS 12.5.8, per task 204), on a phone viewport, **with JavaScript disabled**, and
with CSS disabled. The only thing that may disappear without JS is the map island (task 342); everything
else must be complete.

**2. The numbers are honest.** The distance figures for real 2025 patrols must sit in the same range as the
course's planned length (task 339). The track coverage the page implies must be consistent with what task
082 measured on the device — **the page must not be quietly hiding gaps**, which is the failure mode that
would make this feature dishonest rather than merely buggy.

**3. It discloses nothing it should not.** Task 337's assertions pass, and a human also *looks* — at the
rendered HTML of a real patrol page, for a real name, a real phone number, or an attributed individual
track. Automated assertions catch the fields they know about; a person reading the output catches the one
nobody thought to name.

Also worth a deliberate look, because it is the most-served screen of the whole feature: **the not-yet page**
(task 330/341), in the state it will be in for most of the year, and its indistinguishability from an
unknown patrol number.

## Acceptance Criteria

- [ ] Frontpage, an album page, a patrol page and the glimt strip all verified against 2025 data on a phone
      viewport.
- [ ] Verified on the oldest device available, and with JavaScript disabled, and with CSS disabled.
- [ ] Distance figures for a sample of real 2025 patrols recorded here alongside the course's planned length.
- [ ] Track rendering checked against task 082's measured coverage — gaps visible as gaps, not smoothed.
- [ ] A patrol with no track and a patrol with unpositioned scans both checked, since both are common.
- [ ] A backstop-opened (non-finishing) patrol page checked (task 346).
- [ ] The not-yet page checked, including that it is identical to the unknown-patrol answer.
- [ ] Rendered HTML of a real patrol page read by a human for personal data, in addition to task 337's
      automated assertions.
- [ ] Findings recorded in this task's log, whether or not they lead to changes.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §9 / §10 (throughout).
