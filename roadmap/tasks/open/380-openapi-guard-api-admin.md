# 380 — Extend the OpenAPI guard to `/api/admin`, and annotate every endpoint

**Status:** open
**Priority:** high
**Created:** 2026-09-22
**Picked up by:**
**Started:**
**Completed:**

## Description

Every admin endpoint needs OpenAPI annotations — `@Summary`, `@Description`, `@Tags`, `@Produce`,
`@Success`, `@Router` and a documented `@Failure 401` — which `.rules` already requires of all
endpoints. The problem PRD 022 §8.9 identifies is that **nothing currently enforces it here**:
`isInScope` in `go/cmd/api/glimtopenapi_test.go` matches `/api/glimt*`, anything containing `glimt`,
`/api/public/` and year-prefixed paths. `/api/admin/*` is none of those, so annotations could be
forgotten, half-written or wrong and the suite would stay green.

So: extend `isInScope` to `/api/admin/`, and add a new tag `admin` to `allowedGlimtTags`. A separate
tag rather than reusing `public-site` for the reason that list already records about keeping
`glimt-public` apart from `public-site` — folding the curator's tool in with the public site would put
two things with entirely different rules in one section of the rendered spec.

Two mechanical constraints the AST-based guards impose, worth knowing before writing handlers rather
than after: **handler names must end in `Handler`**, and **route paths must be plain string literals**.
A path built by concatenation is invisible to the guard, which is the same as not being covered.

This is a narrowing of the broader `roadmap/tasks/open/328-openapi-guard-whole-api.md`, not a
replacement for it. Task 328 widens the guard to the whole API; this task does the slice PRD 022 needs
and should leave 328's case intact rather than closing it.

## Acceptance Criteria

- [ ] `isInScope` covers `/api/admin/` and `admin` is in `allowedGlimtTags`
- [ ] Every admin endpoint carries the full annotation set including `@Failure 401`
- [ ] Deleting an annotation from any admin handler fails the suite — verified by breaking it
- [ ] Every admin handler name ends in `Handler` and every admin route path is a plain string literal
- [ ] The rendered spec groups the admin endpoints under their own `admin` tag
- [ ] Task 328 is noted as still open and still broader than this change
