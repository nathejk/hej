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
    expect(store.persistence).toBe('unknown')
    expect(store.evictable).toBe(false)
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

  // Trimming tiles is the normal outcome of a quota eviction: the map still works, it just covers
  // less. Calling that 'evicted' overstates it; calling it 'synced' hides that coverage shrank.
  it('keeps a trimmed dataset usable but incomplete', () => {
    const store = useOfflineStore()
    reportAll(store)
    store.markEvicted('tiles', false)

    expect(store.statuses.tiles.state).toBe('synced')
    expect(store.statuses.tiles.complete).toBe(false)
    expect(store.statuses.tiles.bytes).toBe(10)
    expect(store.evicted).toContain('tiles')
  })

  it('records what reclaiming space cost', async () => {
    const store = useOfflineStore()
    reportAll(store)

    const result = await store.reclaimSpace({ tiles: { evict: async () => 5_000 } }, 1_000)

    expect(result.freedBytes).toBe(5_000)
    expect(store.evicted).toEqual(['tiles'])
    // Trimmed, not emptied — the map still has everything except the deepest zoom it dropped.
    expect(store.statuses.tiles.state).toBe('synced')
    expect(store.statuses.tiles.complete).toBe(false)
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
  it('reads usage and quota', async () => {
    const store = useOfflineStore()
    await store.refreshStorage({
      estimate: async () => ({ usage: 500, quota: 10_000 }),
      persisted: async () => true,
    })

    expect(store.usageBytes).toBe(500)
    expect(store.quotaBytes).toBe(10_000)
    expect(store.persistence).toBe('granted')
  })

  // `persisted()` returning false cannot tell "asked and refused" from "not asked yet", so
  // letting it write 'denied' would invent a refusal and warn the user about it.
  it('never infers a denial from persisted() being false', async () => {
    const store = useOfflineStore()
    await store.refreshStorage({
      estimate: async () => ({ usage: 1 }),
      persisted: async () => false,
    })

    expect(store.persistence).toBe('unknown')
    expect(store.evictable).toBe(false)
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

describe('ensurePersistence', () => {
  it('records a grant', async () => {
    const store = useOfflineStore()
    await store.ensurePersistence({ persist: async () => true })
    expect(store.persistence).toBe('granted')
    expect(store.evictable).toBe(false)
  })

  it('records a refusal, which is the one state worth telling the user about', async () => {
    const store = useOfflineStore()
    await store.ensurePersistence({ persist: async () => false })
    expect(store.persistence).toBe('denied')
    expect(store.evictable).toBe(true)
  })

  // Nothing the user can act on, so it must not produce a warning.
  it('records an unsupported browser without calling it a refusal', async () => {
    const store = useOfflineStore()
    await store.ensurePersistence({})
    expect(store.persistence).toBe('unsupported')
    expect(store.evictable).toBe(false)
  })

  // Asking again after a yes can surface a prompt on some engines, and re-asking someone who
  // already agreed is a good way to have them say no.
  it('does not ask again once granted', async () => {
    const store = useOfflineStore()
    let asked = 0
    const storage = {
      persist: async () => {
        asked++
        return true
      },
    }

    await store.ensurePersistence(storage)
    await store.ensurePersistence(storage)

    expect(asked).toBe(1)
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
