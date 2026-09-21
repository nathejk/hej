// The public map island (PRD 011 §6, §8; task 342).
//
// # Why this is a separate script and not part of the app
//
// The public pages are server-rendered on purpose: they exist so a grandparent on a ten-year-old browser
// can see what the weekend looked like, and anything needing the app's bundle to parse defeats the only
// thing they are for (PRD 011 §8). So `EventMap.vue` cannot be reused — it is a Vue component in that
// bundle — and this is a small island instead: it draws what an endpoint gives it and holds no state.
//
// **The line to hold:** if this grows routing, layer switching, or a store, it has become the app and
// belongs in the app. It is ~200 lines and should stay that way.
//
// # It is an enhancement, never a requirement
//
// The page is complete without it. Where this does not run — JavaScript off, a browser too old for
// Leaflet, the CDN blocked — the container simply stays as the server rendered it, and the scan list
// carries the same information as a list. Nothing here throws a visitor a broken frame.
//
// # The duplication, named rather than hidden
//
// The WMS layer below is duplicated from `src/config/map.ts`, which is the app's source of truth and is
// TypeScript inside the bundle this file may not load. A build-time generator (the pattern PRD 013
// recommends for the rulebook, following `scripts/generate-icons.sh`) would remove the duplication, and
// was judged too much machinery for one URL and one layer name. If a second layer ever appears here,
// that judgement should be revisited.

(function () {
  'use strict'

  var container = document.getElementById('patrolmap')
  if (!container || typeof window.L === 'undefined') {
    // No container means the server decided there was no route to draw. No Leaflet means an old browser
    // or a blocked asset. Both are fine and both leave the page exactly as it was.
    return
  }

  // Duplicated from src/config/map.ts — see the header. `dtk25` is the topographic layer the app opens
  // with, so the public map is recognisably the same place as the one participants used.
  var WMS_URL = 'https://api.dataforsyningen.dk/dkskaermkort_DAF'
  var WMS_LAYER = 'dtk_skaermkort_daempet'
  var ATTRIBUTION = 'Kort: <a href="https://dataforsyningen.dk">Dataforsyningen</a>'

  var patrolNumber = container.getAttribute('data-patrol')
  if (!patrolNumber) return

  // The token is fetched rather than templated into the page, so a cached HTML page cannot pin a rotated
  // token — the same reason the app reads it from /api/config at runtime.
  fetchJSON('/api/config')
    .then(function (config) {
      return Promise.all([
        fetchJSON('/api/public/patrol/' + encodeURIComponent(patrolNumber) + '/map'),
        // Album photographs are a nice-to-have on this map: a failure must not cost the route. Resolved
        // to an empty set rather than rejecting the whole chain.
        fetchJSON('/api/public/albums').catch(function () {
          return { photos: [] }
        }),
        config,
      ])
    })
    .then(function (results) {
      draw(results[0], results[1], results[2])
    })
    .catch(function () {
      // Deliberately silent to the visitor. The scan list below already carries what the map would have
      // shown, so an error banner would be noise about a feature they may not have noticed was missing.
      if (window.console && console.warn) {
        console.warn('the public map could not be drawn')
      }
    })

  function fetchJSON(url) {
    return fetch(url, { credentials: 'omit' }).then(function (res) {
      if (!res.ok) throw new Error(url + ' answered ' + res.status)
      return res.json()
    })
  }

  function draw(map_, albums, config) {
    var L = window.L

    // Revealed before Leaflet initialises, because Leaflet measures the container: a display:none element
    // has no size and the map would come up 0×0. The caveat next to it is revealed with it — it explains
    // the route's gaps, and only makes sense once there is a route on screen.
    container.classList.add('ready')
    var caveat = container.parentNode && container.parentNode.querySelector('.mapcaveat')
    if (caveat) caveat.classList.add('ready')

    var map = L.map(container, {
      // No attribution control of our own: the WMS layer brings one.
      zoomControl: true,
      // The public map is for looking at, not for navigating. Keyboard panning stays on for
      // accessibility; the rest of the interaction is drag and pinch, which need no configuration.
      scrollWheelZoom: true,
    })

    L.tileLayer
      .wms(WMS_URL, {
        layers: WMS_LAYER,
        format: 'image/png',
        transparent: false,
        attribution: ATTRIBUTION,
        token: config.dataforsyningen_token || '',
        maxZoom: 19,
      })
      .addTo(map)

    var bounds = L.latLngBounds([])

    // **The route, as separate strokes.** The endpoint sends a list of segments precisely so a gap in the
    // recording renders as a break: one flat polyline would draw a confident line through terrain nobody
    // walked (PRD 011 §0a). Leaflet draws an array of arrays as exactly that.
    if (map_.track && map_.track.length) {
      L.polyline(map_.track, {
        color: '#1d4ed8',
        weight: 4,
        opacity: 0.85,
        // Rounded joins so a sharp turn does not render as a spike at this weight.
        lineJoin: 'round',
      }).addTo(map)

      map_.track.forEach(function (segment) {
        segment.forEach(function (point) {
          bounds.extend(point)
        })
      })
    }

    // Scan pins. Checkpoints and bandit catches get different colours, because they are different kinds of
    // thing that happened and a single marker would flatten the night's story.
    var scanLayer = L.layerGroup().addTo(map)
    ;(map_.scans || []).forEach(function (scan) {
      var bandit = scan.kind === 'bandit'
      L.circleMarker([scan.lat, scan.lng], {
        radius: 7,
        color: '#ffffff',
        weight: 2,
        fillColor: bandit ? '#b91c1c' : '#166534',
        fillOpacity: 1,
      })
        .bindPopup(escapeHTML(scan.label))
        .addTo(scanLayer)
      bounds.extend([scan.lat, scan.lng])
    })

    // Album photographs, clustered. A post that fifty photographs were taken at must read as one marker
    // with a count, not as a blot (PRD 011 §6). leaflet.markercluster supplies the count and the
    // spiderfy-on-click; where the plugin did not load the markers still draw, unclustered — the page
    // degrading rather than failing, same as everything else here.
    var photos = (albums && albums.photos) || []
    if (photos.length) {
      var clusterable = typeof L.markerClusterGroup === 'function'
      var photoLayer = clusterable
        ? L.markerClusterGroup({
            // A cluster of photographs taken along a road should not swallow the road: a modest radius
            // keeps separate posts separate at the zoom the route is viewed at.
            maxClusterRadius: 40,
            // Off, because the coverage polygon on hover reads as a route to somebody looking at a map
            // that also has an actual route drawn on it.
            showCoverageOnHover: false,
          })
        : L.layerGroup()

      photos.forEach(function (photo) {
        // An L.marker with a divIcon rather than a circleMarker: leaflet.markercluster is built around
        // L.Marker, and a square amber pin also reads as a different *kind* of thing from the round scan
        // dots, which is what the drawing rules ask for.
        var marker = L.marker([photo.lat, photo.lng], {
          icon: L.divIcon({
            className: 'photopin',
            html: '',
            iconSize: [14, 14],
            iconAnchor: [7, 7],
          }),
          keyboard: false,
        })
        // The thumbnail in the popup, so a marker is worth tapping. `loading="lazy"` because a cluster
        // expanded over a post could open a dozen at once.
        marker.bindPopup(
          '<a href="/offentligt/album/' +
            encodeURIComponent(photo.slug) +
            '">' +
            '<img src="/api/public/albums/' +
            encodeURIComponent(photo.album) +
            '/media/' +
            encodeURIComponent(photo.ordinal) +
            '?variant=thumb" alt="" loading="lazy" style="width:11rem;height:auto;display:block">' +
            '</a>',
        )
        photoLayer.addLayer(marker)
      })
      photoLayer.addTo(map)
      // Photographs deliberately do **not** extend the bounds. The map is the patrol's route; a
      // photograph taken at the start area should not zoom the view out to include it.
    }

    if (bounds.isValid()) {
      map.fitBounds(bounds, { padding: [24, 24] })
    } else {
      // Nothing to fit: the route was empty and no scan had a position. The server does not render a
      // container in that case, so this is belt and braces rather than an expected path.
      map.setView([55.6, 11.85], 8)
    }
  }

  // escapeHTML keeps a post's name from becoming markup.
  //
  // The labels come from the organizers' own data rather than from participants, so this is not the most
  // exposed input in the service — but it is being concatenated into a popup on an unauthenticated page,
  // and `html/template` is not here to do it for us.
  function escapeHTML(value) {
    return String(value == null ? '' : value).replace(/[&<>"']/g, function (ch) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]
    })
  }
})()
