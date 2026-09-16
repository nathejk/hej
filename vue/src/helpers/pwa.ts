import { registerSW } from 'virtual:pwa-register'

// Holds the vite-plugin-pwa reload function once the service worker registers.
let reloadWithNewVersion: ((reloadPage?: boolean) => Promise<void>) | undefined

// initPwa registers the service worker. `onNeedRefresh` fires when a new build
// is waiting; the app turns this into an update prompt (task 020).
//
// **Known gap (task 298): this checks for a new build once per document load and never again.**
// `registerSW` triggers an update check at registration; nothing here calls `registration.update()`
// afterwards. On iOS an installed PWA's document survives for hours across suspend/resume (task 280
// measured 47 minutes and a 32-minute suspension on one document), so a device can sit on a stale build
// indefinitely — which also means there is currently no way to ship a fix to already-open apps during
// an event. Do not treat the banner's absence as "no update available" until that is fixed.
//
// Note what `registerType: 'prompt'` means for a user who taps "Senere": the waiting worker stays
// waiting, so `onNeedRefresh` fires again on the next launch and the banner comes back — every launch,
// until they accept it. That is the intended trade (never reload under someone mid-task, especially
// mid-event), but it is worth knowing when reading a bug report: "the app reloads itself on launch" and
// "the app nags on every launch" are both this flow working as designed. Task 297 chased the first one
// for a while before noticing the test session had a new build every few minutes.
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
