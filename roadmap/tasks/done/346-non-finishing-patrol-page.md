# 346 — The page for a patrol that did not finish

**Status:** done
**Priority:** medium
**Created:** 2026-09-19
**Picked up by:** agent
**Started:** 2026-09-21
**Completed:** 2026-09-21

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

- [x] A backstop-opened patrol's page renders with header, distance, scans, track and map, and **no diploma
      slot** — absent, not an empty frame.
- [x] No copy on the page refers to retiring, not finishing, or a missing diploma.
- [x] Finish time being null is handled everywhere it is read, without falling back to a misleading value.
- [x] The page is not opened earlier than the backstop for these patrols.
- [x] A test covers the backstop-opened state as a first-class rendering path, not as a degraded one.
- [x] Reviewed against the question: *would a patrol that got driven home be happy to send this to their
      family?* — done by reading the rendered page, not by inspecting the template. See the log.

## Progress Log

<!-- Append entries here — never edit or delete existing entries -->

- 2026-09-19 — Task created from PRD 011 §11 Q4 / §10 (Phase 3). Depends on 330, 341, 345.
- 2026-09-21 — **Done, and most of the page was already right — the copy was not.**

  Task 341 had already built the important half: `HasDiploma` follows the *finish*, not the gate, so a
  backstop-opened page renders no diploma slot at all rather than an empty frame, and nil finish times were
  handled at the one place they are read.

  **What was actually wrong was two sentences elsewhere.** The frontpage hint and the not-yet page both said
  a patrol's page appears *"når patruljen er i mål"* — true of the first trigger and silent about the second.
  Read by the family of a patrol that was driven home, that sentence says **"not for you"**. Both now name
  both ways a page opens: *"når patruljen er i mål — og senest når løbet er slut"*, and *"når løbet er slut,
  får alle patruljer deres"*.

  That is the whole change, and it is the kind this task existed to catch: the page was inclusive and the
  words around it were not.

  **Three tests**, one of which is the guard that matters: the copy must name *both* triggers, so a future
  tidy-up cannot collapse it back to the shorter, excluding sentence. The others assert a backstop page is
  complete (header, gruppe/korps, distance, registrations, map) and that nothing on it says *udgået*, *opgav*,
  *gennemførte ikke*, *intet diplom* or claims a finish.

  **The review was done by reading the page, not the template.** I rendered a backstop-opened page and read
  it as a parent would: number and name, gruppe and korps, *mindst ~5 km*, the map, the night's registrations
  in order, the takedown line, back to the frontpage. No apology, no gap where something should be. It is
  worth sending.

  **One observation, recorded rather than fixed.** In my fixture the scan list shows a registration at *Mål*
  while the header carries no finish time — because I forced the backstop while keeping the finish scan. That
  combination can occur for real: a scan that could not be attributed to the last checkgroup is a normal
  outcome (task 330's trigger 1 misses occasionally). The page then lists what was registered and the header
  says what the gate concluded, which is honest in both halves. The alternative — hiding a registration to
  make the page self-consistent — would be dropping a fact the patrol earned, so it stays.

  `gofmt`, `go vet`, `go test ./...` clean. No dev-stack verification: Docker is down on this machine, and
  this change is copy plus tests, with the rendered output read in full above.
