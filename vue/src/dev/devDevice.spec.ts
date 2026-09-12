import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  DEV_DEVICE_KEY,
  DEV_KEY_PREFIX,
  applyPreset,
  clearDevOverrides,
  initDevDevice,
  readDevDevice,
  writeDevDevice,
  type DevEnv,
  type DevStorage,
} from '@/dev/devDevice'

// A Map-backed `Storage`, because vitest runs `environment: 'node'` and there is no
// `localStorage` here. Enumerable, so the prefix scan can be tested.
function memory(initial: Record<string, string> = {}): DevStorage {
  const map = new Map(Object.entries(initial))
  return {
    getItem: (key) => map.get(key) ?? null,
    setItem: (key, value) => void map.set(key, value),
    removeItem: (key) => void map.delete(key),
    get length() {
      return map.size
    },
    key: (index) => [...map.keys()][index] ?? null,
  }
}

function env(storage: DevStorage | null = memory(), prod = false): DevEnv {
  return { storage, prod }
}

describe('presets', () => {
  it('simulates an installed iPhone', () => {
    const e = env()
    expect(applyPreset('iphone', e)).toEqual({
      mobile: true,
      standalone: true,
      platform: 'ios',
    })
    expect(readDevDevice(e)).toEqual({ mobile: true, standalone: true, platform: 'ios' })
  })

  it.each([
    ['ipad', 'ipad'],
    ['android', 'android'],
    ['chromium', 'chromium'],
  ])('simulates an installed %s', (name, platform) => {
    const e = env()
    expect(applyPreset(name, e)).toEqual({ mobile: true, standalone: true, platform })
  })

  // The install wall's only route onto a laptop: a laptop with no profile never sees the
  // wall at all, because the device gate sends it out of the SPA first.
  it('tab is mobile but not standalone', () => {
    const e = env()
    expect(applyPreset('tab', e)).toEqual({
      mobile: true,
      standalone: false,
      platform: 'ios',
    })
  })

  // Otherwise `?dev=android` then `?dev=tab` would quietly show the iOS wall — the wrong
  // instructions, and no indication that the platform had changed underfoot.
  it('tab keeps the platform already being simulated', () => {
    const e = env()
    applyPreset('android', e)
    expect(applyPreset('tab', e)).toEqual({
      mobile: true,
      standalone: false,
      platform: 'android',
    })
  })

  // A standalone webview is not a state any real device can be in, so it must not be a
  // state this can produce.
  it('webview is never standalone', () => {
    expect(applyPreset('webview', env())).toEqual({
      mobile: true,
      standalone: false,
      platform: 'webview',
    })
  })

  it.each(['desktop', 'off'])('%s clears the profile', (name) => {
    const e = env()
    applyPreset('iphone', e)
    expect(applyPreset(name, e)).toBeNull()
    expect(readDevDevice(e)).toBeNull()
  })

  // undefined, not null: "you typed a name I do not know" and "simulation is off" must not
  // collapse into one answer, or a typo silently looks like a working clear.
  it('reports an unknown preset distinguishably from a clear', () => {
    expect(applyPreset('nokia3310', env())).toBeUndefined()
  })
})

describe('initDevDevice', () => {
  it('applies ?dev= and persists it', () => {
    const e = env()
    expect(initDevDevice('?dev=iphone', e)).toEqual({
      mobile: true,
      standalone: true,
      platform: 'ios',
    })
    // Persisted, which is the half that matters: the manifest's start_url is '/', so an
    // installed launch drops the query string entirely.
    expect(readDevDevice(e)).not.toBeNull()
  })

  it('is case-insensitive', () => {
    expect(initDevDevice('?dev=iPhone', env())).toEqual({
      mobile: true,
      standalone: true,
      platform: 'ios',
    })
  })

  it('leaves an existing profile alone when no ?dev= is present', () => {
    const e = env()
    applyPreset('android', e)
    expect(initDevDevice('', e)).toEqual({
      mobile: true,
      standalone: true,
      platform: 'android',
    })
  })

  it('warns on an unknown preset and leaves the profile untouched', () => {
    const e = env()
    applyPreset('iphone', e)
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})

    expect(initDevDevice('?dev=nokia3310', e)).toEqual({
      mobile: true,
      standalone: true,
      platform: 'ios',
    })
    expect(warn).toHaveBeenCalled()
    warn.mockRestore()
  })

  it('clears via ?dev=off', () => {
    const e = env()
    applyPreset('iphone', e)
    expect(initDevDevice('?dev=off', e)).toBeNull()
  })
})

describe('stored value validation', () => {
  // Storage is input. A half-populated profile would put the app in a state no real device
  // can be in (`mobile: undefined`), and the resulting bug would look like one in the gates.
  it.each([
    ['not json at all', 'iphone'],
    ['json but not an object', '42'],
    ['missing mobile', '{"standalone":true,"platform":"ios"}'],
    ['missing standalone', '{"mobile":true,"platform":"ios"}'],
    ['non-boolean mobile', '{"mobile":"yes","standalone":true,"platform":"ios"}'],
    ['unknown platform', '{"mobile":true,"standalone":true,"platform":"symbian"}'],
    ['empty', ''],
  ])('reads %s as no simulation', (_label, raw) => {
    expect(readDevDevice(env(memory({ [DEV_DEVICE_KEY]: raw })))).toBeNull()
  })
})

describe('storage failures', () => {
  it('degrades quietly when storage is unavailable', () => {
    const e = env(null)
    expect(readDevDevice(e)).toBeNull()
    expect(() => writeDevDevice({ mobile: true, standalone: true, platform: 'ios' }, e)).not.toThrow()
    expect(() => clearDevOverrides(e)).not.toThrow()
    expect(() => initDevDevice('?dev=iphone', e)).not.toThrow()
  })

  // Safari throws on localStorage access in some privacy modes; every access in this app is
  // wrapped for that reason.
  it('degrades quietly when storage throws', () => {
    const hostile: DevStorage = {
      getItem: () => {
        throw new Error('SecurityError')
      },
      setItem: () => {
        throw new Error('SecurityError')
      },
      removeItem: () => {
        throw new Error('SecurityError')
      },
    }
    const e = env(hostile)
    expect(readDevDevice(e)).toBeNull()
    expect(() => applyPreset('iphone', e)).not.toThrow()
  })
})

describe('clearDevOverrides', () => {
  it('removes every hej.dev.* key and nothing else', () => {
    const storage = memory({
      [DEV_DEVICE_KEY]: '{"mobile":true,"standalone":true,"platform":"ios"}',
      [`${DEV_KEY_PREFIX}panel`]: 'open',
      [`${DEV_KEY_PREFIX}position`]: '{}',
      // Product keys, which must survive: clearing the dev layer is not a factory reset.
      'hej.gates.bypass': '1',
      'hej.install-gate': '1',
      'hej.dataforsyningen-token': 'abc',
    })

    clearDevOverrides(env(storage))

    expect(storage.getItem(DEV_DEVICE_KEY)).toBeNull()
    expect(storage.getItem(`${DEV_KEY_PREFIX}panel`)).toBeNull()
    expect(storage.getItem(`${DEV_KEY_PREFIX}position`)).toBeNull()
    expect(storage.getItem('hej.gates.bypass')).toBe('1')
    expect(storage.getItem('hej.install-gate')).toBe('1')
    expect(storage.getItem('hej.dataforsyningen-token')).toBe('abc')
  })

  it('is a no-op against storage that cannot enumerate', () => {
    const storage: DevStorage = {
      getItem: () => null,
      setItem: () => {},
      removeItem: () => {
        throw new Error('should not be called')
      },
    }
    expect(() => clearDevOverrides(env(storage))).not.toThrow()
  })
})

// The module's single most important property. Vite compiles these branches out of a
// production bundle, so in practice there is no code to reach — but the guard is asserted
// here because that compilation is a build-time claim and this is the behavioural one: a
// production build must not be persuadable into simulating a phone by a URL.
describe('production inertness', () => {
  const PROD = true

  it('reads nothing, even from a populated store', () => {
    const storage = memory({
      [DEV_DEVICE_KEY]: '{"mobile":true,"standalone":true,"platform":"ios"}',
    })
    expect(readDevDevice(env(storage, PROD))).toBeNull()
  })

  it('writes nothing', () => {
    const storage = memory()
    writeDevDevice({ mobile: true, standalone: true, platform: 'ios' }, env(storage, PROD))
    expect(storage.getItem(DEV_DEVICE_KEY)).toBeNull()
  })

  it('ignores ?dev= entirely', () => {
    const storage = memory()
    expect(initDevDevice('?dev=iphone', env(storage, PROD))).toBeNull()
    expect(storage.getItem(DEV_DEVICE_KEY)).toBeNull()
  })

  it('does not clear either — it touches nothing at all', () => {
    const storage = memory({ [`${DEV_KEY_PREFIX}panel`]: 'open' })
    clearDevOverrides(env(storage, PROD))
    expect(storage.getItem(`${DEV_KEY_PREFIX}panel`)).toBe('open')
  })
})

describe('caching', () => {
  // The dev panel (task 208) toggles the profile at runtime, and a value memoised at import
  // time would make the panel's readout disagree with the app's behaviour — the exact
  // confusion the readout exists to prevent.
  it('does not memoise: a later write is visible to the next read', () => {
    const e = env()
    expect(readDevDevice(e)).toBeNull()
    applyPreset('iphone', e)
    expect(readDevDevice(e)?.platform).toBe('ios')
    applyPreset('android', e)
    expect(readDevDevice(e)?.platform).toBe('android')
  })
})

// A guard on the collaboration with task 207 rather than on this file: `hej.gates.bypass` is
// a different switch with a different meaning (it verifies the install_gate kill switch), and
// PRD 014 requires the two stay independent.
describe('independence from the gate bypass', () => {
  it('never touches hej.gates.bypass', () => {
    const storage = memory({ 'hej.gates.bypass': '1' })
    const e = env(storage)
    applyPreset('iphone', e)
    applyPreset('off', e)
    initDevDevice('?dev=tab', e)
    expect(storage.getItem('hej.gates.bypass')).toBe('1')
  })
})
