# 038 — Upgrade the ui image to Node 22 LTS

**Status:** done
**Priority:** high
**Created:** 2026-08-24
**Picked up by:** agent (opus-5)
**Started:** 2026-08-24
**Completed:** 2026-08-24

## Description

Discovered as a hard blocker while doing task 027: the **shadcn-vue CLI cannot run
on Node 20**. It crashes inside `undici` before doing any work:

```
TypeError: webidl.util.markAsUncloneable is not a function
    at new CacheStorage (.../undici/lib/web/cache/cachestorage.js:20:17)
```

`undici` (pulled in by the CLI) requires Node 22+. Verified: the exact same
command runs fine under `node:22-alpine`.

`docker/Dockerfile` pins **`node:20-alpine`** for both the `ui-dev` (dev) and
`ui-builder` (prod) stages. Node 20 has also reached end-of-life, so this is
overdue independently of shadcn.

Work:

- Bump `ui-dev` and `ui-builder` to `node:22-alpine`.
- Bump `@types/node` to `^22` so the types match the runtime.
- Rebuild the `ui` image and confirm dev server, `build` and `type-check` still
  work (Vite 5 supports Node 22).
- Keep both stages on the same major — a dev/prod Node split would be a nasty
  source of "works on my machine".

Not strictly part of PRD 004's intent, but PRD 004 cannot proceed without it, so
it is tracked here rather than smuggled into task 027.

PRD: 004 (blocker for 027, 029). Related: `docker-dev-stack` skill.

## Acceptance Criteria

- [x] `docker/Dockerfile` uses `node:22-alpine` for `ui-dev` and `ui-builder`.
- [x] `@types/node` is `^22`.
- [x] `docker compose build ui` succeeds.
- [x] `npm ci`, `npm run type-check` and `npm run build` pass in the rebuilt
      container.
- [x] The Vite dev server starts and serves the app.
- [x] `npx shadcn-vue@latest` runs without the `undici` crash.

## Progress Log

- 2026-08-24 01:20 — Task created mid-027: the shadcn-vue CLI died on Node 20 with
  an `undici`/`webidl.util.markAsUncloneable` TypeError. Confirmed the same command
  works under `node:22-alpine`, so this is a runtime-version problem, not a CLI
  bug.
- 2026-08-24 01:22 — Picked up. Bumping both `ui` stages rather than only the dev
  one: a dev/prod Node major split is not worth the debugging it eventually costs.
- 2026-08-24 01:28 — `ui-dev` and `ui-builder` → `node:22-alpine`, the ASCII stage
  diagram in the Dockerfile header updated to match, and `@types/node` → `^22.20.1`
  so the types track the runtime. Left a comment at the `ui-dev` stage explaining
  *why* 22 (shadcn CLI + Node 20 EOL), since a bare version bump invites someone to
  "tidy" it back later.
- 2026-08-24 01:30 — ✅ `docker compose build ui` OK; container reports v22.23.2;
  `npm ci`, `type-check` and `build` all clean on 22; the shadcn-vue CLI runs
  without the `undici` crash. Vite 5 had no complaints.
- 2026-08-24 01:31 — Also updated the `vue3-pwa-layout` skill, which documented the
  image as `node:20-alpine`. Completed.
