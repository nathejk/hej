import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { forcedOffline, initForcedOffline, setForcedOffline } from '@/dev/forcedOffline'
import { fetchWrapper, NetworkError } from '@/helpers/fetchWrapper'
import { useAppStore } from '@/stores/app.store'

describe('forced offline', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    initForcedOffline()
    setForcedOffline(false)
  })

  it('drives app.store.online, which is a supported input rather than private state', () => {
    const app = useAppStore()
    setForcedOffline(true)
    expect(app.online).toBe(false)
  })

  // The half that catches real bugs: a component rendering an offline banner while still
  // awaiting a fetch is only distinguishable when requests actually fail.
  it('makes requests fail with the same NetworkError the real path throws', async () => {
    setForcedOffline(true)
    await expect(fetchWrapper.get('/api/config')).rejects.toBeInstanceOf(NetworkError)
  })

  it('lets requests through again when switched off', async () => {
    // A stub server, because the interesting distinction is "did the blocker release" — and with
    // no stub, a real fetch in node fails with the *same* NetworkError, so the assertion would
    // pass while the blocker was still on. (It did, on the first attempt.)
    const original = globalThis.fetch
    globalThis.fetch = (async () => new Response('{}', { status: 200 })) as typeof fetch

    setForcedOffline(true)
    setForcedOffline(false)
    await expect(fetchWrapper.get('/api/config')).resolves.toEqual({})

    globalThis.fetch = original
  })

  // Claiming "online" on un-force would be the same lie in the other direction: the machine may
  // genuinely have no network.
  it('restores connectivity from the browser, not by assuming true', () => {
    const app = useAppStore()
    Object.defineProperty(globalThis.navigator, 'onLine', { value: false, configurable: true })
    setForcedOffline(true)
    setForcedOffline(false)
    expect(app.online).toBe(false)
    Object.defineProperty(globalThis.navigator, 'onLine', { value: true, configurable: true })
  })

  it('exposes its state so the panel can say it is on', () => {
    setForcedOffline(true)
    expect(forcedOffline.value).toBe(true)
  })
})
