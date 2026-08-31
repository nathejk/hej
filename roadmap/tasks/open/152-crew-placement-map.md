# 152 — Crew placement map (section slug → directory population)

**Status:** open
**Priority:** high
**Created:** 2026-08-31

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

## Acceptance Criteria

- [ ] A single placement map: slug `bandit` → bandit population, `goeglerledelse` →
      gøgler population, everything else → crew.
- [ ] A comment at the map explaining the orthogonality, and why the role map must not
      be "corrected" to match.
- [ ] The role map in `classify.go` is unchanged.
- [ ] Unmapped slugs are logged the same way the role map logs them.
- [ ] Tests: a crew member with slug `bandit` is placed in the bandit population, keeps
      role `crew`, and is listed in **both** the bandit list and the crew list.
- [ ] A comment notes the expected migration to a section flag.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §6 / §8 / §11.3.
