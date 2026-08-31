# 152 — Crew placement map (section slug → directory population)

**Status:** done
**Priority:** high
**Created:** 2026-08-31
**Picked up by:** agent session (Zed)
**Started:** 2026-08-31
**Completed:** 2026-08-31

## Description

A crew member whose section slug is `bandit` is listed **in the bandit population**, in
their klan; slug `goeglerledelse` is listed **among the gøglere** (PRD 007 §6,
maintainer direction 2026-08-31). Their app role stays `crew`, so they still view as
crew and keep the patrol lookup.

**Placement and permission are two orthogonal mappings.** Both slugs already exist in
`go/nathejk/table/person/classify.go` mapping to `RoleCrew`, and that must stay true:

> Do **not** "fix" the classifier to map slug `bandit` → `RoleBandit`. It would hand a
> crew member the bandit pane and take away the patrol lookup. `classify.go` already
> carries a comment saying `goeglerledelse` is deliberately not `RoleGoegler`.

So this is a second map — slug → directory population — living next to the role map,
with a comment explaining why there are two.

This is **magic slug matching**, expected to migrate to a **section flag** upstream.
Same status as the role map after task 078: mechanism, not stopgap. Keep it in one
place, log unmapped slugs, and treat the eventual flag as a change of *source* rather
than of structure.

## Implementation

`go/internal/users/placement.go` + `placement_test.go`.

**Placement returns a set, not a value.** The task assumed one population per person.
That cannot express the requirement: a crew bandit appears in **both** the bandit
listing and the crew listing, and hiding them from either makes a real colleague
unfindable in the list where someone is looking for them. So `PopulationsOf(u) []Population`.

This resolved a question I expected to be awkward — *which* group is a crew bandit shown
in? The answer falls out of the manifest being built **per viewer**: it intersects the
person's populations with what that viewer may list. A bandit sees them grouped by klan;
a crew member sees them among crew; neither view needs to know the other exists. Task
153/154 own that intersection.

**`SectionSlug` added to `users.User`.** Only `SectionName` (the display label) was
exposed; placement needs the slug. `User`'s own doc says it is "the single place where
per-user attributes accumulate, so a new consumer needs a field here rather than its own
lookup path", so this is the sanctioned move. Wired through `personDirectory.toUser`.

## Deviation from the acceptance criteria

**No logging of unmapped slugs, deliberately.** The criterion asked for it "the same way
the role map logs them", which I wrote before reading the data model closely. It does not
apply here:

- In `classify.go`, an unrecognised slug means **classification failed** — the person
  falls back to least-privileged `RoleCrew` and someone should fix the data. Worth a
  warning.
- In placement, an unrecognised slug means **the person is crew**, which is the correct
  answer for hq, koekken, rover, hoensegaard, pr, team and noedtelefon alike. There is no
  failure to report, and logging every ordinary crew member would be noise that trains
  people to ignore the log.

The two maps look similar enough that this warranted a comment in the code, which it now
has, rather than a silent difference.

## Acceptance Criteria

- [x] A single placement map: slug `bandit` → bandit population, `goeglerledelse` →
      gøgler population, everything else → crew.
- [x] A comment at the map explaining the orthogonality, and why the role map must not
      be "corrected" to match.
- [x] The role map in `classify.go` is unchanged.
- [~] Unmapped slugs are logged the same way the role map logs them — **deliberately not
      done**; see the deviation above.
- [x] Tests: a crew member with slug `bandit` is placed in the bandit population, keeps
      role `crew`, and is listed in **both** the bandit list and the crew list.
- [x] A comment notes the expected migration to a section flag.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §6 / §8 / §11.3.
- 2026-08-31 12:10 — Picked up. Found `users.User` exposes `SectionName` but not
  `SectionSlug`; the slug is on `person.Person`. Adding the field to `User`, which its own
  doc nominates as the place for new per-user attributes.
- 2026-08-31 12:25 — Realised placement cannot be a single population: PRD 007 requires a
  crew bandit in both the bandit and crew listings. Changed to `PopulationsOf() []Population`.
- 2026-08-31 12:30 — That also answers "which group is a crew bandit displayed in": the
  manifest is per viewer, so it intersects. Recorded for tasks 153/154.
- 2026-08-31 12:45 — Decided against logging unmapped slugs, with reasoning in the code
  and in this file. Placement's default is correct, unlike classification's fallback.
- 2026-08-31 12:55 — ✅ All criteria met (one deliberate deviation). `go build ./...`,
  `go test ./internal/users/ ./cmd/api/` pass; `gofmt` clean.
