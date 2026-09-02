import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useTrackStore } from '@/stores/track.store'
import { useLocationStore } from '@/stores/location.store'
import { useSessionStore } from '@/stores/session.store'

// The recorder's visibility rule (task 202).
//
// A 22-hour device report showed 48 `geoerror — code=3 Timeout expired`, one every 30 s in two runs of a
// quarter of an hour, while the app was hidden — and **zero points recorded** by any of them. iOS leaves the
// page alive enough to run timers and does not service its location requests, so every attempt spent 20 s
// asking for the most expensive fix available on a phone in a pocket, and wrote two entries into a capped
// log that is the only way problems like this get found.
//
// These tests are about not doing that. `document` is not available in the node environment, which is why
// visibility is injected on the store rather than read from a global.

const stored: Record<string, string> = {}
vi.stubGlobal('localStorage', {
  getItem: (k: string) => (k in stored ? stored[k] : null),
  setItem: (k: string, v: string) => {
    stored[k] = v
  },
  removeItem: (k: string) => {
    delete stored[k]
  },
})

beforeEach(() => {
  setActivePinia(createPinia())
  useSessionStore().user = { userId: 'u-1', role: 'gøgler' }
  useLocationStore().permission = 'granted'
})

/** A recorder that is running, with `record()` replaced so nothing touches IndexedDB. */
function recorder(hidden: boolean) {
  const track = useTrackStore()
  let records = 0

  track.recording = true
  track.isHidden = () => hidden
  track.record = async () => {
    records++
  }

  return { track, records: () => records }
}

describe('sampling and visibility', () => {
  it('does not ask for a fix while the document is hidden', async () => {
    const { track, records } = recorder(true)

    await track.sample()

    expect(records()).toBe(0)
  })

  it('asks for a fix while the document is visible', async () => {
    const { track, records } = recorder(false)

    await track.sample()

    expect(records()).toBe(1)
  })

  // The flooding half. Skipping is right; announcing the skip 30 times a run is what filled the buffer.
  it('records the hidden state once per run, not once per attempt', async () => {
    const { track } = recorder(true)

    await track.sample()
    expect(track.skippedWhileHidden).toBe(true)

    // Subsequent attempts while still hidden must not log again — the flag is the guard, and it staying
    // true across attempts is what this asserts.
    await track.sample()
    await track.sample()
    expect(track.skippedWhileHidden).toBe(true)
  })

  // A later backgrounding is worth recording again: the useful information is "it went quiet here", not
  // "it is still quiet".
  it('logs afresh after coming back into view', async () => {
    const track = useTrackStore()
    track.recording = true
    track.record = async () => {}

    track.isHidden = () => true
    await track.sample()
    expect(track.skippedWhileHidden).toBe(true)

    track.isHidden = () => false
    await track.sample()
    expect(track.skippedWhileHidden).toBe(false)

    track.isHidden = () => true
    await track.sample()
    expect(track.skippedWhileHidden).toBe(true)
  })

  // The visibility check must not become a way to keep recording after permission is revoked or the user
  // signs out — those stop the recorder outright, and that ordering matters more than the skip.
  it('still stops when permission is gone, even while hidden', async () => {
    const { track } = recorder(true)
    useLocationStore().permission = 'denied'

    await track.sample()

    expect(track.recording).toBe(false)
  })
})
