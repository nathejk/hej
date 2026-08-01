import { registerSW } from 'virtual:pwa-register'

// Holds the vite-plugin-pwa reload function once the service worker registers.
let reloadWithNewVersion: ((reloadPage?: boolean) => Promise<void>) | undefined

// initPwa registers the service worker. `onNeedRefresh` fires when a new build
// is waiting; the app turns this into an update prompt (task 020).
export function initPwa(onNeedRefresh: () => void) {
  reloadWithNewVersion = registerSW({
    immediate: true,
    onNeedRefresh,
    onOfflineReady() {
      // Shell cached; nothing to surface for the skeleton.
    },
  })
}

// applyUpdate activates the waiting service worker and reloads into the new
// build. No-op if no update is pending.
export async function applyUpdate() {
  if (reloadWithNewVersion) {
    await reloadWithNewVersion(true)
  }
}
