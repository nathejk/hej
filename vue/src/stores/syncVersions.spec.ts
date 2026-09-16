import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { fetchWrapper } from '@/helpers'
import { useCheckpointsStore } from '@/stores/checkpoints.store'
import { useContactsStore } from '@/stores/contacts.store'
import { useHandoutsStore } from '@/stores/handouts.store'
import { useProfileStore } from '@/stores/profile.store'
import { useScansStore } from '@/stores/scans.store'
import { useSessionStore } from '@/stores/session.store'
import { versionedRefresh } from '@/stores/syncVersions'

// The five stores the sync loop dispatches to must behave *identically* on a version, because the loop
// treats them identically. The properties, in the order they fail in production:
//
//   1. same version → no request at all (the common case, and the endpoint's whole economy);
//   2. different version → refetch, replacing the copy wholesale;
//   3. nothing held → fetch outright, version irrelevant;
//   4. a failed refetch keeps the copy AND does not record the version — otherwise the device holds
//      stale data labelled as current and never asks again.
//
// (4) is the one worth the most care: it is silent, and it is the client-side twin of the
// wrongly-stable server version the BFF's own tests guard against.

let getMock: ReturnType<typeof vi.fn>

beforeEach(() => {
  setActivePinia(createPinia())
  getMock = vi.fn()
  fetchWrapper.get = getMock as never
})

describe('versionedRefresh', () => {
  it('skips when the held version matches', async () => {
    const load = vi.fn(async () => true)
    expect(await versionedRefresh('abc', 'abc', load)).toBe(false)
    expect(load).not.toHaveBeenCalled()
  })

  it('loads when the versions differ', async () => {
    const load = vi.fn(async () => true)
    expect(await versionedRefresh('abc', 'def', load)).toBe(true)
    expect(load).toHaveBeenCalledTimes(1)
  })

  // Nothing held makes the version irrelevant, which is what lets the sync loop replace the old
  // quiet-prefetch pass without a special case for "never synced".
  it('loads when nothing is held, whatever the version', async () => {
    const load = vi.fn(async () => true)
    expect(await versionedRefresh('', '', load)).toBe(true)
    expect(load).toHaveBeenCalledTimes(1)
  })

  it('reports a failed load as no refresh', async () => {
    const load = vi.fn(async () => false)
    expect(await versionedRefresh('abc', 'def', load)).toBe(false)
  })
})

describe('scans store versioned refresh', () => {
  beforeEach(() => {
    getMock.mockResolvedValue({ scans: [{ id: 's1', kind: 'checkpoint', label: 'Post 1', lat: null, lng: null, scanned_at: '2026-09-16T10:00:00Z' }] })
  })

  it('fetches when nothing is held, then skips an unchanged version', async () => {
    const store = useScansStore()

    expect(await store.refreshIfVersionDiffers('v1')).toBe(true)
    expect(getMock).toHaveBeenCalledTimes(1)
    expect(store.version).toBe('v1')

    expect(await store.refreshIfVersionDiffers('v1')).toBe(false)
    expect(getMock).toHaveBeenCalledTimes(1)
  })

  it('refetches on a changed version', async () => {
    const store = useScansStore()
    await store.refreshIfVersionDiffers('v1')

    getMock.mockResolvedValue({ scans: [] })
    expect(await store.refreshIfVersionDiffers('v2')).toBe(true)
    expect(store.scans).toEqual([])
    expect(store.version).toBe('v2')
  })

  it('keeps the copy and the old version when the refetch fails', async () => {
    const store = useScansStore()
    await store.refreshIfVersionDiffers('v1')
    const held = store.scans

    getMock.mockRejectedValue(new Error('offline'))
    expect(await store.refreshIfVersionDiffers('v2')).toBe(false)

    expect(store.scans).toEqual(held)
    expect(store.error).not.toBe('')
    // The point of the whole exercise: 'v2' must not be recorded, or the next check would see a match
    // and never refetch the data we failed to get.
    expect(store.version).toBe('v1')
  })
})

describe('handouts store versioned refresh', () => {
  beforeEach(() => {
    getMock.mockResolvedValue({
      handouts: [{ name: 'Kort 1', format: 'a4', qr_id: '1', handed_out: '2026-09-16T10:00:00Z', still_held: true }],
    })
  })

  it('skips an unchanged version and refetches a changed one', async () => {
    const store = useHandoutsStore()
    store.storage = null

    await store.refreshIfVersionDiffers('v1')
    expect(getMock).toHaveBeenCalledTimes(1)

    await store.refreshIfVersionDiffers('v1')
    expect(getMock).toHaveBeenCalledTimes(1)

    await store.refreshIfVersionDiffers('v2')
    expect(getMock).toHaveBeenCalledTimes(2)
    expect(store.version).toBe('v2')
  })

  // Replace, not merge: a sheet the server stopped sending has been reassigned away and must stop
  // existing here, or the drawer keeps naming a sheet the patrol handed over.
  it('replaces the list wholesale', async () => {
    const store = useHandoutsStore()
    store.storage = null
    await store.refreshIfVersionDiffers('v1')

    getMock.mockResolvedValue({ handouts: [] })
    await store.refreshIfVersionDiffers('v2')

    expect(store.handouts).toEqual([])
  })

  it('keeps the old version when the refetch fails', async () => {
    const store = useHandoutsStore()
    store.storage = null
    await store.refreshIfVersionDiffers('v1')

    getMock.mockRejectedValue(new Error('offline'))
    expect(await store.refreshIfVersionDiffers('v2')).toBe(false)
    expect(store.version).toBe('v1')
    expect(store.handouts).toHaveLength(1)
  })

  // Without persistence, every cold start refetches every dataset before the check can say
  // "unchanged" — which is the cost this design exists to remove.
  it('persists the version with the payload and reads it back after a reload', async () => {
    const backing = new Map<string, string>()
    const storage = {
      getItem: (k: string) => backing.get(k) ?? null,
      setItem: (k: string, v: string) => void backing.set(k, v),
      removeItem: (k: string) => void backing.delete(k),
    } as never

    // A signed-in profile, because the storage key is deliberately profile-scoped and null without one.
    const session = useSessionStore()
    session.user = { userId: 'u-1', name: 'Signe', role: 'spejder' } as never

    const store = useHandoutsStore()
    store.storage = storage
    await store.refreshIfVersionDiffers('v1')
    expect(store.version).toBe('v1')

    // A "reload": a brand-new pinia, hydrating from the same storage.
    setActivePinia(createPinia())
    const reloadedSession = useSessionStore()
    reloadedSession.user = { userId: 'u-1', name: 'Signe', role: 'spejder' } as never
    const reloaded = useHandoutsStore()
    reloaded.storage = storage
    reloaded.hydrate()

    expect(reloaded.version).toBe('v1')
    expect(reloaded.handouts).toHaveLength(1)

    // And therefore the same version costs no request at all after a cold start.
    getMock.mockClear()
    expect(await reloaded.refreshIfVersionDiffers('v1')).toBe(false)
    expect(getMock).not.toHaveBeenCalled()
  })
})

describe('checkpoints store versioned refresh', () => {
  beforeEach(() => {
    getMock.mockResolvedValue({
      checkpoints: [{ id: 'cp1', name: 'Post 1', checkgroup: 'cg1', sort_order: 0, lat: 56, lng: 9, open_from: 0, open_until: 0, open_duration_minutes: 0 }],
      next_checkgroup: 'cg1',
    })
  })

  it('skips an unchanged version and refetches a changed one', async () => {
    const store = useCheckpointsStore()
    store.storage = null

    await store.refreshIfVersionDiffers('v1')
    expect(store.checkpoints).toHaveLength(1)
    expect(getMock).toHaveBeenCalledTimes(1)

    await store.refreshIfVersionDiffers('v1')
    expect(getMock).toHaveBeenCalledTimes(1)
  })

  // The arrow moving to the next line is a change the patrol must see even though no post moved — the
  // server hashes `next_checkgroup` for exactly this reason.
  it('applies a next-line change arriving under a new version', async () => {
    const store = useCheckpointsStore()
    store.storage = null
    await store.refreshIfVersionDiffers('v1')

    getMock.mockResolvedValue({
      checkpoints: [{ id: 'cp1', name: 'Post 1', checkgroup: 'cg1', sort_order: 0, lat: 56, lng: 9, open_from: 0, open_until: 0, open_duration_minutes: 0 }],
      next_checkgroup: 'cg2',
    })
    await store.refreshIfVersionDiffers('v2')

    expect(store.nextCheckgroup).toBe('cg2')
  })

  it('keeps the old version when the refetch fails', async () => {
    const store = useCheckpointsStore()
    store.storage = null
    await store.refreshIfVersionDiffers('v1')

    getMock.mockRejectedValue(new Error('offline'))
    await store.refreshIfVersionDiffers('v2')

    expect(store.version).toBe('v1')
    expect(store.checkpoints).toHaveLength(1)
  })
})

describe('profile store versioned refresh', () => {
  beforeEach(() => {
    getMock.mockResolvedValue({
      name: 'Signe', role: 'spejder', team: 'Ravnene', section: '', address: 'Vej 1',
      postal_code: '8600', city: 'Silkeborg', phone: '+4530000001', phone_parent: '+4520000001',
      has_photo: false, confirmation_required: true, verified_at: null,
    })
  })

  it('skips an unchanged version and refetches a changed one', async () => {
    const store = useProfileStore()

    await store.refreshIfVersionDiffers('v1')
    expect(store.details?.name).toBe('Signe')
    expect(getMock).toHaveBeenCalledTimes(1)

    await store.refreshIfVersionDiffers('v1')
    expect(getMock).toHaveBeenCalledTimes(1)

    getMock.mockResolvedValue({
      name: 'Signe', role: 'spejder', team: 'Ravnene', section: '', address: 'Vej 1',
      postal_code: '8600', city: 'Silkeborg', phone: '+4530000001', phone_parent: '+4520000009',
      has_photo: false, confirmation_required: false, verified_at: '2026-09-16T10:00:00Z',
    })
    await store.refreshIfVersionDiffers('v2')

    // A guardian number corrected on another device, and a confirmation made there, both arrive.
    expect(store.details?.phoneParent).toBe('+4520000009')
    expect(store.confirmationRequired).toBe(false)
  })

  it('keeps the old version when the refetch fails', async () => {
    const store = useProfileStore()
    await store.refreshIfVersionDiffers('v1')

    getMock.mockRejectedValue(new Error('offline'))
    await store.refreshIfVersionDiffers('v2')

    expect(store.syncVersion).toBe('v1')
    expect(store.details?.name).toBe('Signe')
  })
})

describe('contacts store versioned refresh', () => {
  beforeEach(() => {
    getMock.mockResolvedValue({
      version: 'v1',
      expiresAt: 0,
      entries: [{ id: 'p1', name: 'Kim Krew', population: 'crew', groups: [] }],
    })
  })

  it('skips an unchanged version without asking for a version of its own', async () => {
    const store = useContactsStore()
    store.storage = null

    await store.refreshIfVersionDiffers('v1')
    expect(getMock).toHaveBeenCalledTimes(1)
    expect(getMock.mock.calls[0][0]).toBe('/api/contacts/manifest')

    // The point of the multiplexed check: no request at all, and in particular no call to the old
    // per-dataset version endpoint.
    await store.refreshIfVersionDiffers('v1')
    expect(getMock).toHaveBeenCalledTimes(1)
  })

  it('refetches when the version differs', async () => {
    const store = useContactsStore()
    store.storage = null
    await store.refreshIfVersionDiffers('v1')

    getMock.mockResolvedValue({ version: 'v2', expiresAt: 0, entries: [] })
    expect(await store.refreshIfVersionDiffers('v2')).toBe(true)
    expect(store.entries).toEqual([])
    expect(store.version).toBe('v2')
  })

  // A role without the pane must not be nudged into asking for it again on every foreground — the
  // reason the old loop had an `enabled` gate.
  it('does nothing for a forbidden role', async () => {
    const store = useContactsStore()
    store.storage = null
    store.forbidden = true

    expect(await store.refreshIfVersionDiffers('v9')).toBe(false)
    expect(getMock).not.toHaveBeenCalled()
  })

  // The manifest carries its own version, so what gets held is always what the payload delivered —
  // this store cannot end up holding a version whose data never arrived.
  it('holds the version the payload delivered, not the one it was given', async () => {
    const store = useContactsStore()
    store.storage = null
    getMock.mockResolvedValue({ version: 'server-truth', expiresAt: 0, entries: [] })

    await store.refreshIfVersionDiffers('what-sync-said')

    expect(store.version).toBe('server-truth')
  })
})
