# PRD 018 — Compass orientation: rotate the map to the device heading

**Status:** draft
**Author:** agent session (Zed / Claude)
**Created:** 2026-09-15
**Last updated:** 2026-09-15 (§11.1 → `leaflet-rotate`; §11.2 → true north; §11.3 → cone whenever heading known; §11.4 → request after location grant; §11.5 → independent of follow)
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

**Happy path.** During the location-permission step, once the patrol grants
location, the app asks for motion-sensor access too (iOS; §11.4). With that
granted, the blue position dot grows a translucent cone showing which way they are
facing — on the still-north-up map it swings around the dot as they turn. They tap
the new orientation button; the map rotates so their heading is up (the cone now
points up by construction) and turns under them as they walk. A second tap returns
to north-up; the cone stays, now swinging again. Next time they open the map, it
is heading-up.

**Edge cases.**

- **No sensor / desktop.** The button is not shown (or shown disabled with a
  clear reason), and no cone appears. Nothing else changes.
- **Location declined.** No compass prompt at all (§11.4): with no position the
  orientation feature has little to align, and a second prompt for someone who
  just declined the first is exactly the pestering to avoid. The button is absent.
- **Motion permission denied** (iOS). The button reflects the blocked state, like
  the locate button does for geolocation, no cone appears, and the map stays
  north-up.
- **Compass noise / no calibration.** Raw compass heading jitters and can be tens
  of degrees off before the figure-8 calibration. Neither the map nor the cone may
  judder — the heading is smoothed, and lagging reality slightly is far better than
  shaking.
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
- [ ] It is shown only when the device can supply a compass heading (which, per
      §11.4, means location is granted and motion permission is available).
- [ ] Motion-sensor permission is requested **after location is granted, and only
      then** (§11.4); on iOS this fires from a user gesture, since the platform
      requires one. Denial is reflected and non-fatal.
- [ ] The position dot shows a heading indicator (a cone/beam) **whenever a
      heading is known** — in both north-up and heading-up mode (§11.3).
- [ ] Heading-up mode rotates the map so the device heading is towards the top.
- [ ] The rotation and the cone are smoothed so a noisy compass does not judder.
- [ ] North-up is restored on a second tap; the cone remains (a heading is still
      known), now free to swing rather than pinned up.
- [ ] The mode is persisted (localStorage, `hej.map.*`), like the base layer.
- [ ] The compass listener runs only while the map is visible and a heading is
      wanted (the cone is shown or the map is heading-up).
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
  widening outward, centred on the device heading. It is shown **whenever a
  heading is known** (§11.3), not only when the map is rotated: on a north-up map
  it swings around the dot as the patrol turns, and in heading-up mode it points
  up by construction. Either way it confirms the app has a fix on their facing.
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
  `deviceorientation`), which is **magnetic**, so it needs Denmark's declination
  (~+3° E) added to reach true north — see §11.2, which settles that we align to
  **true north** because the tiles are drawn to it. This belongs in an
  **`orientation.store.ts`** browser-capability store, next to `location.store`,
  degrading gracefully and never throwing (the store convention).
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
      **normalised to true north** (§11.2: iOS as given, Android + declination),
      injectable, never throws.
- [ ] Task: map rotation wired to the smoothed heading, with the toggle plumbed
      through `EventMap`.
- [ ] Task: the third control button, following `LocateButton`'s pattern (shown
      only when a heading is available; blocked state on denial).
- [ ] Task: heading indicator (a cone/beam) on the position marker, shown
      **whenever a heading is known** — north-up or heading-up (§11.3).
- [ ] Task: request motion permission **after location is granted, and only
      then** (§11.4), on iOS from the first user gesture the constraint allows.
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

2. **True north or magnetic north?** **Resolved (2026-09-15): true north.**

   The map is the deciding fact. The Dataforsyningen tiles are Web Mercator
   (EPSG:3857), whose vertical axis runs along the meridians, so map-up *is*
   geographic (true) north — there is no meaningful grid convergence. Aligning the
   map to the terrain means rotating it by the device heading measured in the
   **same** frame the map is drawn to, so the heading must be true-north. Feed it a
   magnetic heading and the whole map lands rotated wrong by the local declination
   (~+3° E in Denmark today) — which is precisely the skew a patrol would see
   between an on-screen street and the real one they are lining it up against.

   Declination is a **systematic, correctable** bias, unlike the ±5–15° of
   calibration/interference noise a phone compass also carries. That noise is real
   and un-fixable in software, but it is not a reason to leave a known constant
   uncorrected: the two are added errors, and we remove the one we can.

   So, true north everywhere:

   - **iOS** `webkitCompassHeading` is intended to be true-north-referenced (Apple
     compensates using location), so it is used as given — to be confirmed on the
     device pass rather than trusted from the docs.
   - **Android** `deviceorientationabsolute` / `alpha` is magnetic, so Denmark's
     declination (~+3° E) is added to reach true north. A single hardcoded constant
     is enough at this precision over one country; a location-derived value (WMM)
     is not worth the weight.

3. **Show the heading indicator only in heading-up mode, or whenever a heading is
   known?** **Resolved (2026-09-15): whenever a heading is known.** The cone is
   shown on the position dot in both modes — on a north-up map it swings around the
   dot as the patrol turns, in heading-up mode it points up. It is most of the same
   code either way, and "you are facing this way" is useful even when the map has
   not been rotated. The listener therefore runs whenever the cone is wanted, not
   only in heading-up mode (§6).

4. **What triggers the motion-sensor permission, and when?** **Resolved
   (2026-09-15): request it right after location is granted, and only then** — not
   as a standalone prompt and not for users who declined location, for whom the
   orientation feature has little to align and a second prompt would just pester.

   One iOS mechanism caveat the implementation must respect:
   `DeviceOrientationEvent.requestPermission()` needs a **user gesture**, and a
   geolocation success callback is async, so the activation from the tap that
   granted location may have lapsed by the time we know it was granted. So "after
   location is granted" is the *gate*, but the actual iOS prompt fires from the
   first user gesture available once that gate is open — in practice the first
   interaction with the map/orientation control after location is on. On Android
   no gesture is required, so listening can start as soon as the gate opens. Either
   way: no location grant → no compass prompt. The exact gesture to hang it on is
   an implementation detail for the permission task, not a further product
   decision.

5. **Does heading-up force follow-position on?** **Resolved (2026-09-15): no.**
   The two stay independent toggles. Heading-up rotates the map; the locate button
   controls whether the map re-centres on the patrol — and either can be on without
   the other. This matches how the locate button already behaves and keeps each
   button meaning exactly one thing, which is easier to reason about with a thumb
   at 02:00 than a button that silently switches another on.

   The cost is that a patrol can rotate the map while panned away from themselves,
   looking at a heading-up view of somewhere they are not standing. That is a
   legitimate thing to want (reading the route ahead, turned to match the
   direction of travel), not a bug to design out. If it proves confusing in use,
   the lighter fix is a hint, not coupling the toggles.
6. **Smoothing constant and update rate.** A number somebody has felt on a device,
   not guessed. Belongs in the spike's output.
