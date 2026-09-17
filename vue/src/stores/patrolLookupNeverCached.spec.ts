import { readFileSync, readdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

import { RUNTIME_CACHE_MATCHERS } from '@/config/cache'
import { OFFLINE_DATASETS } from '@/config/offline'

// **Nothing from a patrol lookup is ever written to a device** (PRD 007 §8, task 170).
//
// # What breaks if this file is deleted
//
// The patrol lookup is the only path in this app by which a spejder's details — name, phone, status,
// and a photograph of a minor's face — are reachable at all. An earlier draft of PRD 007 had it
// working offline, which meant shipping ~557 spejder thumbnails to every crew device. That was the
// largest privacy cost in the design, and the 2026-08-31 decision to leave the lookup uncached
// removed it entirely.
//
// **The threat is drift, not malice.** Nobody is going to add a "cache minors' faces" feature. What
// will happen is one of these:
//
//   - a broad service-worker rule (`/^\/api\//`, or "cache all images") added for something else;
//   - the portrait matcher loosened from `contacts/people/…` to `contacts/.*\/photo`, by somebody
//     reasonably thinking the two photo endpoints are the same kind of thing;
//   - "add offline support to the lookup", which sounds like a kindness at 03:00 in woodland;
//   - a generic PRD 009 dataset registration that sweeps it up.
//
// Every one of those is silent. The lookup would keep working, the app would look faster, and the
// only symptom would be several hundred children's photographs sitting in a Cache Storage bucket on
// a crew phone that nobody remembers to wipe.
//
// # Why the matchers are imported rather than grepped
//
// The interesting assertions here **run the real Workbox matchers against real lookup URLs**. A grep
// for `patrols` in `vite.config.ts` would prove today's spelling and nothing else — it would pass
// happily against a `/^\/api\//` rule that caches everything. Importing the values means the test
// asks the question that matters: *would any cache claim this URL?*
//
// That is why `src/config/cache.ts` exports them and the build config references them, rather than
// the patterns living as literals in `vite.config.ts` where no test could reach them. The refactor
// was verified output-equivalent: the `registerRoute` calls in the built `sw.js` are byte-identical.

/** The two lookup routes, exactly as `PatrolLookup.vue` builds them. */
const LOOKUP_URLS = [
  'https://hej.nathejk.dk/api/contacts/patrols/138',
  'https://hej.nathejk.dk/api/contacts/patrols/138/photo/p-abc-123?size=thumb',
  // A number with no members, and an id with characters that survive encoding — same routes, but
  // worth driving so a matcher anchored on a digit shape cannot pass by accident.
  'https://hej.nathejk.dk/api/contacts/patrols/7',
  'https://hej.nathejk.dk/api/contacts/patrols/7/photo/00000000-0000-0000-0000-000000000000',
]

/** Runs one matcher the way Workbox would. */
function claims(
  pattern: RegExp | ((ctx: { url: URL; sameOrigin: boolean }) => boolean),
  href: string,
): boolean {
  const url = new URL(href)
  if (typeof pattern === 'function') {
    // Same-origin is the truthful value for these: the lookup is served by this app's own BFF, so a
    // matcher that only guards on cross-origin would not save us.
    return pattern({ url, sameOrigin: true })
  }
  return pattern.test(href)
}

describe('the patrol lookup is never cached by the service worker', () => {
  it('has matchers to check', () => {
    // Guards the guard: if the export is emptied or renamed, this must not become a test that
    // silently checks nothing and passes.
    expect(RUNTIME_CACHE_MATCHERS.length).toBeGreaterThanOrEqual(4)
  })

  it('is claimed by no runtime cache', () => {
    const offenders: string[] = []

    for (const { name, pattern } of RUNTIME_CACHE_MATCHERS) {
      for (const href of LOOKUP_URLS) {
        if (claims(pattern, href)) offenders.push(`${name} claims ${href}`)
      }
    }

    expect(
      offenders,
      'A runtime cache now claims a patrol-lookup URL. This is the invariant PRD 007 calls the ' +
        'most important one in the design: no spejder record is ever written to a device. The ' +
        'lookup serves minors\' names, phone numbers and faces, and a Cache Storage entry outlives ' +
        'the session, the role change and the event. Do not relax this to make the lookup work ' +
        'offline — that was considered and rejected (PRD 007 §8); the fallback is the radio.',
    ).toEqual([])
  })

  // The specific loosening most likely to happen, called out on its own so the failure message can
  // name it. The directory portrait rule and the patrol photo route differ by one path segment.
  it('does not let the portrait cache reach patrol photos', () => {
    const portrait = RUNTIME_CACHE_MATCHERS.find((m) => m.name === 'directory portraits')
    expect(portrait, 'the directory portrait matcher is gone — has it been renamed?').toBeTruthy()

    // The directory's own photo URL *must* still be cached: this is not a test that the rule does
    // nothing, it is a test that it does exactly one thing.
    expect(
      claims(portrait!.pattern, 'https://hej.nathejk.dk/api/contacts/people/p-1/photo?v=abc'),
      'the directory portrait cache has stopped matching directory portraits',
    ).toBe(true)

    // And must not reach the patrol lookup's.
    expect(
      claims(portrait!.pattern, 'https://hej.nathejk.dk/api/contacts/patrols/138/photo/p-1'),
      'the portrait cache now claims patrol photos. `people` and `patrols` are not two spellings ' +
        'of the same endpoint: one serves crew and gøglere who are in the directory, the other ' +
        'serves minors and must never touch disk.',
    ).toBe(false)
  })
})

describe('the patrol lookup is not a cached dataset', () => {
  // PRD 009 §8: anything registered here is measured, budgeted and *persisted*. A lookup dataset
  // would be a browsable index of minors' faces with a progress bar.
  it('is not an offline dataset', () => {
    const ids = OFFLINE_DATASETS.map((d) => d.id)
    for (const forbidden of ['patrol', 'patrols', 'lookup']) {
      expect(ids, `"${forbidden}" is registered as an offline dataset`).not.toContain(forbidden)
    }
  })

  // `/api/sync` drives every refresh. A `patrol` key would mean the client holds a copy to compare a
  // version against — which is the whole thing this task exists to prevent.
  it('is not a sync dataset', () => {
    const source = readFileSync(
      fileURLToPath(new URL('./syncVersions.ts', import.meta.url)),
      'utf8',
    )
    const union = source.match(/export type SyncDataset =[\s\S]*?\n\n/)?.[0] ?? ''
    expect(union, 'the SyncDataset union was not found — has it moved?').toContain('contacts')
    expect(union).not.toMatch(/patrol|lookup/i)
  })
})

describe('nothing persists a patrol lookup result', () => {
  const SRC = fileURLToPath(new URL('../', import.meta.url))

  function sourceFiles(dir: string): string[] {
    const out: string[] = []
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      if (entry.name === 'ui') continue // generated primitives
      const path = `${dir}${entry.name}`
      if (entry.isDirectory()) out.push(...sourceFiles(`${path}/`))
      else if (/\.(ts|vue)$/.test(entry.name) && !entry.name.endsWith('.spec.ts')) out.push(path)
    }
    return out
  }

  // Comments stripped, following `profileNotCached.spec.ts`: `PatrolLookup.vue`'s header explains at
  // length why nothing is written to localStorage, and a guard that fires on its own documentation
  // teaches people to delete the documentation.
  function code(source: string): string {
    return source
      .replace(/<!--[\s\S]*?-->/g, '')
      .replace(/\/\*[\s\S]*?\*\//g, '')
      .replace(/^\s*(\/\/|\*).*$/gm, '')
  }

  const PERSISTENCE = /localStorage|sessionStorage|indexedDB|caches\.open|profileKey\(/

  // The behavioural half of the invariant, asserted structurally because this suite has no DOM and
  // mounts nothing. The scope is any file that touches the lookup routes at all — which is the
  // right net, since a file that does not name the endpoint cannot persist its result.
  it('no file that touches the lookup routes also writes to storage', () => {
    const offenders: string[] = []
    let sawLookup = false

    for (const file of sourceFiles(SRC)) {
      const source = code(readFileSync(file, 'utf8'))
      if (!/contacts\/patrols/.test(source)) continue
      sawLookup = true
      if (PERSISTENCE.test(source)) offenders.push(file.replace(SRC, ''))
    }

    expect(sawLookup, 'no file references the lookup routes — this test is checking nothing').toBe(
      true,
    )
    expect(
      offenders,
      'A file that performs a patrol lookup also writes to device storage. Results must live in ' +
        'component state only, for as long as the panel is open. A recent-lookups list is the ' +
        'specific temptation PRD 007 §8 names, and it accumulates into the browsable index of ' +
        'minors\' faces the design exists to avoid.',
    ).toEqual([])
  })

  // The lookup component holds its results in refs and clears them on close. Asserted so that
  // "closing drops them" cannot quietly become "closing leaves them for next time".
  it('the lookup panel clears its results when it closes', () => {
    const component = code(
      readFileSync(`${SRC}components/contacts/PatrolLookup.vue`, 'utf8'),
    )
    expect(component, 'the lookup no longer resets on close').toMatch(/watch\(\s*open/)
    expect(component).toContain('reset()')
    // The results themselves must be plain component state, not a store.
    expect(component, 'the lookup now reads or writes a Pinia store').not.toMatch(/use\w+Store\(/)
  })
})

// The third leg of this invariant — `Cache-Control: no-store` on both lookup responses — is asserted
// where the response is actually written: `go/cmd/api/patrol_test.go` covers both routes, and covers
// the refusal and not-found paths too, because a cacheable refusal is still a cached answer about a
// real child.
//
// Deliberately **not** asserted from here. A first draft read that file across the directory tree and
// failed: the `ui` container mounts only `vue/`, so `go/` does not exist from the suite's point of
// view. That is the right constraint rather than an obstacle — a frontend test reaching into the
// backend's source to check a header is the wrong place for the claim, and it would pass on a
// developer's host while failing in CI. Named here so a reader looking for the third leg knows where
// it lives.
