import { readFileSync, readdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

// The member's own profile is never cached (PRD 015, task 233).
//
// # Why this file exists rather than a fix
//
// Task 233 was written expecting a stale cached profile to be holding a contact number that task
// 229 now blanks — the member's own number, which reads as verifiable and is worthless. That
// scenario turns out to be impossible today, and the check is written down here because "we looked
// and there was nothing to fix" is worth exactly as much as a fix, and lasts only as long as
// somebody can see it was ever true:
//
//   - `GET /api/me/profile` has no `runtimeCaching` rule in `vite.config.ts`. The service worker
//     caches map tiles and contact photos, and nothing else from `/api/`.
//   - `profile.store` holds the response in Pinia state only. It is not persisted, and the store
//     already says why for `confirmationRequired`: a localStorage copy would let a reinstall skip
//     the step, or re-ask a member who already confirmed, possibly mid-event (PRD 005 §11).
//   - `profileStorage` scopes the *contacts directory* and favourites per profile. Neither carries
//     a contact number — `.rules` forbids one on any contacts surface, and
//     `guardiantripwire_test.go` enforces that end.
//
// So the invariant to protect is not "blank the cached number" but "there is no cached number".
// That is a stronger guarantee and a cheaper one — and the natural way to break it is a well-meant
// change: caching the profile so the app has a name to show offline. Whoever writes that will land
// on this test, which is the point.

const SRC = fileURLToPath(new URL('../', import.meta.url))
const VITE_CONFIG = fileURLToPath(new URL('../../vite.config.ts', import.meta.url))

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

// Comments are stripped before scanning. Without this the test trips over prose *about* not
// persisting things — including the note in `profile.store` explaining why a localStorage copy of
// `confirmationRequired` would be wrong, which is the opposite of a violation.
function code(source: string): string {
  return source.replace(/\/\*[\s\S]*?\*\//g, '').replace(/^\s*(\/\/|\*).*$/gm, '')
}

const PERSISTENCE = /localStorage|sessionStorage|indexedDB|caches\.open|profileKey\(/

describe('the profile response', () => {
  it('is not added to a service worker cache', () => {
    const config = readFileSync(VITE_CONFIG, 'utf8')

    // Every runtimeCaching entry's urlPattern, as written. A rule matching /api/me/profile would
    // put a contact number in a Cache API bucket that outlives the change which blanked it
    // server-side.
    const patterns = config.match(/urlPattern:[\s\S]*?(?=\n\s{12}handler)/g) ?? []
    expect(patterns.length).toBeGreaterThan(0)
    for (const pattern of patterns) {
      expect(code(pattern)).not.toMatch(/\/api\/me|profile/)
    }
  })

  it('is not written to device storage by the store that holds it', () => {
    // The profile store is the only thing holding these details, and it must keep them in memory.
    // A `setItem` here is the shortest path from "show the name offline" to "a stale contact number
    // a member can verify".
    const store = code(readFileSync(`${SRC}stores/profile.store.ts`, 'utf8'))
    expect(store).not.toMatch(PERSISTENCE)
  })

  it('does not have its contact number persisted anywhere', () => {
    // The half that catches the accident: somebody caching the number from a *different* file — a
    // composable, a view, an offline helper — rather than from the store. Scoped to files that
    // actually touch the contact number, so an unrelated localStorage write elsewhere is not a
    // false positive.
    const offenders: string[] = []
    for (const file of sourceFiles(SRC)) {
      const source = code(readFileSync(file, 'utf8'))
      if (!/phoneParent|phone_parent/.test(source)) continue
      if (PERSISTENCE.test(source)) offenders.push(file.replace(SRC, ''))
    }
    expect(offenders).toEqual([])
  })
})
