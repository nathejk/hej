import { defineStore } from 'pinia'
import { fetchWrapper } from '@/helpers'

export type NotifPermission = 'unknown' | 'default' | 'granted' | 'denied' | 'unavailable'

// Convert a URL-safe base64 VAPID key to the Uint8Array the Push API wants.
function urlBase64ToUint8Array(base64String: string): Uint8Array<ArrayBuffer> {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4)
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/')
  const raw = atob(base64)
  const out = new Uint8Array(new ArrayBuffer(raw.length))
  for (let i = 0; i < raw.length; i += 1) {
    out[i] = raw.charCodeAt(i)
  }
  return out
}

// notifications.store owns Web Push: requesting permission, subscribing via the
// service worker with the server VAPID key, and registering the subscription
// with the BFF (tied to the signed-in user). Delivery/fan-out is a later PRD.
export const useNotificationsStore = defineStore('notifications', {
  state: () => ({
    // configured records whether the SERVER has Web Push set up, i.e. whether a VAPID
    // public key exists. null until asked.
    //
    // Worth knowing separately from `available` and `permission`, because it is the one
    // failure the member can do nothing about: with no key, subscribing cannot succeed no
    // matter how many times they tap. Found on a real device 2026-08-29, where a granted
    // permission plus an unconfigured server produced a Tilmeld button that did nothing.
    configured: null as boolean | null,
    permission: 'unknown' as NotifPermission,
    subscribed: false,
    error: '',
  }),
  getters: {
    available: () =>
      typeof window !== 'undefined' &&
      'Notification' in window &&
      'serviceWorker' in navigator &&
      'PushManager' in window,
  },
  actions: {
    syncPermission() {
      if (!this.available) {
        this.permission = 'unavailable'
        return
      }
      this.permission = Notification.permission as NotifPermission
    },

    // syncSubscription reads the live PushSubscription and sets `subscribed` from it.
    //
    // Without this, `subscribed` is only ever set inside enable(), so after a reload
    // the store believes nobody is subscribed — and PRD 003's status row would then
    // tell a subscribed user that push is off.
    //
    // Note that permission and subscription are genuinely independent:
    // `permission === 'granted'` with **no** subscription is a real state (the browser
    // can drop a subscription, and replacing the service worker loses it), and it is
    // precisely the state where push silently does not work. The row has to be able to
    // say so, so this must not be collapsed into syncPermission().
    //
    // Never throws — a status row is not worth breaking a page over.
    async syncSubscription() {
      if (!this.available) {
        this.subscribed = false
        return
      }
      try {
        // getRegistration(), not `ready`: `ready` never resolves when no service
        // worker is registered, which would hang this call forever instead of
        // answering "not subscribed".
        const registration = await navigator.serviceWorker.getRegistration()
        if (!registration) {
          this.subscribed = false
          return
        }
        const subscription = await registration.pushManager.getSubscription()
        this.subscribed = subscription !== null
      } catch {
        // A refused pushManager (private mode, or an unsupported build) is not
        // evidence of a subscription.
        this.subscribed = false
      }
    },

    // syncConfigured asks the BFF whether push is set up at all.
    //
    // Never throws: an unreachable server leaves `configured` null, which the UI treats as
    // "unknown" rather than as "broken" — being offline is not evidence that push is
    // unconfigured.
    async syncConfigured() {
      try {
        const { public_key: publicKey } = await fetchWrapper.get<{ public_key: string }>(
          '/api/push/public-key',
        )
        this.configured = Boolean(publicKey)
      } catch {
        this.configured = null
      }
    },

    // enable requests permission and, if granted, subscribes to push and sends
    // the subscription to the BFF. Returns whether the user is now subscribed.
    // Degrades gracefully (never throws) so callers can handle a false result.
    async enable(): Promise<boolean> {
      if (!this.available) {
        this.permission = 'unavailable'
        return false
      }

      const result = await Notification.requestPermission()
      this.permission = result as NotifPermission
      if (result !== 'granted') {
        return false
      }

      try {
        const { public_key: publicKey } = await fetchWrapper.get<{ public_key: string }>(
          '/api/push/public-key',
        )
        if (!publicKey) {
          this.configured = false
          this.error = 'Nathejk har ikke sat notifikationer op endnu.'
          return false
        }
        this.configured = true

        const registration = await navigator.serviceWorker.ready
        const subscription = await registration.pushManager.subscribe({
          userVisibleOnly: true,
          applicationServerKey: urlBase64ToUint8Array(publicKey),
        })

        const json = subscription.toJSON()
        await fetchWrapper.post('/api/push/subscription', {
          endpoint: json.endpoint,
          keys: json.keys,
        })

        this.subscribed = true
        this.error = ''
        return true
      } catch (err) {
        this.error = err instanceof Error ? err.message : 'Kunne ikke tilmelde notifikationer.'
        return false
      }
    },
  },
})
