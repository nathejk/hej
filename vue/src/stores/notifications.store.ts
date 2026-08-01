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
          this.error = 'Push er ikke konfigureret på serveren.'
          return false
        }

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
