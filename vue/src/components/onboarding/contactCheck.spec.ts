import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { HttpError } from '@/helpers'
import { contactCheckFailure, useProfileStore } from '@/stores/profile.store'

// The contact-number check's client half (PRD 015, task 232).
//
// The component itself cannot be mounted here — this project has no jsdom and no @vue/test-utils —
// so the split is: the *rules* are tested as functions and store actions, and the component is held
// to the ones that can only be expressed structurally. That is the same approach
// `offlineIndicator.spec.ts` and `layout.spec.ts` take, and it is the honest one: a rule that lives
// in a template is asserted against the template's source rather than pretended about.

describe('contactCheckFailure', () => {
  it('reads the server’s attempt count off a 400', () => {
    const err = new HttpError(400, 'de to cifre passer ikke', {
      error: 'de to cifre passer ikke',
      attempts_remaining: 2,
      check_closed: false,
    })

    expect(contactCheckFailure(err)).toEqual({
      message: 'de to cifre passer ikke',
      attemptsRemaining: 2,
      checkClosed: false,
    })
  })

  it('reports the third failure as a closed check', () => {
    const err = new HttpError(400, 'vi spurgte tre gange', {
      error: 'vi spurgte tre gange',
      attempts_remaining: 0,
      check_closed: true,
    })

    expect(contactCheckFailure(err)?.checkClosed).toBe(true)
  })

  // Anything else must fall through to the caller's own message rather than being reported as a
  // failed attempt with a made-up count. Guessing the count here would recreate the client-side
  // counter the server-side rule exists to replace.
  it('returns null for anything that is not a structured attempt failure', () => {
    expect(contactCheckFailure(new HttpError(429, 'for mange forsøg'))).toBeNull()
    expect(contactCheckFailure(new HttpError(503, 'kan ikke gemmes lige nu'))).toBeNull()
    expect(contactCheckFailure(new HttpError(400, 'nummeret ser forkert ud'))).toBeNull()
    expect(contactCheckFailure(new HttpError(400, 'prose only', { error: 'prose only' }))).toBeNull()
    expect(contactCheckFailure(new Error('boom'))).toBeNull()
    expect(contactCheckFailure(null)).toBeNull()
  })
})

describe('skipContactCheck', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.restoreAllMocks()
  })

  it('records the give-up', async () => {
    const post = vi.fn().mockResolvedValue(null)
    const { fetchWrapper } = await import('@/helpers')
    vi.spyOn(fetchWrapper, 'post').mockImplementation(post)

    await useProfileStore().skipContactCheck()

    expect(post).toHaveBeenCalledWith('/api/me/profile/skip', {})
  })

  // The one that matters. Login is the only mandatory step, so nothing here may stand between the
  // member and the app: no signal in a forest must not turn giving up into a dead end. The outcome
  // is lost instead, which is acceptable because check-in asks anyone with no verified number.
  it('never throws, whatever the server or the network does', async () => {
    const { fetchWrapper } = await import('@/helpers')
    const profile = useProfileStore()

    for (const failure of [
      new HttpError(503, 'kan ikke gemmes lige nu'),
      new HttpError(409, 'ingen bekræftelse er nødvendig'),
      new HttpError(500, 'the server exploded'),
      new Error('no signal'),
    ]) {
      vi.spyOn(fetchWrapper, 'post').mockRejectedValue(failure)
      await expect(profile.skipContactCheck()).resolves.toBeUndefined()
    }
  })
})

// The rules that live in WelcomeStepConfirmProfile.vue's script and template.
describe('the contact-check step', () => {
  const source = readFileSync(
    fileURLToPath(new URL('./WelcomeStepConfirmProfile.vue', import.meta.url)),
    'utf8',
  )

  // The attempt count must come from the server on every failure. A local tally is cleared by a
  // reload, so the limit it enforces would not exist — and two counters would disagree about which
  // attempt this is, with the client's being the one a member can reset.
  it('does not count attempts itself', () => {
    expect(source).toContain('contactCheckFailure')
    expect(source).not.toMatch(/attemptsLeft\.value\s*(--|-=|\+\+)/)
    expect(source).not.toMatch(/attempts\s*\+\+/)
  })

  // Three misses end the step by emitting `skip`, not by showing an error. Nobody did anything
  // wrong, and the member goes into the app.
  it('emits skip when the server closes the check', () => {
    expect(source).toMatch(/checkClosed[\s\S]{0,700}emit\('skip'\)/)
  })

  // The server has already recorded the outcome on the third failure, so the exhaustion path must
  // not also call the skip endpoint — that would publish a second event for one give-up.
  it('does not record a second outcome on exhaustion', () => {
    const exhaustion = source.slice(source.indexOf('failed.checkClosed'))
    const nextEmit = exhaustion.indexOf("emit('skip')")
    expect(exhaustion.slice(0, nextEmit)).not.toContain('skipContactCheck')
  })

  // With no number to recall — unregistered, or blanked by the BFF because it was the member's own
  // (task 229) — the step opens in correction mode. Rendering the recall UI would show an empty
  // number followed by two crosses, which reads as a bug.
  it('opens in correction mode when there is no number to recall', () => {
    expect(source).toMatch(/ref<'confirm' \| 'correct'>\(\s*profile\.details\?\.phoneParent \?/)
  })

  // No copy anywhere may suggest the member's own number is an acceptable answer: that is the
  // record task 229 exists to reject, and suggesting it would recreate the problem by hand.
  it('never suggests the member’s own number', () => {
    expect(source).toContain('ikke dit eget')
    expect(source).not.toMatch(/dit eget nummer\b(?![\s\S]{0,40}ikke)/)
  })
})
