/* global self, clients */
// Custom service-worker logic imported by the Workbox-generated SW (see
// vite.config.ts → VitePWA workbox.importScripts). Handles incoming push
// messages and notification clicks. Push delivery/fan-out is a later PRD; this
// is the client-side receiver.

self.addEventListener('push', (event) => {
  let data = {}
  try {
    data = event.data ? event.data.json() : {}
  } catch (e) {
    data = { body: event.data ? event.data.text() : '' }
  }
  const title = data.title || 'Hej Nathejk'
  const options = {
    body: data.body || '',
    icon: '/pwa-192.png',
    // Distinct from `icon`: Android renders `badge` as a flat silhouette from the
    // alpha channel alone, so the app icon would show up as a solid square (its
    // dark ground is opaque edge to edge). badge-96.png is the bare crescent on
    // transparency.
    badge: '/badge-96.png',
    data: { url: data.url || '/' },
  }
  event.waitUntil(self.registration.showNotification(title, options))
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const url = (event.notification.data && event.notification.data.url) || '/'
  event.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true }).then((windowClients) => {
      for (const client of windowClients) {
        if (client.url.includes(url) && 'focus' in client) {
          return client.focus()
        }
      }
      if (clients.openWindow) {
        return clients.openWindow(url)
      }
      return undefined
    }),
  )
})
