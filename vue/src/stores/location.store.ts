import { defineStore } from 'pinia'

export type GeoPermission = 'unknown' | 'prompt' | 'granted' | 'denied' | 'unavailable'

export interface Coords {
  lat: number
  lng: number
  accuracy: number
}

// location.store wraps the browser Geolocation API in a permission-aware store.
// Map rendering that consumes `position` lands in a later feature PRD; this is
// just the plumbing (request / read / permission state). Requires a secure
// context (HTTPS) — provided by the dev Traefik setup and prod.
export const useLocationStore = defineStore('location', {
  state: () => ({
    permission: 'unknown' as GeoPermission,
    position: null as Coords | null,
    error: '',
  }),
  getters: {
    available: () => typeof navigator !== 'undefined' && 'geolocation' in navigator,
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
  },
})
