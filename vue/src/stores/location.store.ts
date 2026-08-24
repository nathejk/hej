import { defineStore } from 'pinia'

export type GeoPermission = 'unknown' | 'prompt' | 'granted' | 'denied' | 'unavailable'

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
      if ('permissions' in navigator) {
        try {
          const status = await navigator.permissions.query({
            name: 'geolocation',
          } as PermissionDescriptor)
          this.permission = status.state as GeoPermission
          status.onchange = () => {
            this.permission = status.state as GeoPermission
          }
        } catch {
          // Some browsers don't support querying geolocation permission; leave
          // it 'unknown' and let request() resolve it.
        }
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
            this.permission = 'granted'
            this.error = ''
            resolve(this.position)
          },
          (err) => {
            this.error = err.message
            if (err.code === err.PERMISSION_DENIED) {
              this.permission = 'denied'
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
          this.permission = 'granted'
          this.error = ''
        },
        (err) => {
          this.error = err.message
          if (err.code === err.PERMISSION_DENIED) {
            this.permission = 'denied'
            // A denied watch will never fire; drop it so we don't hold a dead
            // subscription (and so a later grant can start a fresh one).
            this.stopWatch()
          }
        },
        { enableHighAccuracy: true, timeout: 10_000, maximumAge: 5_000 },
      )
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
