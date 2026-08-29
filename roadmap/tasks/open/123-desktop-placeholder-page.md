# 123 — DesktopView: placeholder for non-mobile visitors

**Status:** open
**Priority:** low
**Created:** 2026-08-30
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 005 §6/§7. Add `vue/src/views/DesktopView.vue`, routed at `/desktop` with
`meta: { public: true, desktop: true }` (the `desktop?: boolean` addition to `RouteMeta` in
`vue/src/router/index.ts` comes with the router-guard task). Every visitor classified as a
desktop computer lands here, and desktop visitors never reach `/welcome` or `/install`.

The content is deliberately small: the brand header, and **one short Danish paragraph** saying
Hej Nathejk is an in-event app for your phone — open the site on your phone to install it.
That is the whole page.

## Deliberately thin, and that is the design

PRD 005 §11 (2026-08-30) is explicit that the desktop side of the PRD is a placeholder only:
the scope is "on a phone or tablet, prompt for installation and deliver the app when
installed; on anything else, show an ordinary website." **Building that website is a separate
PRD, and a §4 non-goal here.**

So this view is written to be **thrown away**. It shares **no components with the mobile app**
(PRD 005 §7) — not the shell, not `BottomNav`, not the onboarding cards. It reads the brand
values from `config/brand.ts` and otherwise stands alone. The property being protected is that
replacing it with the real desktop site cannot regress the app: if the placeholder imported
app components, the eventual replacement would either have to keep them or risk breaking
mobile, and a temporary page would have quietly become a dependency.

For the same reason, do not invest in it. PRD 005 §10 says build it as a stub and do not delay
the mobile flow on it — hence the low priority. Resist adding a login form "for organizers", a
nav, a footer with links, or event information; every one of those is a decision the real
desktop PRD should make.

## No data exposure, therefore no decision to make

The page renders **no participant-facing app content and no login form** (PRD 005 §6). That is
what keeps it free of a data-exposure question: there is nothing on it that is not already
public, so there is no "what may a desktop visitor see" call to get wrong. Note that PRD 005
§11 also settles that **no role may log in on desktop** — including organizers. If desktop
access is ever wanted it is a new PRD that revisits the gate, not a flag added here.

Shell chrome (top bar and `BottomNav`) is hidden on `/desktop`, via the App.vue `showShell`
change — this view does not manage that itself, and must not use `fullBleed`, which only
suppresses the header inside `showShell`.

## UI

Headline in `font-nathejk` per `.rules`. Copy in Danish. A Lucide icon if one helps
(`Monitor` is the one PRD 005 §7 names). shadcn-vue if a primitive fits, but plain markup is
acceptable here — this is one paragraph, and a `Card` around it is not obviously an
improvement. No new dependencies (PRD 005 §8: the placeholder needs none).

## Acceptance Criteria

- [ ] `vue/src/views/DesktopView.vue` exists and `/desktop` is registered with
      `meta: { public: true, desktop: true }` and a lazy import
- [ ] Content is the brand header plus **one short Danish paragraph** explaining that Hej
      Nathejk is an in-event app for the phone, and to open the site on a phone
- [ ] **No login form**, no bottom nav, no app content, no participant data
- [ ] Imports **no components from the mobile app** — only brand config; a comment states that
      this isolation is the point, so the real desktop site can replace it safely
- [ ] Does not manage shell visibility itself and does not use `fullBleed`
- [ ] No new runtime dependencies
- [ ] `font-nathejk` headline, Lucide icon if any, all copy Danish
- [ ] Renders correctly at desktop widths without the mobile layout's safe-area assumptions
- [ ] `npm run type-check` clean

## Depends on

- **Task 116** — `isMobileDevice()` decides who arrives here; the router-guard task
  (PRD 005 §10) does the routing. This view can be built and reached by URL before either
  lands.

## Progress Log

- 2026-08-30 — Task created from PRD 005.
