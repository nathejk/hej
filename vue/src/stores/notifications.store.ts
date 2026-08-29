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

// The reverse, so a subscription's own key can be compared with the server's.
function uint8ArrayToUrlBase64(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer)
  let binary = ''
  for (const byte of bytes) {
    binary += String.fromCharCode(byte)
  }
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

// Where the key a subscription was created with is remembered.
//
// A fallback for `PushSubscription.options.applicationServerKey`, which is the
// authoritative answer but is not guaranteed to be exposed on every engine we support.
// Between the two, "which key is this subscription bound to?" is always answerable.
const SUBSCRIBED_KEY_STORAGE = 'hej.push.serverKey'

function rememberSubscribedKey(key: string) {
  try {
    localStorage.setItem(SUBSCRIBED_KEY_STORAGE, key)
  } catch {
    // Private mode can refuse. Costs only the fallback; options.applicationServerKey
    // still answers on engines that expose it.
  }
}

function subscribedKeyFallback(): string | null {
  try {
    return localStorage.getItem(SUBSCRIBED_KEY_STORAGE)
  } catch {
    return null
  }
}

// keyBehind returns the VAPID public key a subscription was created with, or null when it
// cannot be determined.
function keyBehind(subscription: PushSubscription): string | null {
  const applied = subscription.options?.applicationServerKey
  if (applied) {
    try {
      return uint8ArrayToUrlBase64(applied)
    } catch {
      // Fall through to the remembered value.
    }
  }
  return subscribedKeyFallback()
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
    // serverKey is the VAPID public key the server currently advertises.
    //
    // Kept, not just "is it configured", because it is what makes a stale subscription
    // detectable: a PushSubscription is bound to the key it was created with, so
    // comparing the two answers "can this subscription still receive anything?".
    //
    // This is why no VAPID_SEQUENCE or key-id is needed — the key identifies itself, and
    // a counter would be a second source of truth that can drift from the keys it
    // describes (rotate and forget to bump, and every client keeps a dead subscription).
    serverKey: null as string | null,
    // registeredEndpoint is the endpoint last successfully handed to the BFF in this
    // session. In memory on purpose: it must not survive a reload, because a reload is
    // exactly when re-registering is useful (see syncSubscription).
    registeredEndpoint: null as string | null,
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
        if (!subscription) {
          this.subscribed = false
          return
        }

        // A subscription bound to a key the server no longer uses cannot receive
        // anything, and nothing about it looks broken from here — which is the trap: it
        // still exists, so the naive check reports "subscribed" and the profile row says
        // notifications are on while delivery is silently impossible.
        //
        // Detected by comparing keys rather than by a rotation counter, because the key
        // is its own identifier and cannot fall out of step with itself.
        const boundTo = keyBehind(subscription)
        if (this.serverKey && boundTo && boundTo !== this.serverKey) {
          // Self-healing, and it costs the member nothing: permission is already
          // granted, so re-subscribing raises no prompt. They never learn a rotation
          // happened.
          try {
            await subscription.unsubscribe()
          } catch {
            // Unsubscribing can fail while offline. Leaving the old subscription in
            // place is harmless — the next sync tries again.
          }
          this.subscribed = false
          await this.enable()
          return
        }

        this.subscribed = true

        // Re-register the subscription the server already has.
        //
        // Not redundant: the BFF's subscription store is in-memory today
        // (push.NewMemoryStore), so **every restart forgets every subscriber** while
        // their browsers keep a perfectly valid subscription. Without this, a deploy
        // silently unsubscribes the whole event and every client still displays "Til".
        //
        // Cheap and safe to repeat: the endpoint is idempotent per (user, endpoint).
        // Guarded per endpoint so returning to the page does not re-post on every
        // visibility change; a page load re-posts once, which is the frequency that
        // matters since a restarted server needs exactly one.
        if (this.registeredEndpoint !== subscription.endpoint) {
          await this.registerSubscription(subscription)
        }
      } catch {
        // A refused pushManager (private mode, or an unsupported build) is not
        // evidence of a subscription.
        this.subscribed = false
      }
    },

    // registerSubscription hands a subscription to the BFF.
    //
    // Never throws: failing to register is worth retrying, not worth breaking the profile
    // page for, and `registeredEndpoint` is only set on success so the next sync retries.
    async registerSubscription(subscription: PushSubscription) {
      const json = subscription.toJSON()
      try {
        await fetchWrapper.post('/api/push/subscription', {
          endpoint: json.endpoint,
          keys: json.keys,
        })
        this.registeredEndpoint = subscription.endpoint
      } catch {
        this.registeredEndpoint = null
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
        this.serverKey = publicKey || null
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
        this.serverKey = publicKey

        const registration = await navigator.serviceWorker.ready
        const subscription = await registration.pushManager.subscribe({
          userVisibleOnly: true,
          applicationServerKey: urlBase64ToUint8Array(publicKey),
        })

        // Remembered here, next to the subscribe that used it, so a later sync can tell
        // whether the server has rotated away from it. The engine's own
        // `options.applicationServerKey` is preferred when available; this is the
        // fallback that is always under our control.
        rememberSubscribedKey(publicKey)

        await this.registerSubscription(subscription)
        if (!this.registeredEndpoint) {
          // The subscription exists in the browser but the server does not know about it,
          // so push would not arrive. Reported as a failure rather than a success — the
          // profile row now shows this instead of silently repeating "Tilmeld".
          this.error = 'Tilmeldingen kunne ikke gemmes. Prøv igen.'
          return false
        }

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
