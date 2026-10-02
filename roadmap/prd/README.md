# Product Requirements Documents

A PRD defines **what** we are building and **why**, before implementation starts.
Execution is tracked separately on the task board in `roadmap/tasks/` (see
`roadmap/tasks/TASKS.md`).

New features and significant changes require a PRD (`.rules`). Small bug fixes and
trivial changes go straight to the task board.

---

## Folder structure

```
roadmap/prd/
  draft/      ← being written; not yet agreed
  doing/      ← agreed and being implemented
  done/       ← shipped
```

**The folder is the status.** Moving the file between folders is how status
changes, and the `Status` field in the document header must always agree with the
folder it sits in. A PRD in `draft/` whose tasks are all `done` is a bug in the
board.

`draft/` and `doing/` may be missing — git does not track empty directories.
Create them as needed.

---

## Naming

Zero-padded sequence + slug:

```
draft/005-some-new-feature.md
doing/004-migrate-primevue-to-shadcn-vue.md
done/001-hej-nathejk-event-app-skeleton.md
```

The number is **permanent** and never changes as the file moves. Check the highest
existing number **across all three folders** before assigning a new one.

**Refer to a PRD by number, not by path** — in task files, commit messages and
code comments. Paths go stale as PRDs move; "PRD 004" does not.

---

## Lifecycle

### draft/ → doing/ (the approval gate)

Approval is the product owner's call, not the author's.

1. `Status: doing`
2. `Approved:` today's date
3. Bump `Last updated`
4. Move the file to `doing/`
5. Create the tasks from "Rollout / Task Breakdown" in `roadmap/tasks/open/`
6. Commit: `prd(004): approve — migrate PrimeVue to shadcn-vue`

### doing/ → done/

When every task derived from the PRD is in `roadmap/tasks/done/` and the feature
is shipped:

1. `Status: done`
2. `Shipped:` today's date
3. Bump `Last updated`
4. Move the file to `done/`
5. Commit: `prd(004): done — migrate PrimeVue to shadcn-vue`

PRDs stay in `done/` — they are the record of intent and decisions.

### While a PRD is in doing/

Requirements shift during implementation. When they do, edit the PRD and bump
`Last updated`; a PRD in `doing/` that no longer describes what is being built is
worse than no PRD. If a change is large enough to invalidate the agreement, move
the file back to `draft/` (reset `Status`, clear `Approved`) rather than quietly
rewriting an approved document:
`prd(004): reopen — scope changed, needs re-agreement`.

---

## Commit messages

```
prd(<number>): <action> — <short title>
```

Actions: `create` · `update` · `approve` · `done` · `reopen`

This mirrors the task board's `task(<id>): <action> — <title>`.

---

## Writing one

Use the **`prd`** skill (`.agents/skills/prd/`). The template lives at
`.agents/skills/prd/prd-template.md` — fill in every section; if one genuinely
does not apply, keep the heading and write "N/A" with a one-line reason.

Repo specifics to respect:

- This is a backend-for-frontend setup (Vue 3 + TS frontend, Go BFF). Say clearly
  which side owns each piece of work.
- **Every new or changed API endpoint needs OpenAPI annotations** — call this out
  in Technical Considerations, or state explicitly that there are no endpoint
  changes.
- Dates are `YYYY-MM-DD`.

---

## Current PRDs

| # | Title | Status |
|---|---|---|
| 001 | "Hej Nathejk" Event App Skeleton (PWA shell + phone login) | done |
| 002 | Event Map (own position, Danish topo + aerial layers, patrol scan history) | done |
| 003 | Profile Page (own details, self-portrait, device permission status) | done |
| 004 | Migrate the component library from PrimeVue to shadcn-vue (and upgrade Tailwind to v4) | done |
| 005 | Install-first mobile onboarding (install, confirm, permissions) | done |
| 006 | Member directory for the app (person lookup by phone, app roles) | done |
| 007 | Contacts pane (person lookup with portraits, scoped by race role) | done |
| 008 | Persistence and event-stream infrastructure for `hej` | done |
| 009 | Offline-first client data layer (shared budget, readiness and freshness) | done |
| 010 | Vehicle registration (cars and trailers, self-registered) | done |
| 011 | The public frontpage: albums, patruljens egen side, and public glimt | done |
| 012 | Switch profile from the app bar | done |
| 013 | The anonymous website: what everyone who cannot use the app gets instead | draft |
| 014 | Development device simulation | done |
| 015 | Guardian check outcomes on the stream | done |
| 016 | Map handouts, visible checkpoints, and the scan drawer | done |
| 017 | One sync check on foreground, for every dataset the device holds | done |
| 018 | Compass orientation: rotate the map to the device heading | draft |
| 019 | Glimt: sharing a moment inside the event | done |
| 020 | Video in Glimt | draft |
| 021 | The public site at the root, and two binaries | draft |
| 022 | Photographer admin: a photo library, albums, locations and patrol tags | doing |
| 023 | A shared photo viewer, and album pages that stay bounded and sharp | done |
| 024 | Album sort order | done |
| 025 | Photo credit by crew member | done |
| 026 | Social share previews for the public site | done |
| 027 | Keep the photographer's original | done |
| 028 | Trailer registration and one plate spelling across apps | draft |

Keep this table current when a PRD is added or changes folder.
