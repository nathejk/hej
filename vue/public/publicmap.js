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
// # The duplication, removed rather than named (task 353)
//
// This file used to carry a hand-copied WMS URL, with a comment admitting the duplication and saying the
// judgement should be revisited "if a second layer ever appears here". It should have been revisited sooner:
// the copy had drifted to a **different Dataforsyningen service** from any of the app's three, so the public
// map showed a base map no participant had ever seen.
//
// The layers, the zoom limits and the tile-retry policy now come from `/maplayers.json`, which
// `src/config/map.ts` imports at build time and this file fetches at runtime. One source, no code generation.
// If that fetch fails the map still draws, on the default layer, with the switcher missing — degrading the
// same way everything else here does.

(function () {
  'use strict'

  var container = document.getElementById('patrolmap')
  if (!container || typeof window.L === 'undefined') {
    // No container means the server decided there was no route to draw. No Leaflet means an old browser
    // or a blocked asset. Both are fine and both leave the page exactly as it was.
    return
  }

  // Duplicated from src/config/map.ts — REMOVED, see the header. Kept only as the last-resort fallback for a
  // failed fetch of the shared file, and deliberately the *same* layer the app opens with rather than a
  // different service — the mistake this change exists to correct.
  var FALLBACK_MAP_CONFIG = {
    attribution:
      '&copy; <a target="_blank" rel="noopener" href="https://dataforsyningen.dk/">Styrelsen for Dataforsyning og Infrastruktur</a>',
    default: 'dtk25',
    minZoom: 7,
    maxZoom: 19,
    retry: { limit: 3, baseDelayMs: 400, jitterMs: 250 },
    layers: {
      dtk25: {
        label: 'Topografisk 1:25.000',
        url: 'https://api.dataforsyningen.dk/dtk_25_DAF',
        layer: 'dtk25',
        format: 'image/png',
      },
    },
  }

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
        // Same treatment for the layer definitions: losing them costs the switcher, not the map.
        fetchJSON('/maplayers.json').catch(function () {
          return FALLBACK_MAP_CONFIG
        }),
      ])
    })
    .then(function (results) {
      draw(results[0], results[1], results[2], results[3])
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

  function draw(map_, albums, config, mapConfig) {
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
      minZoom: mapConfig.minZoom,
      maxZoom: mapConfig.maxZoom,
    })

    addBaseLayers(map, mapConfig, config.dataforsyningen_token || '')

    // A scale bar, bottom left. The page states a distance in kilometres; without a scale the map next to it
    // is unitless, and "how far is that gap" has no answer. Metric only — the event is in Denmark, and an
    // imperial row would just be a second number to read past. Bottom left is the one corner with nothing in
    // it: zoom sits top left, the layer switcher top right, the attribution bottom right.
    L.control.scale({ position: 'bottomleft', metric: true, imperial: false, maxWidth: 120 }).addTo(map)

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

    // **Where there is no recording: a dotted line between the two registrations either side.**
    //
    // The commonest case by far — task 082 measured 2% track coverage — used to draw as a handful of
    // unconnected pins, which reads as missing data rather than as travel we did not measure. A dotted line
    // says the true thing instead: the patrol was registered here and then there, and we do not know the way
    // between.
    //
    // Same blue as the route, because it is the same patrol going the same way; dotted and thinner, because
    // it is a weaker claim. That contrast is the whole point — solid means "we recorded this", dotted means
    // "we are joining two things we know" — so if these ever start looking alike, the honesty goes with it.
    if (map_.untracked && map_.untracked.length) {
      L.polyline(map_.untracked, {
        color: '#1d4ed8',
        weight: 3,
        opacity: 0.55,
        // Dots rather than dashes: a dashed line still reads as a route, and this is not one. Round caps
        // turn each 1 px segment into a dot.
        dashArray: '1 7',
        lineCap: 'round',
      }).addTo(map)

      map_.untracked.forEach(function (leg) {
        leg.forEach(function (point) {
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

  // addBaseLayers builds the app's base layers and puts a switcher on the map.
  //
  // # Why all of them and not just the default
  //
  // The maintainer's instruction (2026-09-21): the public map should have the same layers as the app's. A
  // family looking at a patrol's route wants the aerial photograph for the same reason a patrol did — to see
  // the field they walked across — and the 1:50.000 sheet is what some of them will have had on paper.
  //
  // Leaflet's own `L.control.layers` does the switching, so this costs no code of ours. The choice is
  // deliberately **not persisted**: the app stores it under `hej.map.baseLayer` for a member, and this page is
  // read by people who are not members. Sharing that key would let a stranger's browsing change what a
  // member's app opens with, and the island holds no state by design (see the header).
  function addBaseLayers(map, mapConfig, token) {
    var L = window.L
    var switcher = {}
    var chosen = null
    var first = null

    Object.keys(mapConfig.layers).forEach(function (key) {
      var cfg = mapConfig.layers[key]
      // Every option here matches `wmsLayerOptions` in src/config/map.ts, including the two that look
      // omissible and are not:
      //
      //   - no `version`: Leaflet emits lowercase parameter values, and WMS 1.3.0 makes this service answer
      //     `ServiceException: TRANSPARENT must be either TRUE or FALSE` on *every* tile — as a 200
      //     containing XML, not an HTTP error, so it would fail as blank tiles rather than as an error.
      //   - `crossOrigin`: keeps tiles CORS-readable. The app needs it so its service worker can store them
      //     as ordinary rather than opaque responses; here it costs nothing and keeps the URLs identical,
      //     which is what lets one HTTP cache serve both surfaces.
      var layer = L.tileLayer.wms(cfg.url, {
        layers: cfg.layer,
        format: cfg.format,
        crossOrigin: 'anonymous',
        transparent: false,
        attribution: mapConfig.attribution,
        token: token,
        maxZoom: mapConfig.maxZoom,
      })
      attachTileRetry(layer, mapConfig.retry)

      var label = cfg.note ? cfg.label + ' (' + cfg.note + ')' : cfg.label
      switcher[label] = layer

      if (!first) first = layer
      if (key === mapConfig.default) chosen = layer
    })

    // A `default` naming a layer the file does not contain would otherwise leave the map with no tiles at
    // all — the one failure here that reads as a broken page rather than as a missing nicety.
    var base = chosen || first
    if (base) base.addTo(map)

    if (Object.keys(switcher).length > 1) {
      L.control.layers(switcher, {}, { position: 'topright', collapsed: true }).addTo(map)
    }
  }

  // attachTileRetry re-requests a tile that failed, with backoff.
  //
  // # Why this is here at all
  //
  // Leaflet has no built-in retry: one failed image request leaves that tile **grey until the visitor pans
  // away and back**. The app carries the same mechanism (EventMap.vue) because on patchy rural mobile data a
  // failed tile is the normal case — and the morning after the event this page is opened by a hundred people
  // at once on whatever connection they have, which is the same problem from the other end.
  //
  // # Ported faithfully, including the parts that look incidental
  //
  //   - **The same `<img>` is reused**, with its `src` re-assigned. Leaflet's own load/error handlers stay
  //     attached, so a late success still marks the tile loaded and fades it in normally. A fresh image would
  //     leave Leaflet believing the tile never arrived.
  //   - **`_retry=N` busts any negative caching** of the failed response by the browser or an intermediary.
  //     The app's service worker strips this parameter from its cache key (`TILE_CACHE_KEY_IGNORED_PARAMS`),
  //     so a retried tile still matches the cached one — irrelevant on this page, which has no worker, and
  //     kept identical so both surfaces produce the same URLs.
  //   - **Exponential backoff with jitter**, so a whole screen of failed tiles does not retry in lockstep and
  //     hammer a service that is already struggling.
  //   - **A disconnected tile is dropped**: panning or a layer switch discards tiles, and re-assigning `src`
  //     on one Leaflet has thrown away would fetch bytes nobody will see.
  //
  // No timer bookkeeping, unlike the app's version: that cancels pending retries when the component unmounts,
  // and this map lives until the page is navigated away from — which cancels everything anyway.
  function attachTileRetry(layer, retry) {
    var limit = retry && retry.limit ? retry.limit : 0
    if (limit < 1) return

    layer.on('tileerror', function (event) {
      var tile = event && event.tile
      if (!tile) return

      var attempt = (tile._hejRetries || 0) + 1
      if (attempt > limit) return
      tile._hejRetries = attempt

      var delay = retry.baseDelayMs * Math.pow(2, attempt - 1) + Math.random() * (retry.jitterMs || 0)
      var original = tile.src.replace(/&_retry=\d+$/, '')

      window.setTimeout(function () {
        if (!tile.isConnected) return
        tile.src = original + '&_retry=' + attempt
      }, delay)
    })
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
