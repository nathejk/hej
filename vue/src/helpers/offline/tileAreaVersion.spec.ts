import { beforeEach, describe, expect, it } from 'vitest'

import {
  forgetTileAreaVersion,
  heldTileAreaVersion,
  rememberTileAreaVersion,
  tileAreaIsStale,
  type VersionStorage,
} from '@/helpers/offline/tileAreaVersion'

// The rule this file defends is a rule about *not* speaking: the race area is the one sync dataset with
// nothing to refresh, so all a changed version can do is tell the user something. Telling them wrongly —
// nagging about a map they never downloaded, or about a change we cannot actually establish — is worse
// than saying nothing, because the only action it prompts is a few hundred megabytes on mobile data.

function fakeStorage(seed: Record<string, string> = {}): VersionStorage {
  const backing = new Map(Object.entries(seed))
  return {
    getItem: (k) => backing.get(k) ?? null,
    setItem: (k, v) => void backing.set(k, v),
    removeItem: (k) => void backing.delete(k),
  }
}

let storage: VersionStorage

beforeEach(() => {
  storage = fakeStorage()
})

describe('tileAreaIsStale', () => {
  it('is stale when the served version differs from the one the tiles were downloaded for', () => {
    expect(tileAreaIsStale('v1', 'v2')).toBe(true)
  })

  it('is not stale when they match', () => {
    expect(tileAreaIsStale('v1', 'v1')).toBe(false)
  })

  // A device that never downloaded tiles has nothing to be stale — and, just as important, a device that
  // downloaded them before this version existed (task 294 shipped after task 087) must not be told its
  // map is out of date on the strength of a value we never stored. Absent is not stale.
  it('makes no claim when nothing was ever recorded', () => {
    expect(tileAreaIsStale('', 'v2')).toBe(false)
  })

  // The sync check omits `race_area` when the caller cannot hold it and lists it as unavailable when the
  // server could not derive it. Neither is evidence that anything moved.
  it('makes no claim when the server did not serve a version', () => {
    expect(tileAreaIsStale('v1', '')).toBe(false)
  })
})

describe('remembering the tile area version', () => {
  it('round-trips through storage', () => {
    rememberTileAreaVersion('v1', storage)
    expect(heldTileAreaVersion(storage)).toBe('v1')
  })

  it('reads as empty when nothing is stored', () => {
    expect(heldTileAreaVersion(storage)).toBe('')
  })

  // An older BFF sends no version. Storing '' would be indistinguishable from "never downloaded", which is
  // the right *outcome*, but writing it would also overwrite a real version recorded by a newer response.
  it('ignores an empty version rather than storing it', () => {
    rememberTileAreaVersion('v1', storage)
    rememberTileAreaVersion('', storage)
    expect(heldTileAreaVersion(storage)).toBe('v1')
  })

  it('forgets on request', () => {
    rememberTileAreaVersion('v1', storage)
    forgetTileAreaVersion(storage)
    expect(heldTileAreaVersion(storage)).toBe('')
  })

  // No storage at all — a node run, or Safari in a privacy mode where touching localStorage throws. The
  // tiles still download and still work; all that is lost is noticing a later change.
  it('degrades to silence with no storage', () => {
    expect(() => rememberTileAreaVersion('v1', null)).not.toThrow()
    expect(heldTileAreaVersion(null)).toBe('')
    expect(() => forgetTileAreaVersion(null)).not.toThrow()
  })

  it('survives storage that throws on write', () => {
    const hostile: VersionStorage = {
      getItem: () => null,
      setItem: () => {
        throw new Error('QuotaExceededError')
      },
      removeItem: () => {
        throw new Error('SecurityError')
      },
    }
    expect(() => rememberTileAreaVersion('v1', hostile)).not.toThrow()
    expect(() => forgetTileAreaVersion(hostile)).not.toThrow()
  })
})
