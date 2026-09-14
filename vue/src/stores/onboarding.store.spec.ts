import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import { useLocationStore } from '@/stores/location.store'
import { useNotificationsStore } from '@/stores/notifications.store'
import { useOnboardingStore } from '@/stores/onboarding.store'
import { useProfileStore } from '@/stores/profile.store'
import { useSessionStore } from '@/stores/session.store'
import { useVehiclesStore } from '@/stores/vehicles.store'

// The step machine is derived state, so these tests set up the *world* (session,
// permissions, profile) and assert which step falls out — never a step index, because
// there isn't one.
function world(overrides: {
  authenticated?: boolean
  role?: 'spejder' | 'bandit' | 'crew' | 'samarit'
  confirmationRequired?: boolean
  hasPhoto?: boolean
  hasVehicle?: boolean
  location?: 'unknown' | 'granted' | 'denied'
  notifications?: 'unknown' | 'granted' | 'denied'
  subscribed?: boolean
}) {
  const session = useSessionStore()
  const profile = useProfileStore()
  const location = useLocationStore()
  const notifications = useNotificationsStore()
  const vehicles = useVehiclesStore()

  session.user =
    overrides.authenticated === false
      ? null
      : { userId: 'u1', role: overrides.role ?? 'spejder' }
  profile.confirmationRequired = overrides.confirmationRequired ?? false
  profile.hasPhoto = overrides.hasPhoto ?? true
  location.permission = overrides.location ?? 'granted'
  notifications.permission = overrides.notifications ?? 'granted'
  // Defaults to true so the existing cases keep meaning "notifications are done"; the tests
  // below that care about the distinction set it explicitly.
  notifications.subscribed = overrides.subscribed ?? true
  // Defaults to "already has one" for the same reason: it keeps every pre-existing case
  // meaning "nothing left to do". The vehicle cases below set it explicitly.
  vehicles.vehicles = overrides.hasVehicle ?? true ? [vehicleFixture()] : []
}

function vehicleFixture() {
  return {
    id: 'v1',
    licensePlate: 'DK+AB12345',
    brand: '',
    model: '',
    color: '',
    seatCount: 0,
    description: '',
  }
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

  // REGRESSION (task 144). Permission and subscription are independent: a member can have
  // granted notifications long ago and have no subscription registered with the BFF, in which
  // case nothing is ever delivered to them. Treating the grant alone as settled skipped the one
  // step whose job is to create that subscription — silently, and for exactly the people who
  // look most set up.
  it('still asks for notifications when permission is granted but nothing is subscribed', () => {
    world({ notifications: 'granted', subscribed: false })
    expect(useOnboardingStore().currentStep).toBe('notifications')
  })

  it('does not ask when a granted permission already has a subscription behind it', () => {
    world({ notifications: 'granted', subscribed: true })
    expect(useOnboardingStore().currentStep).toBe(null)
  })

  // Nothing further can be done in either state, so neither needs a subscription to settle.
  it('settles a denied or unsupported notification permission without a subscription', () => {
    world({ notifications: 'denied', subscribed: false })
    expect(useOnboardingStore().currentStep).toBe(null)
  })

  it('a declined permission settles the step rather than blocking the flow', () => {
    world({ location: 'denied', notifications: 'denied', subscribed: false })
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

  // PRD 009's slot is still absent. PRD 010's `vehicle` step landed in task 241 and now sits
  // between `portrait` and `location` — if the offline-sync slot ever arrives without this test
  // being updated, that is the signal to re-read PRD 005 §6.
  it('has the full sequence for a member with everything outstanding', () => {
    world({
      role: 'bandit',
      hasPhoto: false,
      hasVehicle: false,
      location: 'unknown',
      notifications: 'unknown',
    })
    expect(useOnboardingStore().steps.map((s) => s.id)).toEqual([
      'login',
      'portrait',
      'vehicle',
      'location',
      'notifications',
    ])
  })

  it('does not contain the PRD 009 offline-sync slot yet', () => {
    world({ hasPhoto: false, confirmationRequired: true, location: 'unknown', notifications: 'unknown' })
    expect(useOnboardingStore().steps.map((s) => s.id)).not.toContain('offline-sync')
  })

  // —— The vehicle step (PRD 010, task 241) ——

  it('asks a bandit with no vehicle about one, after the portrait', () => {
    world({ role: 'bandit', hasVehicle: false, hasPhoto: false })
    const onboarding = useOnboardingStore()

    // Order matters: it is an "about you" question, so it comes after the portrait and before
    // the device prompts.
    expect(onboarding.currentStep).toBe('portrait')
    onboarding.skip('portrait')
    expect(onboarding.currentStep).toBe('vehicle')
  })

  // The one hard rule of this step. A spejder is a minor who does not drive to the event, so
  // there is nothing to ask — and the step must be *absent*, not empty.
  it('never shows the vehicle step to a spejder', () => {
    world({ role: 'spejder', hasVehicle: false })
    const onboarding = useOnboardingStore()
    expect(onboarding.currentStep).toBe(null)
    expect(onboarding.steps.map((s) => s.id)).not.toContain('vehicle')
  })

  // Written as an exclusion rather than an allow-list, so a role nobody thought about when this
  // was built still gets asked. These two are crew roles that no `bandit | gøgler | crew` list
  // would have named.
  it('shows the vehicle step to every other role, not a named few', () => {
    for (const role of ['crew', 'samarit'] as const) {
      setActivePinia(createPinia())
      world({ role, hasVehicle: false })
      expect(useOnboardingStore().steps.map((s) => s.id)).toContain('vehicle')
    }
  })

  it('does not ask again once a vehicle is on file', () => {
    world({ role: 'bandit', hasVehicle: true })
    const onboarding = useOnboardingStore()
    expect(onboarding.currentStep).toBe(null)
    expect(onboarding.steps.map((s) => s.id)).not.toContain('vehicle')
  })

  // "No, I am not bringing a vehicle" is a skip, not a stored fact: it steps aside for this
  // flow and is asked again on a later launch, exactly like the portrait. Persisting it would
  // turn "not now" into "never" for a member who ends up borrowing a car.
  it('lets a declined vehicle question return on a later launch', () => {
    world({ role: 'bandit', hasVehicle: false })
    const onboarding = useOnboardingStore()
    expect(onboarding.currentStep).toBe('vehicle')

    onboarding.skip('vehicle')
    expect(onboarding.currentStep).toBe(null)

    setActivePinia(createPinia())
    world({ role: 'bandit', hasVehicle: false })
    expect(useOnboardingStore().currentStep).toBe('vehicle')
  })

  // Only login blocks. A member who will not talk about their car must still get into a safety
  // app.
  it('never blocks the flow on the vehicle step', () => {
    world({ role: 'bandit', hasVehicle: false })
    expect(useOnboardingStore().blocked).toBe(false)
  })
})
