import { defineStore } from 'pinia'

// app store holds cross-cutting app-shell state. It will grow to own things
// like the "update available" flag and bottom-nav overflow state (see PRD 001).
export const useAppStore = defineStore('app', {
  state: () => ({
    // Set once the service worker reports a new version is waiting (task 020).
    updateAvailable: false,
    // Whether the BFF is currently reachable (task 090).
    //
    // Seeded from navigator.onLine but corrected by what actually happens: onLine is
    // only "this device has a network interface with a route", so it is true on a
    // captive-portal WiFi, true with one bar and no throughput, and true on the
    // event's own patchy coverage. A failed request is the real evidence, so
    // fetchWrapper's NetworkError drives this too, via session.store.
    //
    // It says nothing about whether the *app* works: the shell and map tiles come
    // from the service worker's caches either way. It only decides what the user is
    // told.
    online: typeof navigator === 'undefined' ? true : navigator.onLine,
  }),
  actions: {
    setUpdateAvailable(value: boolean) {
      this.updateAvailable = value
    },
    setOnline(value: boolean) {
      this.online = value
    },
  },
})
