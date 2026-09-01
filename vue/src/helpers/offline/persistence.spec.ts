import { describe, expect, it } from 'vitest'

import { requestPersistence } from '@/helpers/offline/persistence'

describe('requestPersistence', () => {
  it('grants without asking when persistence is already held', async () => {
    let asked = false
    const outcome = await requestPersistence({
      persisted: async () => true,
      persist: async () => {
        asked = true
        return true
      },
    })

    expect(outcome).toBe('granted')
    // The whole reason for the pre-check: a repeat request can surface a prompt on some
    // engines, and asking someone who already agreed is a good way to have them say no.
    expect(asked).toBe(false)
  })

  it('asks when persistence is not yet held', async () => {
    const outcome = await requestPersistence({
      persisted: async () => false,
      persist: async () => true,
    })
    expect(outcome).toBe('granted')
  })

  it('reports a refusal', async () => {
    const outcome = await requestPersistence({
      persisted: async () => false,
      persist: async () => false,
    })
    expect(outcome).toBe('denied')
  })

  it('reports an engine without the API as unsupported, not denied', async () => {
    expect(await requestPersistence({})).toBe('unsupported')
    expect(await requestPersistence(undefined)).toBe('unsupported')
  })

  // Safari throws here in some privacy modes rather than returning false. Calling that
  // 'denied' would warn the user about a decision nobody made.
  it('reports a throwing API as unsupported', async () => {
    const outcome = await requestPersistence({
      persist: async () => {
        throw new Error('blocked')
      },
    })
    expect(outcome).toBe('unsupported')
  })

  it('works when persisted() is absent but persist() is not', async () => {
    expect(await requestPersistence({ persist: async () => true })).toBe('granted')
  })
})
