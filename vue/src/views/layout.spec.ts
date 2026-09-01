import { readdirSync, readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

// Layout invariants of the app shell (task 149's precedent: assert the structural cause, not the
// visible symptom).
//
// # Why negative horizontal margins are the rule here
//
// `main` in App.vue carries **no padding** — every view supplies its own (`ProfileView` uses
// `px-4 pt-4`, and so on). So a view using `-mx-4` to "bleed to the edges" is pulling against
// padding that does not exist, and ends up 2rem wider than the viewport. The result is a pane the
// user can drag sideways, which is what happened to the contacts list (2026-09-01) and is easy to
// reintroduce because the class looks harmless and the effect is invisible on a wide desktop
// window.
//
// A vertical negative margin is not the same hazard — there is no horizontal scrollport to
// overflow — so this only forbids the horizontal ones.

const VIEWS_DIR = fileURLToPath(new URL('.', import.meta.url))

function viewFiles(): string[] {
  return readdirSync(VIEWS_DIR).filter((f) => f.endsWith('.vue'))
}

// Comments are stripped before scanning, which is not incidental: the fix for this very bug
// documents the wrong class in a comment explaining why not to use it, and a naive scan flags that
// as the offence it warns against. A guard that fires on its own documentation trains people to
// delete the documentation.
function sourceWithoutComments(file: string): string {
  return readFileSync(`${VIEWS_DIR}${file}`, 'utf8')
    .replace(/<!--[\s\S]*?-->/g, '') // template comments
    .replace(/\/\*[\s\S]*?\*\//g, '') // block comments
    .replace(/^\s*\/\/.*$/gm, '') // whole-line // comments
}

describe('view layout', () => {
  it('has views to check', () => {
    // Guards the guard: a change to the directory layout must not turn this into a test that
    // silently examines nothing.
    expect(viewFiles().length).toBeGreaterThan(5)
  })

  it('no view uses a negative horizontal margin', () => {
    const offenders: string[] = []

    for (const file of viewFiles()) {
      const source = sourceWithoutComments(file)
      // Class-attribute usage only: `-mx-4`, `-ml-2`, `-mr-px` and their responsive variants.
      const match = source.match(/(?:^|[\s"'])-m[xlr]-[\w[\]./]+/g)
      if (match) offenders.push(`${file}: ${[...new Set(match.map((m) => m.trim()))].join(', ')}`)
    }

    expect(
      offenders,
      'A view cannot bleed sideways with a negative margin: App.vue\'s <main> has no padding to ' +
        'pull against, so this makes the page wider than the viewport and draggable sideways. ' +
        'Use full width instead.',
    ).toEqual([])
  })

  // The paired half of the contacts fix, kept honest: if a view needs to contain a wide child, it
  // must use `overflow-x-clip` and not `overflow-x-hidden`. When one axis is `hidden` and the other
  // `visible`, CSS computes the visible one to `auto` — which silently turns the view into its own
  // scroll container and breaks any `sticky` header inside it, because sticky then resolves against
  // a container that never scrolls.
  it('no view pairs overflow-x-hidden with a sticky child', () => {
    const offenders: string[] = []

    for (const file of viewFiles()) {
      const source = sourceWithoutComments(file)
      if (source.includes('overflow-x-hidden') && /(?:^|[\s"'])sticky(?:[\s"']|$)/m.test(source)) {
        offenders.push(file)
      }
    }

    expect(
      offenders,
      'overflow-x-hidden makes the other axis compute to auto, so this view became a scroll ' +
        'container and its sticky element stopped sticking. Use overflow-x-clip.',
    ).toEqual([])
  })
})
