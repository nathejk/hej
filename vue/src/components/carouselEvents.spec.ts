import { readFileSync, readdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

// Listening for an event a component does not emit (task 316 follow-up, 2026-09-17).
//
// # The bug this exists to stop happening again
//
// `GlimtMediaStrip.vue` had `<Carousel @select="onSelect">`, and the handler **never ran**. The shadcn
// primitive's `CarouselEmits` declares exactly one event, `init-api`. Vue does not treat a binding for
// an undeclared event as an error: it falls through to the root element as a native DOM listener, and
// a `div` never emits `select`. So the dot marking the current photograph sat on the first slide for
// the life of every card.
//
// **Nothing in the toolchain could have caught it.** `vue-tsc` does not check event names against
// `defineEmits` for a binding that could plausibly be native, the build is happy, the linter is happy,
// and this suite runs in node with no DOM so no component is ever mounted. It was found by looking at
// a phone. That is the second bug in this feature found only by looking, which is what makes a
// source-level guard worth the awkwardness.
//
// # Scope, and why it is narrow
//
// This only checks components whose emits are **declared locally** in a `ui/*/interface.ts` as a
// `XxxEmits` interface. Most shadcn-vue primitives forward Reka UI's emits without redeclaring them,
// so their real event list is not readable from this repo — and guessing would produce false
// positives on legitimate bindings like `<DropdownMenuItem @select>`, which Reka does emit. A guard
// that cries wolf gets deleted, so it stays with what it can actually prove.

const UI_DIR = fileURLToPath(new URL('./ui/', import.meta.url))
const SRC = fileURLToPath(new URL('../', import.meta.url))

/** Source files that are ours, excluding the generated primitives and specs. */
function productFiles(dir: string): string[] {
  const out: string[] = []
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (entry.name === 'ui') continue
    const path = `${dir}${entry.name}`
    if (entry.isDirectory()) out.push(...productFiles(`${path}/`))
    else if (/\.vue$/.test(entry.name)) out.push(path)
  }
  return out
}

/**
 * Source with comments removed.
 *
 * Not optional here, and `profileNotCached.spec.ts` records why: **the fix for this very bug is
 * documented in a comment that quotes the wrong binding** to explain why not to use it. A naive scan
 * flags that as the offence it warns against, and a guard that fires on its own documentation trains
 * people to delete the documentation. Caught on the first run of this test.
 */
function code(source: string): string {
  return source
    .replace(/<!--[\s\S]*?-->/g, '') // template comments
    .replace(/\/\*[\s\S]*?\*\//g, '') // block comments
    .replace(/^\s*\/\/.*$/gm, '') // whole-line // comments
}

/**
 * Event names declared in a primitive's `interface.ts`.
 *
 * Reads call-signature declarations of the shape `(e: 'init-api', ...)`, which is how shadcn-vue
 * writes an emits interface.
 */
function declaredEmits(component: string): string[] | null {
  let source: string
  try {
    source = readFileSync(`${UI_DIR}${component}/interface.ts`, 'utf8')
  } catch {
    return null
  }
  const block = source.match(/export interface \w+Emits \{[\s\S]*?\n\}/)?.[0]
  if (!block) return null
  return [...block.matchAll(/\(\s*e:\s*'([^']+)'/g)].map((m) => m[1])
}

describe('the carousel primitive', () => {
  // Pins the contract `GlimtMediaStrip` now depends on. If a `shadcn-vue` regeneration renames or
  // drops `init-api`, the dots break again — and would break silently, exactly as before.
  it('emits init-api and nothing else', () => {
    const events = declaredEmits('carousel')
    expect(events, 'carousel/interface.ts no longer declares an Emits interface').not.toBeNull()
    expect(events).toEqual(['init-api'])
  })

  it('exposes selectedScrollSnap through the api type', () => {
    // The strip reads the current index from Embla directly. `CarouselApi` is the Embla instance, so
    // this asserts the export the strip imports still exists rather than the method itself.
    const index = readFileSync(`${UI_DIR}carousel/index.ts`, 'utf8')
    expect(index).toContain('CarouselApi')
  })
})

describe('no component listens for an event the carousel does not emit', () => {
  it('binds no undeclared event on <Carousel>', () => {
    const events = declaredEmits('carousel') ?? []
    const offenders: string[] = []

    for (const file of productFiles(SRC)) {
      const source = code(readFileSync(file, 'utf8'))
      // Every attribute binding on a `<Carousel ...>` open tag. Deliberately only the `Carousel`
      // element: its children (`CarouselItem` and friends) take no emits worth checking, and the
      // bug was on the parent.
      for (const tag of source.match(/<Carousel(?![A-Za-z])[^>]*>/g) ?? []) {
        for (const [, name] of tag.matchAll(/(?:@|v-on:)([a-z][a-z0-9-]*)/gi)) {
          // Vue accepts either casing for a hyphenated event name.
          const kebab = name.replace(/([a-z0-9])([A-Z])/g, '$1-$2').toLowerCase()
          if (!events.includes(kebab)) {
            offenders.push(`${file.replace(SRC, '')}: @${name}`)
          }
        }
      }
    }

    expect(
      offenders,
      'A binding for an event the component does not emit is not an error in Vue — it becomes a ' +
        'native DOM listener that never fires. The carousel emits only init-api; take the Embla ' +
        'instance from that and subscribe to its own events.',
    ).toEqual([])
  })

  // The positive half: the strip must actually be wired, or the dots are decoration again.
  it('wires the media strip through init-api', () => {
    const strip = readFileSync(`${SRC}components/glimt/GlimtMediaStrip.vue`, 'utf8')
    expect(strip, 'the strip no longer listens for init-api').toMatch(/@init-api|v-on:init-api/)
    expect(strip, 'the strip no longer reads the selected slide from Embla').toContain(
      'selectedScrollSnap',
    )
    // `reInit` matters because Embla re-initialises when the slide list changes, and without it the
    // dot would keep pointing at a slide that has moved.
    expect(strip, "the strip does not resubscribe on Embla's reInit").toContain('reInit')
  })
})
