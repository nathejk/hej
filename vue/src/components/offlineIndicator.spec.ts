import { readFileSync, readdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

// There is exactly ONE global offline indicator (PRD 009 §6, task 188).
//
// # Why this is worth a structural test
//
// "You are offline" is the easiest banner in the world to add locally: a view fails a fetch, the
// author adds a friendly notice, and now two things on screen say the same thing in different
// words — or worse, one says offline while the other is already back. During this event a
// participant may be without signal for hours, so a duplicated notice is not a cosmetic problem;
// it is most of the screen.
//
// It follows `layout.spec.ts`'s precedent: assert the structural cause rather than the visible
// symptom, because the symptom only appears on a phone in a forest.
//
// # What this deliberately does NOT forbid
//
// Per-feature honesty about *data*, which PRD 009 §7 requires: an inline timestamp saying this
// list may be old, or PatrolLookup's "this needs signal, use the radio". That lookup is
// deliberately live-only (PRD 007), so telling a crew member "the app still works offline" while
// the thing they are doing cannot would be worse than saying nothing. Those messages are about a
// feature's data, not about connectivity, which is why the check below is narrowly about the
// global notice's own wording.

const SRC = fileURLToPath(new URL('../', import.meta.url))

function sourceFiles(dir: string): string[] {
  const out: string[] = []
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (entry.name === 'ui') continue // generated primitives
    const path = `${dir}${entry.name}`
    if (entry.isDirectory()) out.push(...sourceFiles(`${path}/`))
    else if (entry.name.endsWith('.vue')) out.push(path)
  }
  return out
}

describe('the global offline indicator', () => {
  // Exactly one exception, and it exists because the global notice *cannot* cover this case:
  // `OfflineNotice` only renders once we know who the user is, so on the login screen there is
  // nothing on screen at all. Login is also the one action that genuinely cannot work from cache,
  // so the wording there is about the attempt ("find somewhere with signal and try again") rather
  // than about the app being offline. Adding to this list should feel like a decision.
  const ALLOWED = ['components/OfflineNotice.vue', 'components/onboarding/WelcomeStepLogin.vue']

  it('is announced only by the global notice and the login step', () => {
    const offenders = sourceFiles(SRC)
      .filter((file) => /Ingen forbindelse/.test(readFileSync(file, 'utf8')))
      .map((file) => file.replace(SRC, ''))
      .sort()

    expect(offenders).toEqual([...ALLOWED].sort())
  })

  it('sends the user to the readiness view rather than explaining itself', () => {
    const source = readFileSync(`${SRC}components/OfflineNotice.vue`, 'utf8')
    expect(source).toMatch(/RouterLink[\s\S]*?\/profil/)
  })

  // One line, in the document flow. An earlier three-line version was honest and unusable, and an
  // overlay either collides with UpdatePrompt or covers the map — for hours at a time.
  it('stays a single Alert with a single title', () => {
    const source = readFileSync(`${SRC}components/OfflineNotice.vue`, 'utf8')
    expect(source.match(/<AlertTitle>/g) ?? []).toHaveLength(1)
    expect(source).not.toMatch(/<AlertDescription>/)
  })
})
