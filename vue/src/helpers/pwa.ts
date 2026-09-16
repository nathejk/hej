import { registerSW } from 'virtual:pwa-register'

// Holds the vite-plugin-pwa reload function once the service worker registers.
let reloadWithNewVersion: ((reloadPage?: boolean) => Promise<void>) | undefined

// The registration itself, so the app can ask again whether a new build exists (task 298).
let registration: ServiceWorkerRegistration | undefined

// initPwa registers the service worker. `onNeedRefresh` fires when a new build
// is waiting; the app turns this into an update prompt (task 020).
//
// **`registerSW` checks for a new build once, at registration.** That is per *document*, and on iOS an
// installed PWA's document survives for hours across suspend/resume — task 280 measured 47 minutes on
// one document, including a 32-minute suspension. So registration alone left devices sitting on stale
// builds indefinitely, with no way to ship a fix to an already-open app during an event (task 298).
// `checkForUpdate` below is how the app asks again; `useUpdateCheck` decides when.
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
    onRegisteredSW(_url, r) {
      registration = r
    },
    onOfflineReady() {
      // Shell cached; nothing to surface for the skeleton.
    },
  })
}

/**
 * Ask the browser whether a new build is waiting.
 *
 * A conditional request for one small file (`sw.js`), so it is cheap enough to run on a schedule. If a
 * new worker is found, the plugin's own `waiting` handling fires `onNeedRefresh` and the banner appears —
 * this does **not** activate anything or reload: `registerType: 'prompt'` is deliberate, and a fix must
 * never reload the app under someone mid-task, least of all mid-event.
 *
 * Never throws. A failed check is a non-event — offline, or the worker not registered yet — and the next
 * one will do the job.
 */
export async function checkForUpdate(): Promise<void> {
  if (!registration) return
  try {
    await registration.update()
  } catch {
    // Offline, or the registration is gone. Nothing to report and nothing to do.
  }
}

// applyUpdate activates the waiting service worker and reloads into the new
// build. No-op if no update is pending.
export async function applyUpdate() {
  if (reloadWithNewVersion) {
    await reloadWithNewVersion(true)
  }
}
