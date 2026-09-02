# 201 — Log the fix strategy where it happens, and identify the build in the report

**Status:** done
**Priority:** medium
**Created:** 2026-09-02
**Picked up by:** agent session (Zed)
**Started:** 2026-09-02
**Completed:** 2026-09-02

## Description

The first shared `/sporing` report (2026-09-02, iPad 6th gen) answered several questions and revealed that
two things it should have carried were missing.

**1. The fix strategy was not logged at all.** Task 199 races three geolocation strategies and logs which
one answered — but the log line was written at the *call site*, in the map's permission card. That device
granted location during **onboarding** instead (PRD 005 step 5), so the card never appeared and the line
never ran. The report showed `placeringstilladelse: granted` and 12 recorded points with no indication of
how the fix was obtained, which is exactly the question the race was built to answer.

The lesson is general: instrumentation belongs where the *event* happens, not where one caller happens to
sit. Location can be requested from onboarding, from the map's card, or from the locate button, and the
answer is worth the same from all three.

**2. The report could not identify the build.** It prints `app: 0.0.0`, because `npm_package_version` is
unset in this project — so a shared report cannot say which bundle produced it. That matters more here than
in most apps: an installed PWA keeps its old bundle until the update prompt is accepted
(`registerType: 'prompt'`), so a device can be a week behind while looking current, and the run that
prompted this task had *both* a reinstall and a version change in it with no way to tell them apart.

## Acceptance Criteria

- [x] The strategy and accuracy of every successful fix are logged from `location.store`, so any surface
      that asks records the same thing.
- [x] Every failure cause is logged from the store too, for the same reason.
- [x] `/sporing` prints the **build id**, not only the version.
- [x] `/sporing` shows the last fix's strategy and whether it was coarse, so the answer is on the page and
      not only in the event list.
- [x] `logEvent` cannot break anything it instruments — verified: it swallows every error, including a
      missing IndexedDB.

## Progress Log

- 2026-09-02 — Task created from the first shared device report.
- 2026-09-02 — Moved the `nofix` / `geoerror` logging from `MapsView.accept()` into the store's own success
  and failure paths. The map view keeps no logging of its own, so there is one place to change and no way for
  a second caller to be silently uninstrumented.
- 2026-09-02 — Added `build` to the report header and a `sidste fix via` / `kun grov placering` pair to the
  recording section. The build line exists specifically so the next shared report cannot be ambiguous about
  which bundle it came from — the previous one was, and it cost us the ability to separate "the reinstall
  fixed it" from "the new version fixed it".
- 2026-09-02 — ✅ Green: suite 386 across 31 files, `type-check` and `build` clean. No new tests: this moves
  existing instrumentation and adds two report lines, both covered by the store's existing specs.
