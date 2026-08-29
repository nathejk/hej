import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import { useLocationStore } from '@/stores/location.store'
import { useNotificationsStore } from '@/stores/notifications.store'
import { useOnboardingStore } from '@/stores/onboarding.store'
import { useProfileStore } from '@/stores/profile.store'
import { useSessionStore } from '@/stores/session.store'

// The step machine is derived state, so these tests set up the *world* (session,
// permissions, profile) and assert which step falls out — never a step index, because
// there isn't one.
function world(overrides: {
  authenticated?: boolean
  role?: 'spejder' | 'bandit'
  confirmationRequired?: boolean
  hasPhoto?: boolean
  location?: 'unknown' | 'granted' | 'denied'
  notifications?: 'unknown' | 'granted' | 'denied'
}) {
  const session = useSessionStore()
  const profile = useProfileStore()
  const location = useLocationStore()
  const notifications = useNotificationsStore()

  session.user =
    overrides.authenticated === false
      ? null
      : { userId: 'u1', role: overrides.role ?? 'spejder' }
  profile.confirmationRequired = overrides.confirmationRequired ?? false
  profile.hasPhoto = overrides.hasPhoto ?? true
  location.permission = overrides.location ?? 'granted'
  notifications.permission = overrides.notifications ?? 'granted'
}

describe('onboarding.store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('starts at login when nobody is signed in', () => {
    world({ authenticated: false })
    const onboarding = useOnboardingStore()
    expect(onboarding.currentStep).toBe('login')
    // Login is the only step allowed to block.
    expect(onboarding.blocked).toBe(true)
  })

  it('is finished when every applicable step is settled', () => {
    world({})
    expect(useOnboardingStore().currentStep).toBe(null)
  })

  it('asks a spejder to confirm their profile when the BFF says so', () => {
    world({ confirmationRequired: true })
    expect(useOnboardingStore().currentStep).toBe('confirm-profile')
  })

  // PhoneParent exists only on spejder, so there is nothing for anyone else to confirm.
  it('never shows profile confirmation to a bandit', () => {
    world({ role: 'bandit', confirmationRequired: true })
    const onboarding = useOnboardingStore()
    expect(onboarding.currentStep).toBe(null)
    expect(onboarding.steps.map((s) => s.id)).not.toContain('confirm-profile')
  })

  // PRD 005 §11 (2026-08-30): the two are independent facts. Conflating them would remove
  // the portrait nudge from the cohort that needs it most.
  it('still asks for a portrait when profile confirmation does not apply', () => {
    world({ confirmationRequired: false, hasPhoto: false })
    expect(useOnboardingStore().currentStep).toBe('portrait')
  })

  it('walks portrait → location → notifications in order', () => {
    world({ hasPhoto: false, location: 'unknown', notifications: 'unknown' })
    const onboarding = useOnboardingStore()
    expect(onboarding.currentStep).toBe('portrait')

    useProfileStore().hasPhoto = true
    expect(onboarding.currentStep).toBe('location')

    useLocationStore().permission = 'denied'
    expect(onboarding.currentStep).toBe('notifications')

    useNotificationsStore().permission = 'denied'
    expect(onboarding.currentStep).toBe(null)
  })

  // The resumability property, stated as a test: nothing was recorded when the user left,
  // so nothing has to be replayed. A permission granted in iOS Settings — entirely outside
  // the app — simply settles the step.
  it('resumes at the first unsettled step with no in-app bookkeeping', () => {
    world({ hasPhoto: true, location: 'unknown', notifications: 'unknown' })
    const onboarding = useOnboardingStore()
    expect(onboarding.currentStep).toBe('location')

    // The user granted location outside the app and reopened. No cursor to reconcile.
    useLocationStore().permission = 'granted'
    expect(onboarding.currentStep).toBe('notifications')
  })

  it('a declined permission settles the step rather than blocking the flow', () => {
    world({ location: 'denied', notifications: 'denied' })
    const onboarding = useOnboardingStore()
    expect(onboarding.currentStep).toBe(null)
    expect(onboarding.blocked).toBe(false)
  })

  it('skip moves past a step for this flow without persisting a refusal', () => {
    world({ hasPhoto: false })
    const onboarding = useOnboardingStore()
    expect(onboarding.currentStep).toBe('portrait')

    onboarding.skip('portrait')
    expect(onboarding.currentStep).toBe(null)

    // A fresh store (a later launch) asks again — the nudge is not silenced permanently,
    // only quieted for the session (PRD 005 §11).
    setActivePinia(createPinia())
    world({ hasPhoto: false })
    expect(useOnboardingStore().currentStep).toBe('portrait')
  })

  // The slots are absent until their PRDs are approved. If either arrives without this
  // test being updated, that is the signal to re-read PRD 005 §6.
  it('does not contain the PRD 009 / PRD 010 slots yet', () => {
    world({ hasPhoto: false, confirmationRequired: true, location: 'unknown', notifications: 'unknown' })
    expect(useOnboardingStore().steps.map((s) => s.id)).toEqual([
      'login',
      'confirm-profile',
      'portrait',
      'location',
      'notifications',
    ])
  })
})
