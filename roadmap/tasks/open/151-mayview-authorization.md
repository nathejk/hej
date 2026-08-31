# 151 — `mayView(viewer, subject)` authorization for contacts

**Status:** open
**Priority:** high
**Created:** 2026-08-31

## Description

The single server-side authorization function behind every contacts response (PRD 007
§6 access matrix, §8 BFF). One function, used by the manifest, the photo handler, the
profile and the patrol lookup — if this logic exists in two places it will diverge, and
the failure mode is either a privacy breach or a broken game.

Matrix (PRD 007 §6):

| Viewer | spejder | bandit | gøgler | crew |
|---|---|---|---|---|
| spejder | — no pane — | — no pane — | — no pane — | — no pane — |
| bandit | ❌ | ✅ all | ❌ | ✅ all |
| gøgler | ❌ | ❌ | ✅ all | ✅ all |
| crew | ⚠️ patrol lookup only | ✅ all | ✅ all | ✅ all |

Notes that matter:

- `samarit` / `guide` / `postmandskab` are all crew and are not distinguished.
- Spejder is never a viewer, and never appears in a directory listing or search result.
- Crew reach spejdere **only** through the patrol lookup (task 157), never the
  directory — so the function needs to distinguish "may list" from "may look up".
- Placement (which population you are listed in) is task 152 and is *not* this
  function's job. This function answers permission only.

## Acceptance Criteria

- [ ] `mayView` (or equivalently named pair, e.g. `mayList` / `mayLookUp`) lives in one
      place in the BFF and is the only authorization path for contacts.
- [ ] Table-driven tests cover **every** viewer/subject role pair in both directions,
      including `gøgler` and generic `crew`.
- [ ] A test asserts spejder is denied as a viewer on every contacts surface.
- [ ] A test asserts the spejder/bandit pair is denied symmetrically — game integrity,
      not just privacy.
- [ ] A test asserts crew may look up a spejder but may not list one.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §6 / §8.
