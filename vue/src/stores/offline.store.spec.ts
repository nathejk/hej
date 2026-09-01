import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import { OFFLINE_DATASETS } from '@/config/offline'
import { useOfflineStore } from '@/stores/offline.store'

beforeEach(() => {
  setActivePinia(createPinia())
})

// Everything a dataset needs to report to count as fully present.
const present = { state: 'synced' as const, complete: true, syncedAt: 1_000, bytes: 10 }

function reportAll(store: ReturnType<typeof useOfflineStore>) {
  for (const dataset of OFFLINE_DATASETS) store.report(dataset.id, present)
}

describe('initial state', () => {
  // The distinction this store exists to keep: nothing reported is not the same as nothing
  // stored. Getting it wrong is how a loading app looks like an empty one (task 090).
  it('starts every dataset as unknown, not empty', () => {
    const store = useOfflineStore()
    for (const dataset of OFFLINE_DATASETS) {
      expect(store.statuses[dataset.id].state, dataset.id).toBe('unknown')
    }
    expect(store.hydrated).toBe(false)
  })

  // A readiness answer that defaults to yes before anything has reported is the one a user acts
  // on before walking into a forest at night.
  it('is not ready before anything has reported', () => {
    expect(useOfflineStore().ready).toBe(false)
  })

  it('knows nothing about storage until asked', () => {
    const store = useOfflineStore()
    expect(store.usageBytes).toBeNull()
    expect(store.persisted).toBeNull()
    expect(store.headroomBytes()).toBeNull()
  })
})

describe('reporting', () => {
  it('merges partial reports rather than replacing them', () => {
    const store = useOfflineStore()
    store.report('tiles', { state: 'synced', syncedAt: 5, complete: true })
    store.report('tiles', { bytes: 1234 })

    expect(store.statuses.tiles.syncedAt).toBe(5)
    expect(store.statuses.tiles.bytes).toBe(1234)
    expect(store.statuses.tiles.state).toBe('synced')
  })

  it('is ready once every dataset is stored and complete', () => {
    const store = useOfflineStore()
    reportAll(store)
    expect(store.ready).toBe(true)
    expect(store.missing).toEqual([])
  })

  // Stored, current, and *incomplete* — a half-fetched tile set. Reporting that as ready is the
  // dishonesty PRD 009 exists to prevent.
  it('is not ready when a dataset is stored but incomplete', () => {
    const store = useOfflineStore()
    reportAll(store)
    store.report('tiles', { complete: false })

    expect(store.ready).toBe(false)
    expect(store.missing).toContain('tiles')
  })

  it('counts stale data as ready, since it is still usable offline', () => {
    const store = useOfflineStore()
    reportAll(store)
    store.report('directory', { state: 'stale' })
    expect(store.ready).toBe(true)
  })

  it('sums reported bytes for the readiness total', () => {
    const store = useOfflineStore()
    reportAll(store)
    expect(store.reportedBytes).toBe(10 * OFFLINE_DATASETS.length)
  })

  it('reports syncing while any dataset is fetching', () => {
    const store = useOfflineStore()
    expect(store.syncing).toBe(false)
    store.report('portraits', { state: 'syncing' })
    expect(store.syncing).toBe(true)
  })
})

describe('eviction and clearing', () => {
  // "You had this an hour ago and the phone removed it" is a different sentence from "you never
  // had this", and on iOS it is the one we have to be able to say.
  it('keeps the last-synced time when a dataset is evicted', () => {
    const store = useOfflineStore()
    store.report('portraits', present)
    store.markEvicted('portraits')

    expect(store.statuses.portraits.state).toBe('evicted')
    expect(store.statuses.portraits.syncedAt).toBe(1_000)
    expect(store.statuses.portraits.complete).toBe(false)
    expect(store.evicted).toEqual(['portraits'])
  })

  it('does not list the same eviction twice', () => {
    const store = useOfflineStore()
    store.markEvicted('tiles')
    store.markEvicted('tiles')
    expect(store.evicted).toEqual(['tiles'])
  })

  it('counts an evicted dataset as missing, so the view can explain it', () => {
    const store = useOfflineStore()
    reportAll(store)
    store.markEvicted('tiles')

    expect(store.ready).toBe(false)
    expect(store.missing).toContain('tiles')
  })

  it('keeps the last-synced time after a clear too', () => {
    const store = useOfflineStore()
    store.report('directory', present)
    store.markCleared('directory')

    expect(store.statuses.directory.state).toBe('empty')
    expect(store.statuses.directory.syncedAt).toBe(1_000)
    expect(store.statuses.directory.bytes).toBeNull()
  })
})

describe('refreshStorage', () => {
  it('reads usage, quota and persistence', async () => {
    const store = useOfflineStore()
    await store.refreshStorage({
      estimate: async () => ({ usage: 500, quota: 10_000 }),
      persisted: async () => true,
    })

    expect(store.usageBytes).toBe(500)
    expect(store.quotaBytes).toBe(10_000)
    expect(store.persisted).toBe(true)
  })

  it('does nothing when the API is absent', async () => {
    const store = useOfflineStore()
    await store.refreshStorage(undefined)
    expect(store.usageBytes).toBeNull()
  })

  // Safari throws here in some privacy modes. Null is honest — the view then says it does not
  // know, instead of showing a confident zero.
  it('leaves values null when the API throws', async () => {
    const store = useOfflineStore()
    await store.refreshStorage({
      estimate: async () => {
        throw new Error('nope')
      },
    })
    expect(store.usageBytes).toBeNull()
  })
})

describe('headroomBytes', () => {
  // A device with a nearly full disk can report less than our conservative iOS 16.4 ceiling, and
  // believing our own number there would plan a download the phone cannot hold.
  it('trusts the browser quota when it is smaller than our ceiling', async () => {
    const store = useOfflineStore()
    await store.refreshStorage({ estimate: async () => ({ usage: 100, quota: 1_000 }) })
    expect(store.headroomBytes()).toBe(900)
  })

  it('falls back to our ceiling when the browser reports no quota', async () => {
    const store = useOfflineStore()
    await store.refreshStorage({ estimate: async () => ({ usage: 0 }) })
    expect(store.headroomBytes()).toBe(1024 * 1024 * 1024)
  })

  it('never reports negative headroom', async () => {
    const store = useOfflineStore()
    await store.refreshStorage({ estimate: async () => ({ usage: 5_000, quota: 1_000 }) })
    expect(store.headroomBytes()).toBe(0)
  })
})
