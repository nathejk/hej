import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { resetLocalState } from '@/dev/resets'
import { useOnboardingStore } from '@/stores/onboarding.store'
import { useOfflineStore } from '@/stores/offline.store'

// `caches`, `indexedDB` and `navigator.serviceWorker` are all absent under vitest's node
// environment, which is convenient rather than limiting: it means these tests assert the parts
// that carry the design decisions — which stores get called, and which dataset is spared —
// without standing up a browser. The absent APIs are guarded in the module, so they no-op.

describe('dev resets', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('resets onboarding through the store action, not by clearing storage', async () => {
    const onboarding = useOnboardingStore()
    const spy = vi.spyOn(onboarding, 'reset')

    await resetLocalState('onboarding')

    // The store's own action is what sign-out uses too, so this button and the product path
    // cannot drift apart.
    expect(spy).toHaveBeenCalledOnce()
  })

  it('logs out through the session store', async () => {
    // Imported lazily so the spy is installed on the same instance the module resolves.
    const { useSessionStore } = await import('@/stores/session.store')
    const session = useSessionStore()
    const spy = vi.spyOn(session, 'logout').mockResolvedValue(undefined)

    await resetLocalState('session')

    expect(spy).toHaveBeenCalledOnce()
  })

  // The point of the whole module: dataset clearing dispatches to the owning feature's
  // registered handler, so `offline.store`'s status bookkeeping runs. Reaching into IndexedDB
  // directly would leave the readiness view reporting something untrue.
  it('clears datasets through offline.store.clear, and never the unrecoverable one', async () => {
    const offline = useOfflineStore()
    const cleared: string[] = []
    vi.spyOn(offline, 'clear').mockImplementation(async (id) => {
      cleared.push(id)
    })

    await resetLocalState('caches')

    expect(cleared.length).toBeGreaterThan(0)
    // The position track: for a participant its local copy may be the only record of where a
    // team was, so the store refuses to clear it and this module respects that refusal. The dev
    // track is deleted by a separate, explicitly dev-only path instead.
    expect(cleared).not.toContain('track')
  })

  it('does not touch identity when only caches were asked for', async () => {
    const { useSessionStore } = await import('@/stores/session.store')
    const logout = vi.spyOn(useSessionStore(), 'logout').mockResolvedValue(undefined)
    const reset = vi.spyOn(useOnboardingStore(), 'reset')

    await resetLocalState('caches')

    expect(logout).not.toHaveBeenCalled()
    expect(reset).not.toHaveBeenCalled()
  })

  it('reports what it did', async () => {
    const report = await resetLocalState('caches')
    // A button that appears to do nothing is worse than no button.
    expect(report).toMatch(/datasets/)
    expect(report).toMatch(/caches/)
    expect(report).toMatch(/sw$/)
  })
})
