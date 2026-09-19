# 346 — The page for a patrol that did not finish

**Status:** open
**Priority:** medium
**Created:** 2026-09-19
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 011 §11 Q4. A patrol that retired, or was driven home, or was never scanned at the finish, has no scan at
the last checkgroup — so task 330's trigger 1 never fires for them. Their page arrives with the **backstop**:
when the last checkpoint closes, every patrol's page opens.

**That already settles the important half**, and it is worth being clear about why it is a good outcome:
nobody is excluded from the feature for not finishing, and **nothing has to publish the fact that they
retired** in order to give them a page. Plenty of Nathejk patrols do not finish, and those patrols still
walked most of a night.

What is left is copy and the diploma slot. Such a page has a track, scans and a distance, but no finish time.

**Recommendation (PRD 011 §11 Q4):** show where they went and how far, make the **diploma slot absent rather
than apologetic**, and let no copy anywhere draw attention to what is missing. A patrol that walked seven
hours and got driven home does not need a page explaining that to their family. The absence of a diploma is
information enough for anyone who is looking for it, and silence is kinder than a sentence about retiring.

**What not to do:** do not open the page earlier for these patrols to compensate. PRD 011 §0b.3's
finish-line reasoning — that a finished patrol's positions disclose nothing useful, because everyone already
knows where the finish is — **does not transfer** to an intermediate post. Opening on a patrol's first scan
would publish part of the course while the field is still walking it, which is a different decision and not
this task's to make.

## Acceptance Criteria

- [ ] A backstop-opened patrol's page renders with header, distance, scans, track and map, and **no diploma
      slot** — absent, not an empty frame.
- [ ] No copy on the page refers to retiring, not finishing, or a missing diploma.
- [ ] Finish time being null is handled everywhere it is read, without falling back to a misleading value.
- [ ] The page is not opened earlier than the backstop for these patrols.
- [ ] A test covers the backstop-opened state as a first-class rendering path, not as a degraded one.
- [ ] Reviewed against the question: *would a patrol that got driven home be happy to send this to their
      family?*

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §11 Q4 / §10 (Phase 3). Depends on 330, 341, 345.
