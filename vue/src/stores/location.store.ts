import { defineStore } from 'pinia'

export type GeoPermission = 'unknown' | 'prompt' | 'granted' | 'denied' | 'unavailable'

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
    // Whether the map should keep recentring on the position. Manual panning
    // turns this off; the locate button turns it back on.
    following: true,
    watchId: null as number | null,
  }),
  getters: {
    available: () => typeof navigator !== 'undefined' && 'geolocation' in navigator,
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
    request(): Promise<Coords | null> {
      if (!this.available) {
        this.permission = 'unavailable'
        return Promise.resolve(null)
      }
      return new Promise((resolve) => {
        navigator.geolocation.getCurrentPosition(
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
            resolve(this.position)
          },
          (err) => {
            this.error = err.message
            if (err.code === err.PERMISSION_DENIED) {
              this.permission = 'denied'
              rememberGrant(false)
            }
            resolve(null)
          },
          { enableHighAccuracy: true, timeout: 10_000, maximumAge: 30_000 },
        )
      })
    },

    // watch starts (or reuses) a single continuous position subscription. Callers
    // must pair it with stopWatch() — the map does so on unmount and whenever the
    // page is hidden, since a high-accuracy watch is expensive on battery.
    watch() {
      if (!this.available || this.watchId !== null) {
        return
      }
      this.watchId = navigator.geolocation.watchPosition(
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
        },
        (err) => {
          this.error = err.message
          if (err.code === err.PERMISSION_DENIED) {
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
      if (message) this.error = message
      rememberGrant(false)
      this.stopWatch()
    },

    stopWatch() {
      if (this.watchId !== null) {
        navigator.geolocation.clearWatch(this.watchId)
        this.watchId = null
      }
    },

    setFollowing(value: boolean) {
      this.following = value
    },
  },
})
