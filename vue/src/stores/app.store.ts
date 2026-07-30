import { defineStore } from 'pinia'

// app store holds cross-cutting app-shell state. It will grow to own things
// like the "update available" flag and bottom-nav overflow state (see PRD 001).
export const useAppStore = defineStore('app', {
  state: () => ({
    // Set once the service worker reports a new version is waiting (task 020).
    updateAvailable: false,
  }),
  actions: {
    setUpdateAvailable(value: boolean) {
      this.updateAvailable = value
    },
  },
})
