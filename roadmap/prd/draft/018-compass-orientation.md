# PRD 018 — Compass orientation: rotate the map to the device heading

**Status:** draft
**Author:** agent session (Zed / Claude)
**Created:** 2026-09-15
**Last updated:** 2026-09-15 (rotation spike run — §11.1 resolved: `leaflet-rotate`, subject to a device check)
**Approved:**
**Shipped:**
**Target users:** participant (patrol member on the map)

<!--
Status must match the folder this file is in: draft/, doing/ or done/.
Leave Approved blank until the PRD moves to doing/, and Shipped blank until it
moves to done/. See roadmap/prd/README.md for the lifecycle.
-->

---

## 1. Summary

A third control on the map, beside the layer switcher and the locate button:
an **orientation toggle**. Tapped, it rotates the map from north-up to
**heading-up** — the map turns under a fixed "up", so the direction the patrol is
facing is always towards the top of the screen. In that mode the blue position
dot gains a **heading indicator** showing which way the device is pointing.

## 2. Problem & Motivation

- **What problem does this solve?** A north-up map forces the patrol to do the
  rotation in their head: they are walking south-east down a firebreak, the map
  says the post is "up and left", and they have to mentally turn the map 135° to
  act on that. In the dark, tired, this is exactly where people set off the wrong
  way. A heading-up map removes the translation: "the post is ahead-left on
  screen" means "ahead-left on the ground".
- **Why now?** The map already has position (PRD 002) and now checkpoints and
  edge arrows (PRD 016). The arrows tell a patrol *which way* to a post; a
  heading-up map makes "which way" line up with their body instead of with north.
  The two compound.
- **Evidence.** This is the single most common feature of every hiking/orienteering
  app, and Nathejk is an orienteering event walked at night. The paper map is
  turned to the terrain constantly; the app should not be the one thing that
  cannot be.

## 3. Goals

- A patrol can switch the map between north-up and heading-up with one tap.
- In heading-up mode the map rotates smoothly to follow the device compass.
- The position dot shows which way the device is facing.
- The choice is remembered, like the base-layer choice.
- None of it degrades the map for a device with no compass, or a user who never
  turns it on.

## 4. Non-Goals

- **Not** rotating the map by the direction of *travel* (GPS course). That is a
  different signal — useful in a car, wrong on foot where you face one way and
  sidestep another — and it needs no new sensor. Compass heading is the ask.
- **Not** a full compass instrument (bezel, degrees readout). This is map
  orientation, not a navigation panel.
- **Not** rotating text/labels to stay upright, beyond what falls out of the
  chosen rotation approach. Danish place names on a rotated map are legible
  enough; fighting the tile renderer to keep them upright is out of scope.
- **Not** changing the north-up default. Heading-up is opt-in per the request,
  and north-up stays the state a cold map opens in until the user has chosen.
- **Not** true-vs-magnetic-north correction beyond what the platform already
  gives us (§11).

## 5. User Stories & Scenarios

- As a **patrol member**, I want the map to turn as I turn, so "ahead" on screen
  is "ahead" on the ground.
- As a **patrol member**, I want to see which way I am pointing, so I can line the
  map up with what I see around me.
- As a **patrol member on an old phone with no compass**, I want the control to be
  absent or plainly disabled rather than there-but-broken.

**Happy path.** A patrol taps the orientation button. On iOS the first tap
raises the system's motion-sensor permission prompt; they allow it. The map
rotates so their heading is up, and the blue dot grows a translucent cone
pointing up (they are, by definition, facing up now). They walk; the map turns
under them. They tap the button again; the map animates back to north-up and the
cone disappears. Next time they open the map, it is heading-up again.

**Edge cases.**

- **No sensor / desktop.** The button is not shown (or shown disabled with a
  clear reason). Nothing else changes.
- **Permission denied** (iOS). The button reflects the blocked state, like the
  locate button does for geolocation, and the map stays north-up.
- **Compass noise / no calibration.** Raw compass heading jitters and can be tens
  of degrees off before the figure-8 calibration. The map must not judder — the
  heading is smoothed, and a rotating map that lags reality slightly is far better
  than one that shakes.
- **Heading-up + following position** interact: both want to control the
  viewport. They compose (recentre on the patrol *and* rotate to heading), and
  the locate button and orientation button stay independent toggles.
- **Backgrounding.** The compass listener stops when the map is hidden, like the
  geolocation watch (PRD 002) — it is a battery drain otherwise.
- **The edge arrows.** They currently assume screen-up is north (task 278). On a
  rotated map that is false, and the arrows' *compass word* in the accessible
  label would be wrong unless corrected (§8). Their placement and chevron are
  screen-space and keep working; only the derived compass label needs the map
  bearing folded in.

## 6. Requirements

### Functional

- [ ] A third button in the top-right control stack toggles orientation.
- [ ] It is shown only when the device can supply a compass heading.
- [ ] On iOS the first activation requests motion-sensor permission from within
      the tap (the platform requires a user gesture); denial is reflected and
      non-fatal.
- [ ] Heading-up mode rotates the map so the device heading is towards the top.
- [ ] The rotation is smoothed so a noisy compass does not judder the map.
- [ ] The position dot shows a heading indicator (a cone/beam) in heading-up mode.
- [ ] North-up is restored on a second tap, with the indicator removed.
- [ ] The mode is persisted (localStorage, `hej.map.*`), like the base layer.
- [ ] The compass listener runs only while the map is visible and the mode is on.
- [ ] The edge arrows' accessible compass word stays correct on a rotated map.

### Non-Functional

- **Battery.** No sensor listener except while heading-up is active and the page
  is visible. This is a night-long event on phones that also run the position
  watch and hold a tile cache.
- **Performance.** Rotation must stay smooth on the baseline device
  (iOS Safari 16.4 / Chrome 111). A rotation approach that reflows or re-rasterises
  tiles every frame is not acceptable.
- **Accessibility.** The button follows the locate button's pattern — labelled,
  `aria-pressed`, blocked state legible. A rotated map is a visual affordance; the
  arrows' text labels remain the non-visual route and must stay correct (§5).
- **Degradation.** Everything works with the feature off or absent. It must never
  be the reason the map fails to load.
- **i18n.** Danish labels.
- **Browser baseline.** iOS/iPadOS Safari 16.4+, Chrome 111+.

## 7. UX / UI Notes

- The button sits third in the existing top-right stack (`MapsView`), same 44 px
  rounded style as the layer and locate buttons, a Lucide icon (`Compass`, or
  `Navigation` for the active state). Active/`aria-pressed` uses the same
  blue-when-on treatment as the locate button.
- The heading indicator is a translucent wedge on the position `CircleMarker`,
  widening outward, centred on the device heading. In heading-up mode it points
  up by construction; the value is that it exists at all and confirms the map is
  tracking. (Whether to also show it in north-up mode when a heading is known is
  §11.)
- Rotation animates when toggled (a short ease) and tracks continuously while on.

## 8. Technical Considerations

**Frontend only. No BFF work** — this is entirely a client capability (sensor +
rendering), so there is no endpoint and nothing to annotate.

- **Rotating a Leaflet map is the central problem.** Leaflet has **no native
  rotation**, and this repo has no rotation plugin. The three routes, with the
  trade this PRD needs a decision on (§11.1):
  1. **`leaflet-rotate` community plugin.** Adds `map.setBearing()` and makes
     `latLngToContainerPoint` rotation-aware — which the edge arrows depend on, so
     they keep working almost for free. Cost: an unmaintained-ish dependency,
     bundle weight, and it patches Leaflet internals (compat risk against our
     pinned Leaflet 1.9).
  2. **CSS `transform: rotate()` on the map pane**, counter-rotating markers.
     Cheap to spin, but it desynchronises Leaflet's own geometry (drag, the tile
     grid's edges show as you rotate a rectangle, `containerPoint` maths goes
     wrong) — and it would break the arrows' projection. Rejected unless (1) and
     (3) fail.
  3. **A CSS-rotated wrapper with a larger-than-viewport map** to hide the
     corners, still Leaflet-native inside. Middle ground; more layout work.
  The recommendation is (1), gated on checking it against Leaflet 1.9 on the
  baseline engines.
- **The compass source.** iOS exposes `webkitCompassHeading` on the
  `deviceorientation` event (true-north-referenced, 0 = north, already
  compensated) and requires `DeviceOrientationEvent.requestPermission()` from a
  user gesture. Chrome/Android uses `deviceorientationabsolute` (or `alpha` on
  `deviceorientation`) and needs the alpha→heading conversion and a magnetic
  declination it does not supply. This belongs in a **`orientation.store.ts`**
  browser-capability store, next to `location.store`, degrading gracefully and
  never throwing (the store convention).
- **Smoothing.** Compass output is noisy; feed it through a low-pass /
  shortest-arc filter before it drives rotation, and rotate the map on an
  animation frame, not on every sensor event.
- **The edge arrows (task 278).** `screenBearingDegrees` gives the chevron's
  screen rotation and is correct on a rotated map *if* placement uses a
  rotation-aware projection (route 1 gives that). But the accessible label derives
  a compass word from the screen bearing assuming screen-up = north; on a rotated
  map that must instead be `screenBearing + mapBearing`. The map's bearing has to
  reach the arrow computation.
- **Position marker.** Today a plain `CircleMarker` (EventMap). The heading wedge
  needs a `divIcon` or an SVG marker whose rotation is bound to the heading.
- **New config/state.** A `hej.map.orientation` persisted key; runtime-tunable
  smoothing constant if we want to adjust it without a release (optional).

## 9. Success Metrics

- A patrol can align the map to the terrain without a second thought — measured
  qualitatively (fewer "we went the wrong way out of the post" reports) rather
  than by a counter.
- No measurable battery or frame-rate regression on the baseline device with the
  mode **off** (the default).
- Rotation stays smooth (no visible judder) with the mode on, on the baseline
  device.
- Zero reports of the map failing to load attributable to the sensor or the
  rotation code.

## 10. Rollout / Task Breakdown

Sequenced so the risky decision is settled first and nothing else starts until
the map can actually rotate.

- [ ] Task: spike — rotate a Leaflet 1.9 map on iOS 16.4 and Chrome 111, decide
      route 1 vs 3 (§11.1). **Code-level spike done 2026-09-15 — `leaflet-rotate`
      chosen; the remaining piece is the on-device rendering/smoothness proof.**
      Nothing else proceeds until that lands.
- [ ] Task: `orientation.store.ts` — compass heading + iOS permission, smoothed,
      injectable, never throws.
- [ ] Task: map rotation wired to the smoothed heading, with the toggle plumbed
      through `EventMap`.
- [ ] Task: the third control button, following `LocateButton`'s pattern (shown
      only when a heading is available; blocked state on denial).
- [ ] Task: heading indicator on the position marker.
- [ ] Task: persist the mode (`hej.map.orientation`).
- [ ] Task: fold the map bearing into the edge arrows' compass label so it stays
      correct when rotated.
- [ ] Task: stop the listener when hidden / mode off; verify battery behaviour.

## 11. Open Questions

1. **How do we rotate the map — `leaflet-rotate` or a CSS wrapper?** **Resolved by the
   spike (2026-09-15): `leaflet-rotate`, pending one on-device confirmation.**

   What the spike established, at code level, against our pinned Leaflet:

   - **Version fit.** `leaflet-rotate@0.2.8` declares `peerDependencies:
     { leaflet: ^1.9.3 }`; we run `^1.9.4`. Clean match. (Last published 2023-07;
     it targets exactly the Leaflet 1.9 line we are pinned to, so "unmaintained"
     matters less than it would for a fast-moving base.)
   - **API.** Adds `map.setBearing(deg)` / `map.getBearing()` and a `rotate: true`
     map option — exactly the toggle this feature needs.
   - **The arrows keep working, which was the real worry.** The plugin overrides
     `layerPointToContainerPoint` (and its inverse) to apply the bearing, and
     Leaflet computes `latLngToContainerPoint` through that — so `EdgeArrows`'
     `project()` becomes rotation-aware for free. Confirmed in the plugin source,
     not assumed. Only the arrows' *compass word* still needs `+ mapBearing`
     (§8), because screen-up is no longer north.
   - **Types.** Ships none, and `@types/leaflet` has no rotation API, so it needs a
     small module-augmentation shim (`setBearing`/`getBearing`, the `rotate`
     option). Written during the spike; type-check passed clean.
   - **Build.** Vite bundles its ESM against Leaflet 1.9 without complaint. Size:
     ~21 KB min / ~6 KB gzip, and in the real integration it sits inside the
     already-lazy `leaflet` chunk (150 KB → ~171 KB), so the app shell is
     unaffected — provided it is imported from within `EventMap` (the lazy chunk),
     **not** the app entry. The spike proved this the hard way: importing it from
     `main.ts` pulled all of Leaflet into the entry bundle, which is the mistake
     the real code must avoid.

   **What the spike could not answer — the remaining device check.** It ran in the
   toolchain, not on glass, so it says nothing about *rendering*: whether rotated
   square tiles show gaps/corners at the viewport edges, whether rotation is smooth
   at 60 fps on an iPhone (Safari 16.4) and a mid-range Android (Chrome 111), and
   whether the plugin's touch-rotate gesture fights our pan/zoom handlers. Those
   are the things that would send us to route 3 (the oversized CSS-rotated
   wrapper), and they can only be judged on a device. So the first implementation
   task stays a **thin on-device proof** before the rest proceeds — the code-level
   risk is now retired, the rendering risk is not.

   The spike was run and then fully reverted (plugin uninstalled, shim and probe
   removed); the tree is unchanged. Re-adding it belongs to the first task once
   this PRD is approved.

2. **True north or magnetic north?** iOS `webkitCompassHeading` is true-north
   compensated. Android's absolute orientation is magnetic and needs a declination
   (~3–4° E in Denmark) we would have to hardcode or ignore. Is ignoring a few
   degrees acceptable? For lining a map up to terrain, almost certainly yes — but
   it should be a decision, not an accident.
3. **Show the heading indicator only in heading-up mode, or whenever a heading is
   known?** The request says heading-up. But a north-up map with a "you are facing
   this way" cone is useful too and is most of the same code. Cheap to offer;
   worth deciding.
4. **What triggers the iOS permission prompt?** The orientation button's first tap
   is the natural user gesture. Confirm we are happy to spend the prompt there
   rather than during onboarding — the map is where the feature lives, so probably
   yes.
5. **Does heading-up force follow-position on?** A heading-up map centred nowhere
   near the patrol is disorienting. Options: heading-up implies following, or they
   stay independent and the user can pan away. Leaning towards independent (matches
   the locate button) but worth confirming.
6. **Smoothing constant and update rate.** A number somebody has felt on a device,
   not guessed. Belongs in the spike's output.
