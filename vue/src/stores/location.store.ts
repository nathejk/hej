import { defineStore } from 'pinia'

export type GeoPermission = 'unknown' | 'prompt' | 'granted' | 'denied' | 'unavailable'

/**
 * Why the last attempt to get a position failed.
 *
 * Four causes, and they are kept apart because the user's next move differs for each — which is the whole
 * lesson of task 197, where every one of them looked identical (like nothing at all):
 *
 *  - `denied` — this site was refused. Recoverable only in Settings; `blockedGuidance` says how.
 *  - `unavailable` — iOS reports `POSITION_UNAVAILABLE`, which on an iPhone or iPad most often means
 *    **Location Services is off for the whole device**. Telling that user "you denied this app" would
 *    send them to the wrong screen.
 *  - `timeout` — no fix in time. Indoors, or a WiFi-only iPad with nothing to triangulate. Worth
 *    retrying, unlike the two above.
 *  - `stuck` — neither callback ever fired. Not a Geolocation error code at all: it is our own
 *    wall-clock guard, because the API's `timeout` bounds acquiring a fix and does **not** cover waiting
 *    for the permission dialog to be answered. Without this the UI waits for ever.
 */
export type GeoFailure = 'denied' | 'unavailable' | 'timeout' | 'stuck' | null

/**
 * How long to wait before concluding the browser is never going to answer, in ms.
 *
 * Comfortably longer than the 10 s fix timeout, because the legitimate slow case is a human reading an
 * iOS dialog and deciding — cutting that off early would report a failure to someone who is about to
 * grant permission. Short enough that a button cannot appear dead: 25 s of a spinner is annoying,
 * whereas an unbounded wait is what task 197 was filed for.
 */
export const GEO_STUCK_MS = 25_000

/**
 * Options for the first attempt: precise, and quick to give up.
 *
 * High accuracy is right as a *first* choice — the position track wants precision, since 30 s sampling at
 * walking pace puts points 33–50 m apart and a GPS error of 10–30 m is already most of that.
 */
export const GEO_PRECISE: PositionOptions = {
  enableHighAccuracy: true,
  timeout: 10_000,
  maximumAge: 30_000,
}

/**
 * Options for the second attempt: coarse, patient, and willing to accept an old fix.
 *
 * For devices with **no GNSS receiver at all** — every Wi-Fi-only iPad, including the iPad 6th generation
 * that found this bug (task 198). Their only source is Apple's Wi-Fi network database, which is slower and
 * coarser than GPS and fails outright where the surrounding networks are unknown. Asking such a device for
 * high accuracy is asking for something the hardware cannot do; the answer is a slow failure, or none.
 *
 * Both relaxations matter and neither is padding: the longer timeout is what a network lookup needs, and
 * accepting a two-minute-old fix is what makes a *cached* Wi-Fi position usable instead of demanding a
 * fresh lookup that may not succeed.
 *
 * A coarse fix is a normal success. ±500 m still answers "which end of the forest is this patrol in",
 * which is the question that matters at 03:00, and the map's accuracy circle draws the uncertainty
 * honestly without any extra copy.
 */
export const GEO_COARSE: PositionOptions = {
  enableHighAccuracy: false,
  timeout: 15_000,
  maximumAge: 120_000,
}

// Remembers that this device has granted location at least once.
//
// WHY THIS IS NECESSARY, measured on an iPhone (iOS 18.7, task 082's device run).
// **WebKit's `navigator.permissions.query({name: 'geolocation'})` answers `prompt` even
// when permission is granted** — Safari does not expose a queryable "granted" state for
// geolocation. On a cold start the app therefore concluded it had no permission, refused to
// start the track recorder, and stayed off until the user happened to open the map, which
// is the only thing that produced a successful fix and corrected the state.
//
// That was worse than it sounds. iOS kills a backgrounded standalone web app aggressively
// — the device run recorded three cold loads in eight minutes — so in practice recording
// was off for most of a session unless the participant kept returning to the map. Task 082
// moved the recorder out of `MapsView` so the track would not depend on looking at the
// map; permission detection had quietly re-coupled them.
//
// A successful fix is direct evidence of a grant, so it is remembered here and trusted
// over the Permissions API. It is self-correcting: if the user revokes access in iOS
// Settings, the next fix fails with PERMISSION_DENIED, which clears this and sets `denied`.
//
// It is also why we can call `getCurrentPosition` on startup without being rude — we only
// do so for a device that has already agreed. A device that has not is left alone, so
// PRD 002's deliberate consent copy still comes before any system dialog.
const GRANT_KEY = 'hej.geo-granted'

function rememberGrant(granted: boolean) {
  try {
    if (granted) localStorage.setItem(GRANT_KEY, '1')
    else localStorage.removeItem(GRANT_KEY)
  } catch {
    // Private mode can refuse; costs only the cold-start optimisation.
  }
}

function grantRemembered(): boolean {
  try {
    return localStorage.getItem(GRANT_KEY) === '1'
  } catch {
    return false
  }
}

export interface Coords {
  lat: number
  lng: number
  accuracy: number
}

/**
 * The part of `navigator.geolocation` this store uses.
 *
 * Injectable for the same reason as `helpers/platform.ts`'s environment: the interesting cases here — a
 * call that hangs, `POSITION_UNAVAILABLE` from a device with Location Services off — cannot be produced
 * on the machine running the tests, so they have to be handed in.
 */
export interface GeolocationLike {
  getCurrentPosition: (
    success: PositionCallback,
    error?: PositionErrorCallback | null,
    options?: PositionOptions,
  ) => void
  watchPosition: (
    success: PositionCallback,
    error?: PositionErrorCallback | null,
    options?: PositionOptions,
  ) => number
  clearWatch: (id: number) => void
}

function browserGeolocation(): GeolocationLike | null {
  if (typeof navigator === 'undefined' || !('geolocation' in navigator)) return null
  return navigator.geolocation
}

/**
 * Fold a `GeolocationPositionError` into one of our causes.
 *
 * The numeric codes are used rather than the `err.PERMISSION_DENIED` constants: those live on the error
 * instance, and a stubbed error in a test — or a browser that returns a plain object — does not carry
 * them. The values are fixed by the spec (1, 2, 3).
 */
export function classifyGeoError(err: { code?: number }): GeoFailure {
  switch (err.code) {
    case 1:
      return 'denied'
    case 2:
      return 'unavailable'
    case 3:
      return 'timeout'
    default:
      // An error we do not recognise is still a failure, and "we could not get a fix" is the honest
      // reading of it — not "you denied this", which would send the user to the wrong Settings screen.
      return 'unavailable'
  }
}

/** Danish, plain, and specific to the cause — task 197. */
export function geoFailureMessage(failure: GeoFailure): string {
  switch (failure) {
    case 'denied':
      return 'Appen har ikke lov til at bruge din placering.'
    case 'unavailable':
      // The likeliest cause on an iPhone or iPad, and the one nobody guesses: the setting is off for the
      // whole device, not for this app.
      return 'Telefonen ville ikke oplyse din placering. Tjek at Stedtjenester er slået til under Indstillinger → Anonymitet og sikkerhed.'
    case 'timeout':
      return 'Det tog for lang tid at finde din placering. Prøv igen — helst udendørs.'
    case 'stuck':
      return 'Der kom ikke noget svar fra telefonen. Prøv igen, eller tjek Stedtjenester under Indstillinger.'
    default:
      return ''
  }
}

// location.store wraps the browser Geolocation API in a permission-aware store.
// Requires a secure context (HTTPS) — provided by the dev Traefik setup and prod.
//
// `request()` is a one-shot read (used by the soft permission prompt); `watch()`
// starts the continuous subscription the map consumes. The position is never sent
// to the BFF — see PRD 002, which rules out server-side tracking.
export const useLocationStore = defineStore('location', {
  state: () => ({
    permission: 'unknown' as GeoPermission,
    position: null as Coords | null,
    // When `position` was acquired, in epoch ms. Lets a consumer tell a fresh fix from
    // a stale one — the track recorder (task 082) reuses the map's fix when it is
    // recent enough and only asks for its own when it is not, which is what keeps
    // recording free while the map is open.
    positionAt: 0,
    error: '',
    /**
     * Why the last attempt failed, or null. Task 197.
     *
     * Separate from `error`, which holds WebKit's own English string: that is worth keeping for the
     * diagnostic log and is not worth showing to a Danish twelve-year-old.
     */
    failure: null as GeoFailure,
    /** True while a one-shot request is outstanding, so a tap is never silent. */
    requesting: false,
    /**
     * True when the current position came from the coarse fallback rather than GPS (task 198).
     *
     * Not shown to the user — the accuracy circle already draws the uncertainty, and a Wi-Fi fix is a
     * normal success, not a degraded one. It exists so a device run can tell "GPS worked" from "only
     * Wi-Fi worked", which is the difference between an iPhone and a Wi-Fi-only iPad.
     */
    coarse: false,
    // Whether the map should keep recentring on the position. Manual panning
    // turns this off; the locate button turns it back on.
    following: true,
    watchId: null as number | null,
    /** Injected in tests; the browser's own by default. */
    geo: browserGeolocation() as GeolocationLike | null,
  }),
  getters: {
    available: (state) => state.geo !== null,
    watching: (state) => state.watchId !== null,
  },
  actions: {
    // syncPermission reads the current permission state without prompting.
    async syncPermission() {
      if (!this.available) {
        this.permission = 'unavailable'
        return
      }
      // Our own evidence first: see the note on GRANT_KEY. On WebKit the query below
      // would otherwise talk us out of a permission we demonstrably have.
      if (grantRemembered()) {
        this.permission = 'granted'
      }
      if ('permissions' in navigator) {
        try {
          const status = await navigator.permissions.query({
            name: 'geolocation',
          } as PermissionDescriptor)
          this.applyPermissionState(status.state)
          status.onchange = () => {
            this.applyPermissionState(status.state)
          }
        } catch {
          // Some browsers don't support querying geolocation permission; leave
          // it as it stands and let request() resolve it.
        }
      }
    },

    // applyPermissionState folds a Permissions API answer into our state.
    //
    // `granted` and `denied` are believed outright — they are definite. `prompt` is not:
    // WebKit reports it for a granted permission too, so it must not overwrite a
    // remembered grant, or every cold start on iOS would look like a fresh install.
    applyPermissionState(state: PermissionState) {
      if (state === 'granted') {
        this.permission = 'granted'
        rememberGrant(true)
      } else if (state === 'denied') {
        this.permission = 'denied'
        rememberGrant(false)
      } else if (!grantRemembered()) {
        this.permission = 'prompt'
      }
    },

    // request prompts for (or reuses) permission and reads the current position.
    // Resolves to the coords on success, or null on denial/error/unavailable —
    // it never rejects, so callers can degrade gracefully.
    //
    // **Guarded by our own clock** (task 197). The `timeout` option below bounds acquiring a fix; it
    // does not bound waiting for the permission dialog to be answered. On the iPad that filed this
    // task neither callback ever fired, so without a wall-clock guard the promise never settles and the
    // button is indistinguishable from a dead one. Whatever WebKit is doing, the app now has an answer.
    request(): Promise<Coords | null> {
      const geo = this.geo
      if (!geo) {
        this.permission = 'unavailable'
        this.failure = 'unavailable'
        return Promise.resolve(null)
      }

      this.requesting = true
      this.failure = null
      this.error = ''
      this.coarse = false

      return new Promise((resolve) => {
        // Whichever arrives first wins; the loser is ignored. A late success after the guard has fired
        // is still worth keeping — the position is good — but it must not resolve twice.
        let settled = false
        const settle = (coords: Coords | null) => {
          if (settled) return
          settled = true
          this.requesting = false
          clearTimeout(timer)
          resolve(coords)
        }

        const timer = setTimeout(() => {
          if (settled) return
          this.failure = 'stuck'
          this.error = 'geolocation did not answer'
          settle(null)
        }, GEO_STUCK_MS)

        const onSuccess = (coarse: boolean) => (pos: GeolocationPosition) => {
          this.position = {
            lat: pos.coords.latitude,
            lng: pos.coords.longitude,
            accuracy: pos.coords.accuracy,
          }
          this.positionAt = pos.timestamp || Date.now()
          this.permission = 'granted'
          this.coarse = coarse
          rememberGrant(true)
          this.error = ''
          this.failure = null
          settle(this.position)
        }

        const onFinalError = (err: { code?: number; message?: string }) => {
          this.error = err.message ?? ''
          this.failure = classifyGeoError(err)
          if (this.failure === 'denied') {
            this.permission = 'denied'
            rememberGrant(false)
          }
          settle(null)
        }

        geo.getCurrentPosition(
          onSuccess(false),
          (err) => {
            const cause = classifyGeoError(err)

            // A refusal is final. Retrying it would be pointless — the answer cannot change without a
            // trip to Settings — and on some platforms repeat requests are what get a permission
            // permanently blocked.
            if (cause === 'denied') {
              onFinalError(err)
              return
            }

            // Second attempt, coarse (task 198). On a device with no GPS chip the first request asked for
            // something the hardware cannot provide; this asks for what it can.
            geo.getCurrentPosition(onSuccess(true), onFinalError, GEO_COARSE)
          },
          GEO_PRECISE,
        )
      })
    },

    // watch starts (or reuses) a single continuous position subscription. Callers
    // must pair it with stopWatch() — the map does so on unmount and whenever the
    // page is hidden, since a high-accuracy watch is expensive on battery.
    watch() {
      const geo = this.geo
      if (!geo || this.watchId !== null) {
        return
      }
      this.watchId = geo.watchPosition(
        (pos) => {
          this.position = {
            lat: pos.coords.latitude,
            lng: pos.coords.longitude,
            accuracy: pos.coords.accuracy,
          }
          this.positionAt = pos.timestamp || Date.now()
          this.permission = 'granted'
          rememberGrant(true)
          this.error = ''
          this.failure = null
        },
        (err) => {
          this.error = err.message
          this.failure = classifyGeoError(err)
          if (this.failure === 'denied') {
            this.permission = 'denied'
            rememberGrant(false)
            // A denied watch will never fire; drop it so we don't hold a dead
            // subscription (and so a later grant can start a fresh one).
            this.stopWatch()
          }
        },
        { enableHighAccuracy: true, timeout: 10_000, maximumAge: 5_000 },
      )
    },

    // markDenied records a denial observed elsewhere (e.g. the track recorder's own fix
    // request), so the remembered grant is cleared in one place rather than two.
    markDenied(message = '') {
      this.permission = 'denied'
      this.failure = 'denied'
      if (message) this.error = message
      rememberGrant(false)
      this.stopWatch()
    },

    stopWatch() {
      if (this.watchId !== null) {
        this.geo?.clearWatch(this.watchId)
        this.watchId = null
      }
    },

    setFollowing(value: boolean) {
      this.following = value
    },
  },
})
