# 385 — Write the curator's half-page: the credential, the year, and what delete means

**Status:** doing
**Priority:** medium
**Created:** 2026-09-22
**Picked up by:** agent (the page); **blocked on the maintainer** for §11 Q4's name and the read-through
**Started:** 2026-09-23
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

- [x] Half a page, in Danish, checked in beside the PRD or in the repo docs
- [x] Covers the credential, the year and the delete distinction, with no shell command required anywhere
- [ ] PRD 022 §11 Q4 is answered concretely — a named holder, a storage place, and a post-event action —
      **the shape is written, the name is not mine to supply**
- [x] States that there is no undelete in v1 and what to do instead
- [ ] Read end to end by somebody who did not build the tool, who then uploads a batch unaided
- [ ] Any step that person got stuck on is either fixed in the tool or written into the page

## What was written

[`docs/billedarkiv-for-fotografer.md`](../../../docs/billedarkiv-for-fotografer.md) — about a page, in
Danish, linked from the README as the thing to hand over with the password.

It opens by saying you will not need a terminal for any of it, and that **if you ever think "a developer has
to do this bit", that is a bug in the tool — say so.** That sentence is there because PRD 022 §9's success
condition is binary and phrased exactly that way: *if it needs a shell, it failed.* Putting it at the top
turns a photographer's confusion into a bug report instead of a quiet workaround, which is the only way the
remaining two criteria ever get met.

The five sections are the three the task asks for plus the two things that actually happen in between:

1. **Login.** Shared, not personal, and the page says so plainly — *it does not record who uploaded what,
   the tool never writes your name anywhere*. That is not reassurance, it is the truth of §8.2, and a curator
   who believes otherwise will eventually be surprised by it. HTTPS only, changed after the event, rotation
   is a developer's five minutes and not something they can do themselves. With the note that this login is
   interim and role-based access is coming.
2. **The year.** "Look at it the first time you open the tool." Then the reason, as an instruction rather
   than an explanation: *if the year is wrong, stop and say so — everything else on the page can be fixed,
   this cannot.* The tool already gives the year its own size and accent colour for the same reason (PRD 022
   §5), so the page and the screen agree.
3. **Uploading.** Drag the card in; duplicates are skipped so re-dragging the whole card is safe; non-images
   are refused and that is not a failure. And the disk-full case from task 384, as the only instruction in
   the document printed as a stop: **behold kortet** — do not clear the card before the photographs are up.
4. **Building an album.** With the sentence the whole privacy design rests on: the photographs are in the
   archive but *nobody can see them yet, and that is deliberate — a photograph reaches the site because
   somebody chose it*. Then the **Uden album** filter as the answer to "what have I not sorted yet", and the
   honest description of an out-of-area coordinate: the photograph still appears, only the map pin does not.
5. **Removing versus deleting.** A two-row table, then one line on its own: **"Slet fra arkivet" is the one
   to use when somebody has asked for a photograph to be taken down.** Not the other.

And the no-undelete paragraph (§11 Q6), written as what to do rather than as a warning: you cannot get it
back, a developer can but by hand, so if a **Vælg alle der matcher filteret** went wrong, ring someone now
and say how many and when. It ends on *read that number* — the confirmation always states the count.

A short troubleshooting list closes it: the five things that will actually happen, including the rate limiter
("you have tried too many times; wait a quarter of an hour"), which otherwise reads as the tool being broken.

## Deliberately not duplicated into the tool

The obvious next step is a "Hjælp" panel on `/admin` carrying this text. Not done, and not from laziness:
**two copies of the delete distinction would drift**, and the copy that matters is the one next to the button
— task 379 put it there. A help panel repeating it in slightly different words is how the two come to
disagree, and the version a curator reads under pressure is the one on the button.

The page travels with the credential instead. Whoever hands over the password hands over this, in the same
conversation, and §11 Q4's unresolved half is precisely about naming that person.

## What remains, and why it cannot be finished here

1. **§11 Q4 needs a name.** The document says the credential comes from "arrangørgruppens tekniske
   ansvarlige", is not sent by mail, and is changed after the event. That is the *shape* of the answer and it
   is genuinely useful — but the criterion asks for a named holder and a storage place, and inventing either
   would be worse than leaving the gap visible. When it is decided, three lines change here and in PRD 022
   §11 Q4.
2. **The read-through is the point of the task.** "Read end to end by somebody who did not build the tool,
   who then uploads a batch unaided" cannot be self-certified — the author of a document is the one person
   who cannot test whether it is followable. The last criterion depends entirely on it: *any step that person
   got stuck on is either fixed in the tool or written into the page.* That is where this document earns its
   keep or gets rewritten, and it is a half-hour with a volunteer and a laptop.
