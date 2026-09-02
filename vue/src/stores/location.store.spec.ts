import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  GEO_STUCK_MS,
  classifyGeoError,
  geoFailureMessage,
  useLocationStore,
  type GeolocationLike,
} from '@/stores/location.store'

// The geolocation cases that matter cannot be produced on the machine running the tests — a call that
// never answers, or a device with Location Services switched off — so the API is injected. Task 197 exists
// because all of them looked identical on a real iPad: like nothing at all.

const position = {
  coords: { latitude: 55.9, longitude: 12.2, accuracy: 12 },
  timestamp: 1_000,
} as GeolocationPosition

/** A geolocation that answers with the given error code. Plain object: a real browser error carries the
 *  code constants on the instance, and stubbing those adds nothing a spec-fixed number does not. */
function failingGeo(code: number): GeolocationLike {
  return {
    getCurrentPosition: (_ok, err) =>
      err?.({ code, message: 'nope' } as GeolocationPositionError),
    watchPosition: () => 1,
    clearWatch: () => {},
  }
}

/** The iPad case: neither callback is ever called. */
const silentGeo: GeolocationLike = {
  getCurrentPosition: () => {},
  watchPosition: () => 1,
  clearWatch: () => {},
}

const workingGeo: GeolocationLike = {
  getCurrentPosition: (ok) => ok(position),
  watchPosition: () => 1,
  clearWatch: () => {},
}

// `node` environment, so there is no localStorage. The store guards every access (private mode throws),
// but the remembered-grant behaviour is worth asserting, so a minimal stand-in is installed here rather
// than switching the whole suite to jsdom for one key.
const store: Record<string, string> = {}
vi.stubGlobal('localStorage', {
  getItem: (k: string) => (k in store ? store[k] : null),
  setItem: (k: string, v: string) => {
    store[k] = v
  },
  removeItem: (k: string) => {
    delete store[k]
  },
  clear: () => {
    for (const k of Object.keys(store)) delete store[k]
  },
})

beforeEach(() => {
  setActivePinia(createPinia())
  localStorage.clear()
})

afterEach(() => {
  vi.useRealTimers()
})

describe('classifyGeoError', () => {
  it('maps the spec codes to causes', () => {
    expect(classifyGeoError({ code: 1 })).toBe('denied')
    expect(classifyGeoError({ code: 2 })).toBe('unavailable')
    expect(classifyGeoError({ code: 3 })).toBe('timeout')
  })

  // An unrecognised failure must not be reported as a denial: that would send the user to a Settings
  // screen to undo a decision they never made.
  it('treats an unknown code as unavailable rather than denied', () => {
    expect(classifyGeoError({})).toBe('unavailable')
    expect(classifyGeoError({ code: 99 })).toBe('unavailable')
  })
})

describe('geoFailureMessage', () => {
  it('says something different, and Danish, for every cause', () => {
    const messages = (['denied', 'unavailable', 'timeout', 'stuck'] as const).map(geoFailureMessage)

    expect(new Set(messages).size).toBe(4)
    for (const message of messages) expect(message.length).toBeGreaterThan(20)
  })

  // The cause nobody guesses, and the one the iPad most likely hit: the setting is off for the whole
  // device, not for this app. A generic apology sends them looking in the wrong place.
  it('points at Stedtjenester when the device would not answer', () => {
    expect(geoFailureMessage('unavailable')).toMatch(/Stedtjenester/)
  })

  it('says nothing when there is no failure', () => {
    expect(geoFailureMessage(null)).toBe('')
  })
})

describe('request', () => {
  it('records a position and a grant on success', async () => {
    const store = useLocationStore()
    store.geo = workingGeo

    const coords = await store.request()

    expect(coords).toEqual({ lat: 55.9, lng: 12.2, accuracy: 12 })
    expect(store.permission).toBe('granted')
    expect(store.failure).toBeNull()
    expect(store.requesting).toBe(false)
  })

  it('marks a refusal as denied', async () => {
    const store = useLocationStore()
    store.geo = failingGeo(1)

    await store.request()

    expect(store.permission).toBe('denied')
    expect(store.failure).toBe('denied')
  })

  // The bug behind task 197: before it, only PERMISSION_DENIED changed anything, so this case left the
  // prompt sitting there looking untouched — no state, no message, nothing.
  it('reports an unavailable position without calling it a denial', async () => {
    const store = useLocationStore()
    store.geo = failingGeo(2)

    await store.request()

    expect(store.failure).toBe('unavailable')
    expect(store.permission).not.toBe('denied')
    // And the remembered grant is untouched: the device did not refuse us, it just could not answer.
    expect(localStorage.getItem('hej.geo-granted')).toBeNull()
  })

  it('reports a timeout as its own cause', async () => {
    const store = useLocationStore()
    store.geo = failingGeo(3)

    await store.request()

    expect(store.failure).toBe('timeout')
  })

  // The iPad. Neither callback ever fires, and the Geolocation API's own `timeout` option does not cover
  // waiting for the permission dialog — so without our own clock the promise never settles and the button
  // is indistinguishable from a dead one, for ever.
  it('gives up on a call that never answers', async () => {
    vi.useFakeTimers()
    const store = useLocationStore()
    store.geo = silentGeo

    const pending = store.request()
    expect(store.requesting).toBe(true)

    vi.advanceTimersByTime(GEO_STUCK_MS)
    const coords = await pending

    expect(coords).toBeNull()
    expect(store.failure).toBe('stuck')
    expect(store.requesting).toBe(false)
  })

  it('waits long enough for someone to read an iOS dialog before giving up', () => {
    // Cutting the wait short would report a failure to a user who is about to grant permission.
    expect(GEO_STUCK_MS).toBeGreaterThan(10_000)
  })

  it('flags that a request is in flight, so a tap is never silent', () => {
    const store = useLocationStore()
    store.geo = silentGeo

    void store.request()

    expect(store.requesting).toBe(true)
  })

  it('reports unavailable when there is no geolocation at all', async () => {
    const store = useLocationStore()
    store.geo = null

    expect(await store.request()).toBeNull()
    expect(store.permission).toBe('unavailable')
    expect(store.failure).toBe('unavailable')
  })

  it('clears a previous failure when a later attempt succeeds', async () => {
    const store = useLocationStore()
    store.geo = failingGeo(3)
    await store.request()
    expect(store.failure).toBe('timeout')

    store.geo = workingGeo
    await store.request()

    expect(store.failure).toBeNull()
    expect(store.error).toBe('')
  })
})
