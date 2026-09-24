# 387 — "1 billeder" on the frontpage: the album count has no singular

**Status:** done
**Priority:** low
**Created:** 2026-09-23
**Picked up by:** agent
**Started:** 2026-09-24
**Completed:** 2026-09-24

## Description

The frontpage album card renders its photograph count with no singular form:

```
{{if .Count}}<span class="meta">{{.Count}} billeder</span>{{end}}
```

— `go/cmd/api/publicsite.go`, the `Albums` range in the frontpage template. An album holding one
photograph therefore reads **"1 billeder"**, which is wrong Danish. It should be "1 billede".

Found while verifying task 386 live: with `PUBLIC_ALBUMS=true` the dev frontpage showed "2 billeder" for one
album and "1 billeder" for another.

Small, and worth doing rather than shrugging at, for two reasons. It is on the **public** page — the one
surface read by families rather than by organizers — and a curator will hit it on the first album they build,
because an album starts with one photograph in it. It is also the kind of thing that reads as carelessness
about the whole page.

## Where else to check

Do not fix only the one line. The same construction is likely elsewhere, and a fix that leaves siblings
wrong is worse than none because it makes the remaining ones look deliberate:

- `publicsite.go`'s album card (the reported one).
- The album page itself, and the glimt strip's counts if any.
- `cmd/api/adminpage.go` and `adminalbumpage.go` — the curator's counts (`Kontaktark`, the album editor's
  item count, the selection counters, the delete confirmation's "you are about to delete N"). The delete
  confirmation matters most: task 379 made a point of the count being the thing a curator reads before
  confirming, and "1 billeder skal slettes" undercuts that.

A helper rather than an `{{if}}` at each site, since there will be more of these — but only if there are
genuinely several. One `{{if eq .Count 1}}` is not worth a helper.

## What was found

The reported line was the only one *missing* the singular. It was also the fifteenth copy of the rule: the same
choice was written out by hand at **six Go sites and eight JavaScript sites**, every one of them correct. So the
frontpage was not a careless line, it was a grammatical fact with no home — each new caller re-derived it, and one
eventually did not.

One sibling was genuinely wrong: the contact sheet's select-all note read "1 billeder lagt til valget — 1 valgte i
alt". Both numbers can be one.

And one fault was a step further on than the task described. After the count was fixed, the position message read
**"Position sat på 1 billede. *De* vises på kortet."** — the pronoun does not agree either. Found by reading the
live message for a one-photograph selection rather than by looking for it. Worth recording as the lesson: a plural
is not finished when the noun agrees.

## What was done

`cmd/api/plural.go` holds the three facts, called from Go and registered as template functions on **both** the
public site's and the admin tool's sets — the same function value, so one album's count cannot read one way on the
frontpage and another in the curator's list.

- `photoCount(n)` — "1 billede" / "12 billeder"
- `albumCount(n)` — "1 album" / "12 album". **Invariant in Danish**, and the function exists to say so once: the
  analogy with `billede` makes "albummer" look right, and it is not.
- `photoPronoun(n)` — "det" / "de"

JavaScript gets one copy, `ctx.photoCount` in `main.js`, because there is no build step on this surface and nothing
compiles the two together. Two is the floor, and `TestTheAdminCountsAgreeAcrossGoAndJavaScript` reads the
definition out of main.js and holds it to the Go one.

Not a general `pluralise()`: Danish inflection does not reduce to a suffix rule, and two of the three nouns this
product counts are invariant. Naming each noun keeps the irregularity in the language rather than in an argument.

## Two exemptions, both deliberate

`TestNothingWritesTheDanishPluralByHand` allows exactly two hand-written plurals, each with its reason, and checks
that each still matches something so a stale exemption cannot outlive the code it excuses:

- **main.js**, which is the definition.
- **The delete confirmation.** "slet 1 billede?" is a form; "slet dette billede?" is a question. A demonstrative
  does not compose with a count, and task 379 made the point that this is the one sentence a curator actually
  reads before confirming.

`den` in two position messages was left alone: it refers to the **position**, which is singular whatever the
selection holds. Asserted, so a later tidy-up cannot sweep it into `photoPronoun`.

## Acceptance Criteria

- [x] An album with one photograph reads "1 billede" on the frontpage
- [x] Every other count in the public site and the admin tool checked, and any with the same fault fixed
- [x] A test covers the singular, not only the plural — the existing frontpage tests assert "2 billeder" and
      would pass with the bug intact
- [x] Verified live on the dev frontpage with a one-photograph album
