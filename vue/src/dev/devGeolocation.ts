import { ref } from 'vue'

import { useLocationStore, type GeolocationLike } from '@/stores/location.store'

// A fake position source (PRD 014, task 212).
//
// # Why not Chrome's sensor override
//
// The sensor panel can pin one coordinate. It cannot *move*, and movement is what the map, the
// position marker, the tile cache and the track log actually respond to. It also cannot produce a
// failure on demand — and the failure states are the ones hardest to reach deliberately:
// `track.store` dedupes identical geolocation failures, and the map has a "location off" state,
// neither of which has any other trigger on a laptop.
//
// # Injected, never patched
//
// This satisfies the `GeolocationLike` slice `location.store` already declares, and is handed in
// at that store's existing accessor. `navigator.geolocation` is not monkey-patched: a global patch
// is invisible at the call site and would also affect code paths that are not under test.

/** The failure modes worth being able to produce on demand. */
export type DevGeoFailure =
  | 'none'
  /** The user said no. `location.store` folds code 1 into its "denied" cause. */
  | 'denied'
  /** Location Services off at the OS level, or no fix. Code 2. */
  | 'unavailable'
  /** Code 3, reported promptly. */
  | 'timeout'
  /**
   * Never answers at all. Deliberately distinct from `timeout`: `config/track.ts` notes that a
   * hanging geolocation call holds the radio, and a callback that never arrives is a different
   * failure from one that arrives saying "timed out". Only the latter is easy to produce by
   * accident, which is why the former needs a switch.
   */
  | 'hang'

const enabled = ref(false)
const failure = ref<DevGeoFailure>('none')
// 12m: a plausible good urban fix. Large enough that the map's accuracy circle is visible,
// small enough not to look broken.
const accuracy = ref(12)

// Sjælland, reusing FALLBACK_CENTER from @/config/map rather than a new constant, and
// deliberately **not** the event area: that config's comment records that the area is not fully
// known to participants and is not revealed by this app, so a dev fixture must not be the thing
// that commits it to the repository.
const START = { lat: 55.6, lng: 11.85 }
const coord = ref({ ...START })

/** Where a moving source (task 213's playback) says we are, when one is registered. */
type MovingSource = () => { lat: number; lng: number } | null
let movingSource: MovingSource | null = null

export function setMovingSource(source: MovingSource | null) {
  movingSource = source
}

export const devGeoEnabled = enabled
export const devGeoFailure = failure
export const devGeoAccuracy = accuracy
export const devGeoCoord = coord

export function setDevGeoEnabled(on: boolean) {
  enabled.value = on

  // Keep the permission state consistent with the fake, or the app contradicts itself: a laptop
  // that has *denied* location for this origin would show "location off" while simulated positions
  // were arriving — and `location.store` may not even call the source in that state. PRD 014 is
  // explicit that nothing may look like it works when it does not, and this is the same rule in the
  // other direction.
  //
  // `applyPermissionState` is the store's own folding of a Permissions API answer, so this claims
  // exactly what a real grant claims, by the same code path.
  const location = useLocationStore()
  if (on) {
    location.applyPermissionState('granted')
  } else {
    // Stopping the fake watches matters: `location.store` holds one watch id and would not know to
    // clear ours, so the interval would keep pushing simulated positions over the real ones — the
    // app would look like it has two devices.
    stopDevWatches()
    // Back to whatever the browser actually says.
    void location.syncPermission()
  }
}

export function setDevGeoFailure(mode: DevGeoFailure) {
  failure.value = mode
}

function activeCoord() {
  return movingSource?.() ?? coord.value
}

function position(): GeolocationPosition {
  const { lat, lng } = activeCoord()
  // A structural stand-in rather than a real GeolocationPosition, which cannot be constructed:
  // the interface is read-only and has no public constructor. Every field the app reads is here;
  // `toJSON` is present because the type demands it and nothing calls it.
  return {
    coords: {
      latitude: lat,
      longitude: lng,
      accuracy: accuracy.value,
      altitude: null,
      altitudeAccuracy: null,
      heading: null,
      speed: null,
      toJSON: () => ({}),
    },
    timestamp: Date.now(),
    toJSON: () => ({}),
  } as unknown as GeolocationPosition
}

function error(): GeolocationPositionError {
  const code = failure.value === 'denied' ? 1 : failure.value === 'unavailable' ? 2 : 3
  // Plain object on purpose: `location.store` reads the numeric code rather than the
  // `err.PERMISSION_DENIED` constants, precisely so that a stub like this one works — see its
  // comment. Constructing the real type is not possible.
  return {
    code,
    message: `dev: simulated ${failure.value}`,
    PERMISSION_DENIED: 1,
    POSITION_UNAVAILABLE: 2,
    TIMEOUT: 3,
  } as GeolocationPositionError
}

// How often a watch emits. 1s is faster than a real device settles but keeps playback smooth
// enough to watch; the map's own throttling is what the interesting behaviour hangs off.
const WATCH_INTERVAL_MS = 1000

let nextWatchId = 1
const watches = new Map<number, ReturnType<typeof setInterval>>()
// Which watch ids this module issued. Needed because the bridge below can be switched between the
// fake and the real device *while a watch is running*: routing `clearWatch` by the current toggle
// would hand a fake id to the real API (which ignores it, leaking our interval) or a real id to
// ours (leaking the device's watch, i.e. the radio stays on). Ownership is a property of the id,
// not of the toggle's current position.
const ownedWatches = new Set<number>()

/** The fake source, unconditionally. */
function fakeSource(): GeolocationLike {
  return {
    getCurrentPosition(success, fail) {
      if (failure.value === 'hang') return
      if (failure.value !== 'none') {
        fail?.(error())
        return
      }
      // Asynchronously, like the real API. A synchronous callback would let code that happens to
      // work only because of the delay pass here and fail on a device.
      setTimeout(() => success(position()), 0)
    },

    watchPosition(success, fail) {
      const id = nextWatchId++
      ownedWatches.add(id)
      if (failure.value === 'hang') return id
      if (failure.value !== 'none') {
        setTimeout(() => fail?.(error()), 0)
        return id
      }
      setTimeout(() => success(position()), 0)
      watches.set(
        id,
        setInterval(() => success(position()), WATCH_INTERVAL_MS),
      )
      return id
    },

    clearWatch(id) {
      const timer = watches.get(id)
      if (timer) clearInterval(timer)
      watches.delete(id)
      ownedWatches.delete(id)
    },
  }
}

function realSource(): GeolocationLike | null {
  if (typeof navigator === 'undefined' || !('geolocation' in navigator)) return null
  return navigator.geolocation
}

/**
 * A source that follows the toggle **per call**, or `null` when there is nothing to offer.
 *
 * A pass-through bridge rather than "the fake when enabled", and this is the part that makes the
 * panel's switch usable: `location.store` resolves its geolocation **once**, into state, when the
 * store is created — so a provider that returned `null` while switched off would be captured as
 * the real device and the toggle would then do nothing until a reload. Handing over a bridge lets
 * the decision be made at each call instead.
 */
export function devGeolocationBridge(): GeolocationLike | null {
  const real = realSource()
  if (!real) return enabled.value ? fakeSource() : null

  return {
    getCurrentPosition(...args) {
      const source = enabled.value ? fakeSource() : real
      source.getCurrentPosition(...args)
    },
    watchPosition(...args) {
      const source = enabled.value ? fakeSource() : real
      return source.watchPosition(...args)
    },
    clearWatch(id) {
      // By ownership, not by the toggle — see `ownedWatches`.
      if (ownedWatches.has(id)) fakeSource().clearWatch(id)
      else real.clearWatch(id)
    },
  }
}

/** Stops every simulated watch.
 *
 * Deliberately does **not** forget which ids were ours. Ownership has to outlive the toggle: if it
 * did not, a watch started against the fake and cleared after switching off would be routed to the
 * real API — which ignores an unknown id, leaving our interval running forever. (Found by test.)
 */
export function stopDevWatches() {
  for (const timer of watches.values()) clearInterval(timer)
  watches.clear()
}
