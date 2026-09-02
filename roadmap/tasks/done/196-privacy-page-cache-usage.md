# 196 — Say what the app has cached, and what for, on the privacy page

**Status:** done
**Priority:** medium
**Created:** 2026-09-02
**Picked up by:** agent session (Zed)
**Started:** 2026-09-02
**Completed:** 2026-09-02

## Description

Maintainer, 2026-09-02: *"On the 'Data og privatliv' page there is a status section, state here how
much cache is used and what it's used for."*

`PrivacyView.vue`'s **"Hvad ligger på din telefon lige nu?"** section currently accounts for exactly
one thing: the position track, in points. Since PRD 009 the app also holds map tiles, a contacts
directory, portrait thumbnails and the app shell — hundreds of megabytes on a prepared phone — and the
page that claims to answer "what is on your phone" does not mention any of it.

That is a gap in the page's own promise, not a feature request. This is the only written account of
what the app stores that most parents will ever see; the start-area briefing reaches the participants,
not them.

### Not a second readiness view

The profile page already lists these datasets (task 187), and PRD 009 §7 is explicit that there is
**one** readiness surface. This is not that. The two answer different questions and the wording should
make it obvious which is which:

| page | question | tone |
|---|---|---|
| profile → *Klar til offline* | am I ready to go into the woods? | operational: sizes, staleness, "Hent nu" |
| privacy → *Hvad ligger på din telefon* | what does this app keep about me and other people, and why? | accountability: what, what for, how long |

So: no sync buttons here, no progress, no staleness. Sizes, purposes, and when it deletes itself.

### Voice

The file's own rule, and it is stricter than the rest of the app: readable by a 12-year-old **and**
their parent, plain Danish, no legalese. "Cache" is not a word that belongs on this page. Nor is a
dataset name that means nothing to a reader — "Kortbilleder", not "tiles".

## Acceptance Criteria

- [x] The status section states, per stored dataset: a plain-Danish name, how much space it uses, and
      **what it is for** in one sentence. *A new section, "Hvad fylder appen på telefonen?", rather
      than more prose inside the track's — see the log.*
- [x] A total, so the answer to "how much of my phone is this app using" is one number.
- [x] Rows appear only for datasets that actually hold something.
- [x] The purposes live in `config/offline.ts` next to the budgets, not in the template.
- [x] The **14-day self-deletion** is stated in "Hvor længe gemmer vi det?".
- [x] No sync or clear controls, and no staleness — only a link to the readiness view.
- [x] No `phoneParent` anywhere near it.

## Progress Log

- 2026-09-02 — Task created from maintainer direction.
- 2026-09-02 — **A separate section rather than more sentences in the track's.** "Hvad ligger på din
  telefon lige nu?" is about the user's *own* recording and reads as a running commentary on it
  ("appen optager netop nu…"). Storage is a different question with a different answer, and folding
  five datasets into that paragraph would have buried the track — the one thing on this page that
  cannot be re-fetched — in a list of things that can. So: "Hvad fylder appen på telefonen?",
  immediately after it.
- 2026-09-02 — **Purposes moved into `config/offline.ts`.** The template was the obvious place and the
  wrong one: the profile page describes the same five datasets, and two templates describing one thing
  is how they come to disagree. A test now asserts every dataset has a label and a purpose, that the
  purpose is a sentence rather than a noun, and that neither contains jargon — no "cache", no "tiles",
  no "sync". This page's rule is that a 12-year-old and their parent can read it, and the words the
  code uses fail that even when they are accurate.
- 2026-09-02 — Renamed the tile dataset's label from "Kort" to **"Kortbilleder"**. "Kort" is the name
  of a *feature* in this app, and a row saying the map takes 300 MB invites the reading that the map
  itself is the problem rather than the pictures it is made of.
- 2026-09-02 — **The total comes from `navigator.storage.estimate()` when available**, not from summing
  our own rows. Ours are per-dataset estimates and they miss whatever the browser counts that we do not
  know about — and on a page whose whole purpose is accountability, the number that is actually true
  beats the number that adds up.
- 2026-09-02 — Empty state written for the common case rather than as an error: a phone that has stored
  nothing yet is told it happens by itself when the app is used, because it does.
- 2026-09-02 — ✅ All criteria complete. 2 new tests (13 in `offline.spec.ts`); suite 325 across 27
  files; `type-check` and `build` clean.
