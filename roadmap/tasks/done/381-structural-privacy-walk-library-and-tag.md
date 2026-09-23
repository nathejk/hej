# 381 — Extend the structural privacy walk to the library and the tag

**Status:** done
**Priority:** high
**Created:** 2026-09-22
**Picked up by:** agent
**Started:** 2026-09-23
**Completed:** 2026-09-23

## Description

The structural privacy test (task 337, in `go/cmd/api/publicprivacy_test.go` —
`TestPublicAlbumReadModelHasNowhereToPutAPerson`, `TestPublicViewTypesHaveNowhereToPutAPerson` and
their siblings) must walk the new types too: the `photo` projection's row and event structs, the
`photo_patrol` tag, and every admin response type that reaches a public surface.

PRD 022 §6 Non-Functional states the requirement flatly: the library row has **no uploader, no curator,
no person id, no name, no phone number, and emphatically no `phoneParent`**. §9's last metric is that
this is enforced "by extending the task 337 structural test rather than by review", and that phrasing is
the whole point of this task. Review catches it the first time and not the fifth; a walk over the types
catches it on the compile after somebody adds a convenient field. Task 361's record shows this working
exactly that way — the structural guard fired on the first compile, which is what it was for.

The repo rule this defends is unusually hard: guardian phone numbers may exist on exactly one surface in
this project, a user confirming their **own** guardian's number, and nowhere else — not a directory, not
a contact list, not a cached manifest. A photo library with a `phoneParent` would be absurd, which is
precisely why nobody would notice a struct embedding that carried one.

There is an additional reason the absence is structural here rather than a policy: with a **shared
credential** (PRD 022 §8.2) the tool cannot honestly attribute anything to a person, so a curator or
uploader field would be a lie in a column as well as a privacy hazard.

## Acceptance Criteria

- [x] The walk covers the `photo` row type, all four/five `photo` event types, `photo_patrol`, and the
      admin response types
- [x] Adding a `name`, `phone`, `phoneParent`, `personId`, `uploadedBy` or `curator` field to any of
      them fails the suite — verified by adding one and removing it
- [x] The walk recurses through embedded structs, slices and maps, not just top-level fields
- [x] A SQL-level check asserts the new tables declare no person-shaped column
- [x] The test's failure message explains *why*, referencing task 337 and PRD 022 §6
- [x] No existing assertion from task 337 is weakened to accommodate the new types

## What was done

A new file, `go/cmd/api/libraryprivacy_test.go`, alongside task 337's rather than inside it — because it
asks a different question. Task 337 asks "can anything on the **public** surface carry a person?" This
asks whether anything in the library can, and the library is *not* public: it is behind a credential, and
it must still name nobody.

Three kinds of check, because each misses what the others catch:

1. **Enumeration from source.** Every struct declared in the `photo` package and in `cmd/api/admin*.go`,
   whether or not anybody remembered to list it. This is what covers the type added next month. Request
   types included deliberately: a person-shaped field on the way *in* is the likelier mistake ("let the
   uploader say who they are") and it is the one that writes the value into the event log, permanently.
2. **Recursion through the types.** `personShapedPaths` walks named struct fields, pointers, slices and
   maps, returning **dotted paths** — `photo.Uploaded.Location.PhoneParent`, not `"PhoneParent"`, which
   would send a reader to the wrong struct. An enumeration of locally declared structs cannot see a field
   whose type comes from another package; the recursion can.
3. **The schema.** A Go struct with a bad field is fixed in a commit. A column written to since Friday
   holds the values, in the database and in every backup taken since. So the structural walk is necessary
   and not sufficient.

Each check has a **floor assertion** — at least five structs, at least ten columns, both tables found —
because the way any of these stops working is by finding nothing, and finding nothing would otherwise be
indistinguishable from finding nothing wrong.

## The gap this found, which is the reason for the task

The first break-test was `UploadedBy` on `photo.LibraryPhoto`. **It passed.** None of
`isPersonShaped`'s needles — `person`, `phone`, `portrait`, `photo`, `author`, `curator`, `uploader` —
matches `uploadedby`. The single field the acceptance criteria name by example was invisible to the check,
and only adding it and watching the test go green was ever going to surface that.

So `isPersonShaped` now flags **anything ending in `By`**. A suffix rather than a needle, because in this
codebase `…By` names an actor without exception (`HiddenBy`, `hiddenBy`, `uploadedBy`) — and an actor is
exactly what PRD 022 §8.2 makes impossible to fill honestly: the credential is shared, so there is nobody
to name and a value in that column would be invented. A false positive (`standby`, `nearby`) fails loudly
and gets excepted with a reason, which is the direction this trap should err in.

## Three false positives, and why they were excepted rather than smoothed over

The blunt `photo` and `curator` needles fired on:

- `adminLibraryResponse.Photos` — a collection of photographs.
- `adminAlbumItemView.PhotoDeleted` — the flag that *prevents* a deleted photograph being rendered.
- `photo.Table.curatorQuerier` — an embedded **unexported** querier.

The first two joined the allowlist by name, next to `photoId`, with the reasoning — following the rule that
file already states: except by name, never loosen the needle, so the trap stays set for a genuine
`PhotoOfPerson`. The third was not allowlisted, because allowlisting a private implementation detail is
the wrong repair: the AST walk now **skips unexported fields**, matching the reflection walk's
`PkgPath != ""` rule and for the same reason — an unexported field cannot be serialised into a response or
rendered into a template, so it cannot leak.

## Verified by breaking it

Each reached a different mechanism, and each was reverted:

| Sabotage | Caught by |
|---|---|
| `UploadedBy` on `photo.LibraryPhoto` | nothing, first time — see above; now both walks |
| `PhoneParent` on `photo.Location` (nested two deep) | the recursion, reporting `photo.Uploaded.Location.PhoneParent` and `photo.Updated.Location.PhoneParent` |
| `CuratorName` on `adminLibraryPhoto` | the admin-file enumeration |
| `taggedBy` column on `photo_patrol` | the SQL check |

## Notes

- **Nothing from task 337 was weakened.** The two allowlist entries (`photos`, `photoDeleted`) are names
  no type on the public surface carries — that suite passed before the entries existed and passes after —
  and the `…By` suffix rule only ever adds failures. The public walk, its fixture-value assertions and
  `TestPublicPatrolTypeHasNowhereToPutAPerson`'s exact field list are untouched.
- **"The admin tool" is a filename prefix, not a package.** The curator's handlers live in `package main`
  with every other handler, so the enumeration selects `admin*.go`. Worth stating rather than hiding: a new
  admin file must be named `admin*` to be covered — the same convention `curatorboundary_test.go`'s
  `adminOwnedFiles` allowlist already depends on.
- The SQL column parser is a regex, which is adequate because the question is only whether a person-shaped
  *name* appears as a column: a false positive fails safely, and a parser that stopped matching would
  report zero columns, which the floor assertion turns into a failure rather than a pass.
