# 385 — Write the curator's half-page: the credential, the year, and what delete means

**Status:** open
**Priority:** medium
**Created:** 2026-09-22
**Picked up by:**
**Started:**
**Completed:**

## Description

Half a page, in Danish, for the two or three people who use this tool. PRD 022 §9 sets the binary
success condition: **a photographer uploads a full card unaided** — one photographer, one laptop, no
maintainer on the phone. *If it needs a shell, it failed.* A tool that needs its author present is not
finished, and the document is the last piece of that.

Three things it must cover, because each is a way the tool goes wrong silently:

- **The credential.** Where it comes from, that it is shared, that it goes over HTTPS only, and that
  rotating it is a config change and a redeploy (PRD 022 §8.2). This is also where PRD 022 §11 Q4 gets
  answered — who generates it, where it is stored, who is told, and what happens after the event. The
  default outcome if nobody decides is "it stays in a chat message until next year", and writing that
  down is what makes it a decision rather than a drift.
- **The year.** The tool writes to one configured event year and cannot write to another; changing years
  is a deploy-time change. The page states the year in large type for the reason PRD 022 §7 gives — it is
  the one thing that cannot be undone by editing.
- **What delete means.** The same distinction task 379 writes into the UI: removing a photograph from an
  album leaves it everywhere else; deleting it from the library removes it from all of them, and *that* is
  what an organizer means by "take it down". Say also that there is **no undelete button** in v1 (PRD 022
  §11 Q6) — the events are soft deletes so a maintainer can recover, but a mis-aimed "select all" is a
  phone call, not an undo.

Keep it short on purpose. This is read once, in a hurry, by a volunteer with a camera and 300 files to
hand in; a manual nobody finishes is the same as no manual.

## Acceptance Criteria

- [ ] Half a page, in Danish, checked in beside the PRD or in the repo docs
- [ ] Covers the credential, the year and the delete distinction, with no shell command required anywhere
- [ ] PRD 022 §11 Q4 is answered concretely — a named holder, a storage place, and a post-event action
- [ ] States that there is no undelete in v1 and what to do instead
- [ ] Read end to end by somebody who did not build the tool, who then uploads a batch unaided
- [ ] Any step that person got stuck on is either fixed in the tool or written into the page
