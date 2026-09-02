import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  GEO_COARSE,
  GEO_COARSE_AFTER_MS,
  GEO_PRECISE,
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
  // Every strategy fails, because the race only gives up when they all have — a single failure says
  // nothing about the calls still in flight (task 199).
  return {
    getCurrentPosition: (_ok, err) => err?.({ code, message: 'nope' } as GeolocationPositionError),
    watchPosition: (_ok, err) => {
      err?.({ code, message: 'nope' } as GeolocationPositionError)
      return 1
    },
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

  // Task 200. Reinstalling is what actually recovered the wedged iPad, and it is deliberately NOT
  // suggested here: it clears the app's storage, including a position track that has not been uploaded —
  // the one thing on the device that exists nowhere else. A participant cannot know that, so the advice
  // they get is the safe half, and the destructive half is documented for organisers.
  it('never tells a participant to delete the app', () => {
    for (const failure of ['denied', 'unavailable', 'timeout', 'stuck'] as const) {
      const message = geoFailureMessage(failure).toLowerCase()
      for (const word of ['slet', 'afinstal', 'geninstal', 'fjern app']) {
        expect(message, `${failure} suggests "${word}"`).not.toContain(word)
      }
    }
  })

  it('offers the two safe steps when nothing answered', () => {
    const message = geoFailureMessage('stuck')
    expect(message).toMatch(/Stedtjenester/)
    // Closing and reopening is free and sometimes enough.
    expect(message).toMatch(/luk appen/i)
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

describe('the coarse fallback', () => {
  // A Wi-Fi-only iPad has no GNSS receiver at all, so the first request asks for something the hardware
  // cannot provide. Retrying coarse asks for what it can — and ±500 m still answers "which end of the
  // forest is this patrol in", which is the question that matters at 03:00.
  it('retries coarse when high accuracy cannot be satisfied', async () => {
    const attempts: (PositionOptions | undefined)[] = []
    const store = useLocationStore()
    store.geo = {
      getCurrentPosition: (ok, err, options) => {
        attempts.push(options)
        if (options?.enableHighAccuracy) {
          err?.({ code: 2, message: 'kCLErrorLocationUnknown' } as GeolocationPositionError)
        } else {
          ok({
            coords: { latitude: 55.9, longitude: 12.2, accuracy: 480 },
            timestamp: 2_000,
          } as GeolocationPosition)
        }
      },
      watchPosition: (_ok, err) => {
        err?.({ code: 2, message: 'kCLErrorLocationUnknown' } as GeolocationPositionError)
        return 1
      },
      clearWatch: () => {},
    }

    const coords = await store.request()

    expect(attempts).toHaveLength(2)
    expect(attempts[0]?.enableHighAccuracy).toBe(true)
    expect(attempts[1]?.enableHighAccuracy).toBe(false)

    // A coarse fix is a normal success, not a degraded state: the map's accuracy circle draws the truth.
    expect(coords?.accuracy).toBe(480)
    expect(store.permission).toBe('granted')
    expect(store.failure).toBeNull()
    expect(store.coarse).toBe(true)
  })

  it('does not start the coarse attempt when the precise one succeeds', async () => {
    let calls = 0
    const store = useLocationStore()
    store.geo = {
      getCurrentPosition: (ok) => {
        calls++
        ok(position)
      },
      watchPosition: () => 1,
      clearWatch: () => {},
    }

    await store.request()

    expect(calls).toBe(1)
    expect(store.coarse).toBe(false)
    expect(store.strategy).toBe('precise')
  })

  // Retrying a refusal cannot change the answer — that needs a trip to Settings — and on some platforms
  // repeat requests are exactly what gets a permission permanently blocked.
  it('never retries a refusal', async () => {
    let calls = 0
    const store = useLocationStore()
    store.geo = {
      getCurrentPosition: (_ok, err) => {
        calls++
        err?.({ code: 1, message: 'denied' } as GeolocationPositionError)
      },
      watchPosition: () => 1,
      clearWatch: () => {},
    }

    await store.request()

    // One attempt, and no coarse follow-up: the answer cannot change without a trip to Settings, and on
    // some platforms continuing to ask is what gets a permission permanently blocked.
    expect(calls).toBe(1)
    expect(store.failure).toBe('denied')
  })

  it('reports the last thing tried when every attempt fails', async () => {
    const store = useLocationStore()
    store.geo = {
      getCurrentPosition: (_ok, err, options) =>
        err?.({
          code: options?.enableHighAccuracy ? 2 : 3,
          message: 'still nothing',
        } as GeolocationPositionError),
      watchPosition: (_ok, err) => {
        err?.({ code: 2, message: 'still nothing' } as GeolocationPositionError)
        return 1
      },
      clearWatch: () => {},
    }

    await store.request()

    // The coarse attempt's cause, not the precise one's: what the user is told should describe the last
    // thing actually attempted.
    expect(store.failure).toBe('timeout')
  })

  it('gives the coarse attempt room to work', () => {
    // Both relaxations matter: a Wi-Fi database lookup needs the time, and accepting an older fix is what
    // makes a cached position usable rather than demanding a fresh lookup that may never succeed.
    expect(GEO_COARSE.timeout ?? 0).toBeGreaterThan(GEO_PRECISE.timeout ?? 0)
    expect(GEO_COARSE.maximumAge ?? 0).toBeGreaterThan(GEO_PRECISE.maximumAge ?? 0)
    expect(GEO_COARSE.enableHighAccuracy).toBe(false)
  })
})

describe('the strategy race', () => {
  // The iPad, exactly: `getCurrentPosition` never calls either callback, while `watchPosition` — a
  // different WebKit code path — delivers. Task 198's coarse retry hung off the one-shot's error handler,
  // so on this device it could never run: a fallback chained to a failure cannot rescue a call that does
  // not fail. This is the test that would have caught that.
  it('is answered by the watch when the one-shot never calls back', async () => {
    const store = useLocationStore()
    store.geo = {
      getCurrentPosition: () => {},
      watchPosition: (ok) => {
        ok(position)
        return 7
      },
      clearWatch: () => {},
    }

    const coords = await store.request()

    expect(coords).not.toBeNull()
    expect(store.strategy).toBe('watch')
    expect(store.permission).toBe('granted')
    expect(store.failure).toBeNull()
  })

  // When the watch wins it is exactly the subscription the map is about to need, so tearing it down and
  // starting another would throw away the one thing on that device that answered.
  it('keeps the winning watch instead of clearing it', async () => {
    let cleared: number | null = null
    const store = useLocationStore()
    store.geo = {
      getCurrentPosition: () => {},
      watchPosition: (ok) => {
        ok(position)
        return 7
      },
      clearWatch: (id) => {
        cleared = id
      },
    }

    await store.request()

    expect(cleared).toBeNull()
    expect(store.watchId).toBe(7)
  })

  // Better than starting one and tearing it down: if the one-shot has already answered, there is nothing
  // for a watch to add and every reason not to open a high-accuracy subscription on a phone.
  it('does not start a watch at all when the one-shot has already answered', async () => {
    let watches = 0
    const store = useLocationStore()
    store.geo = {
      getCurrentPosition: (ok) => ok(position),
      watchPosition: () => {
        watches++
        return 7
      },
      clearWatch: () => {},
    }

    await store.request()

    expect(store.strategy).toBe('precise')
    expect(watches).toBe(0)
  })

  // And when it *was* started — the normal case, where the one-shot answers a moment later — it must be
  // cleared. A watch left running is a live high-accuracy subscription nobody asked for, on a battery that
  // has to last the night.
  it('clears a started watch when something else wins', async () => {
    let cleared: number | null = null
    const store = useLocationStore()
    store.geo = {
      // Asynchronous, like a real browser, so the watch is started before this answers.
      getCurrentPosition: (ok) => {
        setTimeout(() => ok(position), 0)
      },
      watchPosition: () => 7,
      clearWatch: (id) => {
        cleared = id
      },
    }

    await store.request()

    expect(store.strategy).toBe('precise')
    expect(cleared).toBe(7)
  })

  // The regression this design nearly introduced: ignoring non-denied errors so the race can continue is
  // right, but if every strategy fails *fast* the user must be told immediately rather than sitting
  // through the 25 s guard. Task 197 exists precisely because a silent wait reads as a dead button.
  it('gives up as soon as every strategy has failed, without waiting out the guard', async () => {
    vi.useFakeTimers()
    const store = useLocationStore()
    store.geo = failingGeo(2)

    const pending = store.request()
    // No timer advanced at all: the coarse attempt is pulled forward because nothing else can answer.
    await pending

    expect(store.failure).toBe('unavailable')
    expect(store.requesting).toBe(false)
  })

  it('reports stuck only when nothing answers at all', async () => {
    vi.useFakeTimers()
    const store = useLocationStore()
    store.geo = {
      getCurrentPosition: () => {},
      watchPosition: () => 1,
      clearWatch: () => {},
    }

    const pending = store.request()
    vi.advanceTimersByTime(GEO_STUCK_MS)

    expect(await pending).toBeNull()
    expect(store.failure).toBe('stuck')
  })

  it('survives a browser whose watchPosition throws', async () => {
    const store = useLocationStore()
    store.geo = {
      getCurrentPosition: (ok) => ok(position),
      watchPosition: () => {
        throw new Error('no watch here')
      },
      clearWatch: () => {},
    }

    expect(await store.request()).not.toBeNull()
    expect(store.strategy).toBe('precise')
  })

  it('starts the coarse attempt on a delay rather than immediately', () => {
    // A device that can do better should be given the chance to, so the coarse attempt is a fallback in
    // time as well as in accuracy — but not gated on the precise attempt failing.
    expect(GEO_COARSE_AFTER_MS).toBeGreaterThan(0)
    expect(GEO_COARSE_AFTER_MS).toBeLessThan(GEO_STUCK_MS)
  })
})
