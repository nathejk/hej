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
| 001 | "Hej Nathejk" event app skeleton (PWA shell + phone login) | done |
| 002 | Event map (position, Danish topo + aerial layers, patrol scan history) | doing |
| 003 | Profile page (own details, self-portrait, device permission status) | done |
| 004 | Migrate the component library from PrimeVue to shadcn-vue (and upgrade Tailwind to v4) | done |
| 005 | Install-first mobile onboarding (install, confirm, permissions) | draft |
| 006 | Member directory for the app (person lookup by phone, app roles) | done |
| 007 | Portrait identification | draft |
| 008 | Persistence and event-stream infrastructure | done |
| 009 | Offline-first client data layer | draft |
| 010 | Vehicle registration | draft |
| 011 | Post-race experience | draft |

Keep this table current when a PRD is added or changes folder.
