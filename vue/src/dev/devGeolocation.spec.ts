import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import {
  devGeoAccuracy,
  devGeolocationBridge,
  setDevGeoEnabled,
  setDevGeoFailure,
  stopDevWatches,
} from '@/dev/devGeolocation'
import { pausePlayback, initPlayback, resetPlayback, startPlayback } from '@/dev/devPlayback'

// A stand-in for the device's own geolocation, so the bridge's delegation can be observed.
function realStub() {
  return {
    getCurrentPosition: vi.fn(),
    watchPosition: vi.fn(() => 9001),
    clearWatch: vi.fn(),
  }
}

let real: ReturnType<typeof realStub>

beforeEach(() => {
  // Needed because toggling the fake also corrects `location.store`'s permission state — otherwise
  // a laptop that has denied location would show "location off" while simulated positions arrived.
  setActivePinia(createPinia())
  vi.useFakeTimers()
  real = realStub()
  Object.defineProperty(globalThis.navigator, 'geolocation', {
    value: real,
    configurable: true,
  })
  setDevGeoEnabled(false)
  setDevGeoFailure('none')
  // What `@/dev/bootstrap` does in the app: registers the walk as the fake's position source.
  // Without it the fake reports the fixed coordinate and never moves — which is exactly what the
  // first run of the playback test caught.
  initPlayback()
  resetPlayback()
})

afterEach(() => {
  stopDevWatches()
  vi.useRealTimers()
})

describe('the dev geolocation bridge', () => {
  // The reason it is a bridge and not "the fake when enabled": location.store resolves its source
  // **once**, into state, when the store is created. A provider that returned null while switched
  // off would be captured as the real device, and the panel's toggle would do nothing until a
  // reload.
  it('delegates to the device while the fake is off', () => {
    devGeolocationBridge()!.getCurrentPosition(vi.fn())
    expect(real.getCurrentPosition).toHaveBeenCalled()
  })

  it('answers from the fake once switched on, without being re-resolved', () => {
    const bridge = devGeolocationBridge()!
    setDevGeoEnabled(true)

    const success = vi.fn()
    bridge.getCurrentPosition(success)
    vi.runAllTimers()

    expect(real.getCurrentPosition).not.toHaveBeenCalled()
    expect(success).toHaveBeenCalledOnce()
    expect(success.mock.calls[0][0].coords.accuracy).toBe(devGeoAccuracy.value)
  })

  // Ownership is a property of the id, not of the toggle's current position. Routing by the toggle
  // would hand a fake id to the real API (leaking our interval) or a real id to ours (leaving the
  // device's watch running, i.e. the radio on).
  it('clears a watch by which source issued it, even after the toggle flips', () => {
    const bridge = devGeolocationBridge()!
    setDevGeoEnabled(true)
    const fakeId = bridge.watchPosition(vi.fn())

    setDevGeoEnabled(false)
    bridge.clearWatch(fakeId)
    expect(real.clearWatch).not.toHaveBeenCalled()

    const realId = bridge.watchPosition(vi.fn())
    bridge.clearWatch(realId)
    expect(real.clearWatch).toHaveBeenCalledWith(9001)
  })

  it('stops emitting when switched off mid-watch', () => {
    const bridge = devGeolocationBridge()!
    setDevGeoEnabled(true)
    const success = vi.fn()
    bridge.watchPosition(success)
    vi.advanceTimersByTime(2500)
    const before = success.mock.calls.length
    expect(before).toBeGreaterThan(1)

    setDevGeoEnabled(false)
    vi.advanceTimersByTime(5000)
    // Otherwise the interval keeps pushing simulated positions over the real ones and the app
    // looks like it has two devices.
    expect(success.mock.calls.length).toBe(before)
  })
})

describe('permission consistency', () => {
  // PRD 014 forbids the layer from producing a state where the app contradicts itself. A laptop
  // that has denied location for this origin would otherwise show "location off" while simulated
  // positions were arriving — and the store may not even call the source in that state.
  it('claims the permission the fake implies, through the store\u2019s own folding', async () => {
    const { useLocationStore } = await import('@/stores/location.store')
    const location = useLocationStore()
    location.applyPermissionState('denied')
    expect(location.permission).toBe('denied')

    setDevGeoEnabled(true)
    expect(location.permission).toBe('granted')
  })
})

describe('failure modes', () => {
  beforeEach(() => setDevGeoEnabled(true))

  it.each([
    ['denied', 1],
    ['unavailable', 2],
    ['timeout', 3],
  ] as const)('reports %s as code %i', (mode, code) => {
    setDevGeoFailure(mode)
    const fail = vi.fn()
    devGeolocationBridge()!.getCurrentPosition(vi.fn(), fail)
    vi.runAllTimers()
    // location.store reads the numeric code rather than the error instance's constants, precisely
    // so a stub like this works — see its comment.
    expect(fail.mock.calls[0][0].code).toBe(code)
  })

  // Distinct from `timeout` on purpose: a callback that never arrives is a different failure from
  // one that arrives saying "timed out", and only the latter happens by accident.
  it('hang never calls back at all', () => {
    setDevGeoFailure('hang')
    const success = vi.fn()
    const fail = vi.fn()
    devGeolocationBridge()!.getCurrentPosition(success, fail)
    vi.advanceTimersByTime(60_000)
    expect(success).not.toHaveBeenCalled()
    expect(fail).not.toHaveBeenCalled()
  })
})

describe('playback', () => {
  beforeEach(() => setDevGeoEnabled(true))

  it('moves the position over time, and pausing stops it', () => {
    const bridge = devGeolocationBridge()!
    const read = () => {
      const success = vi.fn()
      bridge.getCurrentPosition(success)
      vi.runAllTimers()
      const { latitude, longitude } = success.mock.calls[0][0].coords
      return `${latitude},${longitude}`
    }

    const first = read()
    startPlayback()
    vi.advanceTimersByTime(30_000)
    const moved = read()
    expect(moved).not.toBe(first)

    pausePlayback()
    const paused = read()
    vi.advanceTimersByTime(30_000)
    // Pausing must not silently advance the walk — progress is kept as distance travelled rather
    // than derived from a start timestamp for exactly this reason.
    expect(read()).toBe(paused)
  })
})
