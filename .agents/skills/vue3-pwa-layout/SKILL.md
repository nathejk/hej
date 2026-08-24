---
name: vue3-pwa-layout
description: >
  Conventions and directory layout for the Vue 3 + TypeScript PWA frontend in
  the `hej` repo: Tailwind v4, shadcn-vue components, Pinia stores, vue-router,
  service worker / installable PWA, Lucide icons. Apply this skill when adding
  views, routes, components, stores, styling or API calls in the `vue/`
  workspace. Trigger phrases: "add a page", "new view", "new route", "new
  component", "shadcn", "component library", "Pinia store", "Tailwind",
  "frontend", "PWA", "service worker", "push", "Vite config".
---

# Vue 3 PWA Layout

The frontend is a **Vue 3 + TypeScript** installable **PWA** built with Vite,
styled with **Tailwind v4**, using **shadcn-vue** for components and **Pinia**
for state. It is always developed and run inside the `ui` container — never on
the host.

> Replaces the legacy `vue3-spa-layout` skill (kept as
> `vue3-spa-layout-legacy` for the sibling repos that still run it).
> **PrimeVue is forbidden** in this repo (`.rules`, PRD 004): no `primevue`, no
> `@primevue/*`, no Lara/theme presets, no PrimeIcons.

---

## The one rule that matters most

**Use a standard shadcn-vue component whenever one exists. Hand-roll only when
absolutely necessary.**

Before writing any UI markup, check the [shadcn-vue
catalogue](https://www.shadcn-vue.com/docs/components) for something that fits —
button, input, dialog, sheet/drawer, card, tabs, accordion, alert, badge,
skeleton, switch, checkbox, select, popover, tooltip, sonner (toast), and so on.
Then:

1. **Generate it** (once per component, in the container):

   ```sh
   docker compose run --rm ui npx shadcn-vue@latest add sheet
   ```

   It lands in `src/components/ui/<name>/` as source you own.

2. **Compose it** in your app component or view.

3. **Adapt it in place** if it does not quite fit — edit
   `src/components/ui/...` directly. Do not wrap a primitive in another component
   purely to patch its styling, and do not fork a parallel implementation.

Hand-rolling is a last resort, allowed when the catalogue genuinely has no
equivalent (e.g. the bottom navigation bar, a map overlay control). When you do
it, **say why in a comment at the top of the component**:

```vue
<!-- Hand-rolled: shadcn-vue has no bottom-tab-bar primitive. Composes
     RouterLink + Lucide icons directly; see PRD 001. -->
```

Rationale: shadcn primitives are Reka-UI based, so they bring correct keyboard
handling, focus management, ARIA wiring and scroll-locking that hand-rolled
markup silently lacks. Reinventing them is how a PWA ends up inaccessible.

---

## Top-level layout

```
vue/
├── index.html              # Vite entry — mounts #app
├── vite.config.ts          # dev server on :80, /api proxy → api, PWA plugin
├── components.json         # shadcn-vue CLI config (aliases, style, icons)
├── tsconfig.json           # `@/*` → `vue/src/*`
├── env.d.ts                # ambient types (incl. __APP_VERSION__)
├── package.json            # scripts: dev / build / preview / type-check
├── public/                 # static assets copied verbatim (incl. push-sw.js)
└── src/
    ├── main.ts             # createApp, Pinia, router, PWA init, mount('#app')
    ├── App.vue             # app shell (top bar + <RouterView/> + BottomNav)
    ├── router/index.ts     # routes + auth/role guards — see "Routing"
    ├── views/              # one file per top-level route — *View.vue
    ├── components/         # app components, composed from ui/ primitives
    │   ├── ui/             # shadcn-vue generated primitives (owned source)
    │   └── icons/          # custom SVG marks only — regular icons are Lucide
    ├── stores/             # Pinia stores — *.store.ts
    ├── config/             # declarative app config (brand, navigation, contact)
    ├── helpers/            # fetchWrapper, pwa, cn(), misc utils
    └── assets/             # main.css (Tailwind + theme tokens), images, fonts
```

The path alias `@` resolves to `vue/src/` (set in `vite.config.ts` and
`tsconfig.json`). Use `@/...` imports — never long relative chains.

---

## Routing

Routes live in `src/router/index.ts`. Conventions:

- Views are lazily imported: `() => import('@/views/XxxView.vue')`.
- Route names are lowercase/kebab strings. This app navigates by name
  (`router.replace({ name: 'login' })`), so name every route.
- **Navigation is data-driven.** Destinations are declared once in
  `src/config/navigation.ts` (path, label, Lucide icon, allowed `roles`) and
  drive both the router and `BottomNav.vue`. Add a page there, not in two places.
- **Guards** enforce auth and role. An unauthenticated user only ever sees
  `LoginView`; a role that may not see a route is redirected to one it may.
- Use `meta` for shell behaviour (e.g. `fullBleed` for edge-to-edge pages such as
  the map) rather than special-casing route names inside `App.vue`.

### Adding a page

1. Create `src/views/<Name>View.vue`.
2. Register it in `src/router/index.ts` with a lazy import and a `name`.
3. Add it to `src/config/navigation.ts` with a Lucide icon and, if it is
   role-limited, a `roles` array. Destinations beyond the 5th automatically move
   into the `MoreMenu` overflow sheet.
4. If it needs server data, fetch through `@/helpers` `fetchWrapper` from a Pinia
   store action (preferred) or `onMounted`.

---

## State (Pinia)

- One store per concern, file named `<concern>.store.ts`.
- Use `defineStore('<id>', { state, getters, actions })` — match the shape of
  `session.store.ts`.
- `session.store.ts` is the single source of truth for "who is signed in";
  guards and the shell read it. Don't duplicate identity elsewhere.
- Stores own API calls that mutate shared state. Component-local, single-use
  fetches may live in the component.
- Persist only what must survive a reload, in `localStorage`, inside the relevant
  action. Namespace keys `hej.<area>.<thing>`.
- Browser-capability stores (`location.store.ts`, `notifications.store.ts`,
  `app.store.ts`) wrap permission-gated platform APIs. They must **degrade
  gracefully and never throw** — expose an `available` getter and a permission
  state, and resolve to `null`/`false` on denial so callers can carry on.

---

## API calls

- Always use `@/helpers` `fetchWrapper` — never bare `fetch` or `axios`. It
  centralises session handling and turns a 401 into "session lost → login".
- Hit relative paths (`/api/...`). In dev, `vite.config.ts` proxies `/api` to the
  `api` container; in prod the Go binary serves both the API and the SPA from one
  origin, so no proxy is needed.
- Never hardcode hostnames. The backend base URL is implicitly the same origin.
- The BFF speaks `snake_case` JSON; map it to `camelCase` at the store boundary
  (see `session.store.ts`'s `IdentityResponse` → `Identity`).

---

## Components & styling

- **shadcn-vue** is the component library. See "The one rule that matters most".
  Primitives live in `src/components/ui/` and are **owned source**: commit them,
  edit them in place, and don't reformat them gratuitously so upstream diffs stay
  readable. Note local deviations in a comment.
- **Tailwind v4** is the styling tool, configured **CSS-first** in
  `src/assets/main.css` (`@import "tailwindcss"`, `@theme`,
  `@custom-variant dark`) — there is no `tailwind.config.js`. Design tokens are
  CSS variables; change colour centrally there, not per component.
- Prefer utility classes. Reach for `<style scoped>` only for what Tailwind
  genuinely cannot express.
- **`cn()`** from `@/helpers` merges conditional class lists (clsx +
  tailwind-merge). Use it instead of template-string concatenation when classes
  are conditional.
- **Icons: Lucide only** (`lucide-vue-next`), imported as components:
  `import { MapPin } from 'lucide-vue-next'`. Custom brand marks go in
  `src/components/icons/` as SVG components. No other icon set — no PrimeIcons,
  no Font Awesome.
- **Component naming:** PascalCase filenames, one component per file. Views end in
  `View.vue`; reusable pieces don't.
- Components are **explicitly imported**. There is no global auto-registration.

### Mobile-first, always

This is a phone app installed to the home screen, used one-handed, outdoors, at
night:

- Touch targets ≥44px. Audit generated shadcn components for this — several are
  desktop-sized by default; adjust them in `ui/`.
- Respect safe areas: `env(safe-area-inset-top/bottom)` on anything pinned to a
  screen edge (see `App.vue`'s header and `BottomNav.vue`).
- The shell owns full-height layout (`h-full`, `min-h-0 flex-1 overflow-y-auto`);
  views should not fight it with their own viewport units.
- Prefer sheets/drawers over centred desktop dialogs for anything substantial.
- All user-facing copy is **Danish**.

---

## PWA

- `vite-plugin-pwa` provides the manifest, service worker and update flow.
  Manifest values mirror `@/config/brand` — keep them in sync.
- `registerType: 'prompt'`: a waiting new build raises an event that
  `@/helpers/pwa` (`initPwa`) turns into `app.store.setUpdateAvailable(true)`,
  which `UpdatePrompt.vue` surfaces as a non-blocking "reload" notice. Don't
  auto-reload behind the user's back.
- Custom push / `notificationclick` handling lives in `public/push-sw.js` and is
  pulled in via the Workbox `importScripts` option.
- The app runs in a **secure context** in dev (Traefik TLS on
  `hej.local.nathejk.dk`), which is required for service workers, geolocation,
  camera and Web Push. That is why we don't use `localhost`.
- **Never precache third-party tiles/media** (e.g. map tiles) — it blows the
  storage quota.
- A build id is exposed as `__APP_VERSION__` via a Vite `define`.

### Browser baseline

**iOS/iPadOS Safari 16.4+ and Chrome 111+.** iOS only supports Web Push for
home-screen web apps from 16.4, so the app's own feature set already sets that
floor — and Tailwind v4 requires the same. Target it without apology: no
polyfills or graceful fallbacks for older engines, because those browsers cannot
run the app's primary features (push, service worker, geolocation) anyway. Modern
CSS (`@property`, `color-mix()`, cascade layers, `:has()`) is fair game.

---

## Testing & checks

| Tool     | Scope       | Command              |
|----------|-------------|----------------------|
| vue-tsc  | type-check  | `npm run type-check` |
| Vite     | prod build  | `npm run build`      |

There is no unit or e2e suite in this repo yet, so **verification is manual**:
after any change to the shell, styling or a shared component, build and click
through every route on a phone viewport. If you add a test runner, add it to this
table.

```sh
docker compose run --rm ui npm run type-check
docker compose run --rm ui npm run build
```

---

## Running things

The `ui` container (`docker/init/ui-dev`) runs `npm ci && npm run dev`, starting
Vite on port 80 inside the container, routed by Traefik to
`https://hej.local.nathejk.dk` (HTTP redirects to HTTPS). `node_modules` lives in
a named volume so a host re-clone doesn't trigger a full reinstall.

> **One-off commands need `--entrypoint`.** The `ui-dev` image sets
> `ENTRYPOINT ["ui-dev"]`, so `docker compose run --rm ui npm install x` silently
> ignores your command and runs `npm ci && npm run dev` instead. Always override
> the entrypoint, and add `--no-deps` so you don't drag up `api`/`db`.

```sh
# add a dependency
docker compose run --rm --no-deps --entrypoint sh ui -c "npm install <pkg>"

# add a shadcn component
docker compose run --rm --no-deps --entrypoint sh ui -c "npx shadcn-vue@latest add dialog"

# rebuild on lockfile change
docker compose build ui

# tail
docker compose logs -f ui
```

The image is `node:20-alpine` with **no Python or C toolchain**, so any tool that
pulls a native module (e.g. `@tailwindcss/upgrade`, which needs
`tree-sitter-javascript`) dies with an opaque `gyp ERR! find Python`. Install
build deps in the throwaway container first, and use `--no-save` so nothing leaks
into `package.json`:

```sh
docker compose run --rm --no-deps --entrypoint sh ui -c "apk add --no-cache python3 make g++ && npm i --no-save <tool>"
```

---

## Don'ts

- Don't use PrimeVue, PrimeIcons, or any second component library.
- Don't hand-roll UI that shadcn-vue already provides — and if you must, comment
  why.
- Don't wrap a `ui/` primitive just to restyle it; edit the primitive.
- Don't run `npm`, `npx`, `node`, or `vite` on the host.
- Don't add a `tailwind.config.js` back — Tailwind v4 config is CSS-first.
- Don't import from deep relative paths (`../../../../something`) — use `@/...`.
- Don't sprinkle `fetch()` / `axios` calls — go through `@/helpers`.
- Don't introduce an icon set other than Lucide.
- Don't register a page in the router without adding it to
  `src/config/navigation.ts` (or deliberately noting why it has no nav entry).
- Don't precache external tiles/media in the service worker.
