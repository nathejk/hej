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

/**
 * How long to wait before *also* trying the coarse one-shot, in ms.
 *
 * The precise attempt is not abandoned — both stay in flight and the first answer wins. That is the
 * correction task 199 makes to task 198: chaining the fallback off the precise attempt's error callback
 * cannot help a call that never calls back, which is exactly what iPadOS does (see the task).
 */
export const GEO_COARSE_AFTER_MS = 6_000

/** Which strategy produced the position. Logged, so a device run can confirm what actually works. */
export type GeoStrategy = 'precise' | 'coarse' | 'watch' | null

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
      // Two safe, cheap steps and nothing else. Reinstalling would probably also work — it is what
      // recovered the iPad this copy exists for — but it clears the app's storage, including a position
      // track that has not been uploaded yet, and a participant cannot know that. Telling them to delete
      // the app is advice that can destroy their own recorded route (task 200); it lives in the device
      // protocol, aimed at organisers, with "get online first" attached.
      return 'Der kom ikke noget svar fra telefonen. Tjek under Indstillinger, at Stedtjenester er slået til, og at Hej Nathejk må bruge din placering. Hjælper det ikke, så luk appen helt og åbn den igen.'
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
    /** Which strategy answered (task 199). Diagnostic only. */
    strategy: null as GeoStrategy,
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
      this.strategy = null

      return new Promise((resolve) => {
        // Three strategies in a race, and the shape is the point (task 199).
        //
        // On the iPad that produced this design, `getCurrentPosition` never calls **either** callback:
        // not an error, silence. Task 198's coarse retry hung off the precise attempt's error handler, so
        // on that device it could never run — a fallback chained to a failure cannot rescue a call that
        // does not fail. Everything here is therefore started independently and the first answer wins:
        //
        //   - `precise`  — high accuracy, straight away. What a phone with GPS should satisfy quickly.
        //   - `watch`    — started straight away too, because it is a *different* WebKit code path, and
        //                  iOS home-screen web apps have a long history of the watch delivering while the
        //                  one-shot hangs. It is also the call the map wants anyway.
        //   - `coarse`   — after a short delay, for hardware with no GNSS receiver at all (task 198).
        //                  Delayed rather than immediate so a device that can do better is given the
        //                  chance to, but not gated on the precise attempt failing.
        let settled = false
        let watchId: number | null = null
        // How many strategies could still answer. The race ends when this reaches zero (everything failed)
        // or when one succeeds — and *not* on the first failure, because a strategy that fails says
        // nothing about the ones still in flight.
        let outstanding = 0
        // Whether the watch is the strategy that answered. Tracked as a flag rather than read back from
        // `watchId`, because a callback can fire **before** `watchPosition` returns its id — a real
        // browser is asynchronous, but nothing in the API promises that, and if it happens the id is lost
        // and the subscription leaks: a high-accuracy watch nobody can stop, on a battery that has to last
        // the night.
        let watchWon = false
        // The most recent cause, which is what the user is told: it describes the last thing actually
        // tried, and by construction that is the coarse attempt when it ran.
        let lastFailure: GeoFailure = null
        let coarseStarted = false

        const cleanup = () => {
          clearTimeout(stuckTimer)
          clearTimeout(coarseTimer)
          // The watch is only dropped when it did NOT win. When it did, it is exactly the subscription
          // the map is about to need, so tearing it down and starting another would throw away the one
          // thing on this device that answered.
          if (watchId !== null && !watchWon) {
            geo.clearWatch(watchId)
            watchId = null
          }
        }

        const settle = (coords: Coords | null) => {
          if (settled) return
          settled = true
          this.requesting = false
          cleanup()
          resolve(coords)
        }

        const onSuccess = (strategy: GeoStrategy) => (pos: GeolocationPosition) => {
          if (settled) return
          this.position = {
            lat: pos.coords.latitude,
            lng: pos.coords.longitude,
            accuracy: pos.coords.accuracy,
          }
          this.positionAt = pos.timestamp || Date.now()
          this.permission = 'granted'
          this.strategy = strategy
          this.coarse = strategy === 'coarse'
          rememberGrant(true)
          this.error = ''
          this.failure = null
          // Set before settling so `cleanup` can see which strategy won.
          if (strategy === 'watch') {
            watchWon = true
            if (watchId !== null) this.watchId = watchId
          }
          settle(this.position)
        }

        const startCoarse = () => {
          if (coarseStarted || settled) return
          coarseStarted = true
          clearTimeout(coarseTimer)
          outstanding++
          geo.getCurrentPosition(onSuccess('coarse'), (err) => handleError(err), GEO_COARSE)
        }

        const stuckTimer = setTimeout(() => {
          if (settled) return
          this.failure = 'stuck'
          this.error = 'no geolocation strategy answered'
          settle(null)
        }, GEO_STUCK_MS)

        const coarseTimer = setTimeout(startCoarse, GEO_COARSE_AFTER_MS)

        // Declared after `startCoarse` because it calls it; `startCoarse` reaches this through a closure
        // rather than a forward reference, hence the arrow wrapper at its call site.
        const handleError = (err: { code?: number; message?: string }) => {
          if (settled) return
          const cause = classifyGeoError(err)

          // A refusal ends the race outright: it is final until the user visits Settings, and on some
          // platforms continuing to ask is what gets a permission permanently blocked.
          if (cause === 'denied') {
            this.error = err.message ?? ''
            this.failure = 'denied'
            this.permission = 'denied'
            rememberGrant(false)
            settle(null)
            return
          }

          lastFailure = cause
          this.error = err.message ?? ''
          outstanding--

          // Nothing else is going to answer, so stop waiting out the delay and try the coarse attempt now.
          // This is what keeps a device that fails *fast* — location switched off, say — from sitting
          // through the full stuck timeout before being told anything, which was the whole point of 197.
          if (outstanding <= 0 && !coarseStarted) {
            startCoarse()
            return
          }

          if (outstanding <= 0) {
            this.failure = lastFailure
            settle(null)
          }
        }

        outstanding++
        geo.getCurrentPosition(onSuccess('precise'), handleError, GEO_PRECISE)

        // Wrapped: a browser without `watchPosition`, or one that throws on it, must not take the
        // one-shot down with it.
        //
        // Skipped entirely once something has already answered — otherwise a one-shot that resolves
        // synchronously would leave a watch running that nothing ever clears.
        if (!settled) {
          try {
            outstanding++
            watchId = geo.watchPosition(onSuccess('watch'), handleError, GEO_PRECISE)
            // The callback may have fired during the call above. Adopt the id if the watch won; drop the
            // subscription if the race finished without it.
            if (watchWon) this.watchId = watchId
            else if (settled && watchId !== null) {
              geo.clearWatch(watchId)
              watchId = null
            }
          } catch {
            outstanding--
            watchId = null
          }
        }
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
