---
name: prd
description: Author a Product Requirements Document (PRD) for a new feature in the tilmelding repo using the team's standard template and section structure. Use this whenever a user wants to spec, plan, or write a PRD for a new feature or significant change before implementation.
---

# Writing a Product Requirements Document

Use this skill to produce a Product Requirements Document (PRD) for a new feature
or significant change before implementation begins.

## When to use

- The user asks to "write a PRD", "spec a feature", "plan a feature", or similar.
- A new feature or non-trivial change is being proposed and needs an agreed
  definition before code is written.

Do **not** use this skill for small bug fixes or trivial changes — those belong
directly on the task board (`roadmap/tasks/`).

## Where PRDs live

Write finished PRDs to `roadmap/prd/` at the repo root, named with a zero-padded
sequence and a slug, mirroring the task board convention:

```
roadmap/prd/001-participant-self-service-signup.md
roadmap/prd/002-team-leader-dashboard.md
```

Check the highest existing number in `roadmap/prd/` before assigning a new one.
Create the `roadmap/prd/` directory if it does not yet exist.

## Process

1. **Gather context first.** Before drafting, make sure you understand:
   - The problem being solved and who has it (which user type — participant,
     team leader, organizer, etc.).
   - Any related existing code in `go/` (BFF) and `vue/` (frontend).
   - Whether the feature touches API endpoints, UI, or both.
   Ask the user targeted clarifying questions for anything you cannot determine
   from the codebase. Do not invent requirements.

2. **Draft the PRD** using the template in `prd-template.md` (in this skill's
   directory). Fill in every section; if a section genuinely does not apply,
   keep the heading and write "N/A" with a one-line reason rather than deleting
   it.

3. **Respect repo conventions:**
   - This is a backend-for-frontend setup: a Vue 3 (TS) frontend backed by a
     dedicated Go BFF. Call out clearly which side owns each piece of work.
   - **Every new or changed API endpoint must have OpenAPI annotations** — note
     this explicitly in the Technical Considerations section for any endpoint
     the feature introduces.
   - Use `YYYY-MM-DD` for all dates, matching the task board.

4. **Link to the task board.** A PRD defines *what* and *why*; the task board
   (`roadmap/tasks/`) tracks *execution*. In the "Rollout / Task Breakdown"
   section, list the concrete tasks that should be created in
   `roadmap/tasks/open/`, but do not create those task files unless the user
   asks.

5. **Review with the user.** After drafting, summarize the key decisions and
   open questions and ask the user to confirm before considering the PRD final.

## Template

The full template lives alongside this file at `prd-template.md`. Read it and
use it verbatim as the starting structure for every PRD.
