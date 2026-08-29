import { defineStore } from 'pinia'

import { useLocationStore } from '@/stores/location.store'
import { useNotificationsStore } from '@/stores/notifications.store'
import { useProfileStore } from '@/stores/profile.store'
import { useSessionStore } from '@/stores/session.store'

// The step machine behind /welcome, plus the per-device "onboarding complete" flag the
// router gate reads (PRD 005 §8).
//
// Two kinds of state, and keeping them apart is the point:
//
// - **Per-device**, in localStorage under `hej.onboarding.*`: the completion flag.
// - **Per-user**, from the BFF: profile confirmation, via `confirmation_required` on
//   GET /api/me/profile. Never stored client-side (PRD 005 §6) — it has to survive
//   reinstalls, new devices and cleared site data, and a client-side copy would let a
//   reinstall silently skip the step or a stale flag re-ask someone who already confirmed.

export type OnboardingStepId =
  | 'login'
  | 'confirm-profile'
  | 'portrait'
  | 'location'
  | 'notifications'

/** Everything the applicability and settled predicates are allowed to look at. */
interface StepContext {
  authenticated: boolean
  /** From the BFF; false while the field is absent, so the step simply does not apply. */
  confirmationRequired: boolean
  isSpejder: boolean
  hasPhoto: boolean
  locationSettled: boolean
  notificationsSettled: boolean
  skipped: OnboardingStepId[]
}

interface StepDescriptor {
  id: OnboardingStepId
  /** Danish label for the progress indicator. */
  label: string
  /** Login is the only mandatory step: nothing else may block the flow (PRD 005 §6). */
  mandatory?: boolean
  /** Does this step apply to this user at all? */
  applies: (ctx: StepContext) => boolean
  /** Is the question this step asks already answered? */
  settled: (ctx: StepContext) => boolean
}

// The canonical sequence, as data.
//
// Deliberately a declarative array rather than if/else control flow, for two reasons:
// PRD 009's offline-sync step and PRD 010's vehicle step are *slots* that must be absent
// until those PRDs are approved and addable without touching this machine's logic; and the
// step count has already drifted once between PRD 005 §5 and §6, which is what happens
// when a sequence is implied by code instead of written down in one place.
//
// Slots deliberately NOT present yet:
//   - `vehicle` — bandit/gøgler/crew only, owned by **PRD 010** (unapproved). Sits after
//     `portrait` and before `location`: it is another "about you" question rather than a
//     device prompt.
//   - `offline-sync` — first sync, owned by **PRD 009** (unapproved). Sits last.
const STEPS: StepDescriptor[] = [
  {
    id: 'login',
    label: 'Log ind',
    mandatory: true,
    applies: () => true,
    settled: (ctx) => ctx.authenticated,
  },
  {
    id: 'confirm-profile',
    label: 'Bekræft oplysninger',
    // Spejder only: `PhoneParent` exists on no other population, so for a bandit, gøgler
    // or crew member there is no guardian number to confirm and the step must not render
    // an empty field as though data were missing (PRD 005 §6).
    //
    // `confirmationRequired` is server-derived from "has verified" OR "has started the
    // event", so the "already started" skip rule lives on the BFF and is not duplicated
    // here.
    applies: (ctx) => ctx.isSpejder && ctx.confirmationRequired,
    settled: (ctx) => !ctx.confirmationRequired,
  },
  {
    id: 'portrait',
    label: 'Portræt',
    // Independent of confirmation, on purpose. A member who skipped confirmation because
    // they already started the event and has no portrait is precisely the person
    // personnel will fail to identify at 03:00, so skipping one must never skip the other
    // (PRD 005 §11, 2026-08-30).
    applies: (ctx) => !ctx.hasPhoto,
    settled: (ctx) => ctx.hasPhoto,
  },
  {
    id: 'location',
    label: 'Placering',
    applies: () => true,
    settled: (ctx) => ctx.locationSettled,
  },
  {
    id: 'notifications',
    label: 'Beskeder',
    applies: () => true,
    settled: (ctx) => ctx.notificationsSettled,
  },
]

const COMPLETE_KEY = 'hej.onboarding.complete'

// Wrapped, like every other localStorage access in the app: Safari throws in some privacy
// modes, and the router guard reads this on every navigation, so an exception here would
// white-screen the app (task 090's lesson).
function readComplete(): boolean {
  try {
    return localStorage.getItem(COMPLETE_KEY) === '1'
  } catch {
    return false
  }
}

function writeComplete(done: boolean) {
  try {
    if (done) localStorage.setItem(COMPLETE_KEY, '1')
    else localStorage.removeItem(COMPLETE_KEY)
  } catch {
    // Costs a repeated onboarding on the next cold start, nothing worse. Every step is
    // idempotent and settled steps are skipped, so the user sees at most a flash of the
    // final screen rather than the whole flow again.
  }
}

export const useOnboardingStore = defineStore('onboarding', {
  state: () => ({
    // Per-device completion. Read eagerly: the router gate needs it synchronously on the
    // first navigation.
    complete: readComplete(),
    // Steps the user chose to skip, in memory only and per session.
    //
    // Not persisted, and not the same thing as a step cursor: it exists so a skippable
    // step does not immediately re-present itself for the rest of *this* flow. Persisting
    // it would turn "not now" into "never", which is exactly the one-shot behaviour
    // PRD 005 §11 rejected for the portrait.
    skipped: [] as OnboardingStepId[],
  }),
  getters: {
    /**
     * The live inputs to the step predicates.
     *
     * Everything is **derived** from state the app already holds rather than from a
     * persisted step index. That is what makes the flow resumable in the honest sense: a
     * user who kills the app mid-flow, or grants location in iOS Settings instead of in
     * the dialog, resumes at the first genuinely unsettled step. A cursor is a second
     * source of truth for facts the platform already answers, and it drifts — re-asking
     * for a permission the OS has granted, or skipping one that was revoked outside the
     * app entirely.
     */
    context(state): StepContext {
      const session = useSessionStore()
      const profile = useProfileStore()
      const location = useLocationStore()
      const notifications = useNotificationsStore()

      return {
        authenticated: session.isAuthenticated,
        confirmationRequired: profile.confirmationRequired,
        isSpejder: session.role === 'spejder',
        hasPhoto: profile.hasPhoto,
        // `location.store`'s resolved value, deliberately not a fresh
        // `navigator.permissions` query: WebKit answers `prompt` for a *granted*
        // geolocation permission (see that store's comment), so re-querying here would
        // make the step machine disagree with the map about whether location works.
        locationSettled:
          location.permission === 'granted' ||
          location.permission === 'denied' ||
          location.permission === 'unavailable',
        notificationsSettled:
          notifications.permission === 'granted' ||
          notifications.permission === 'denied' ||
          notifications.permission === 'unavailable',
        skipped: state.skipped,
      }
    },

    /** The steps that apply to this user, in order — what the progress indicator shows. */
    steps(): { id: OnboardingStepId; label: string; settled: boolean }[] {
      const ctx = this.context
      return STEPS.filter((step) => step.applies(ctx)).map((step) => ({
        id: step.id,
        label: step.label,
        settled: step.settled(ctx),
      }))
    },

    /** The first applicable step that is neither settled nor skipped, or null when done. */
    currentStep(): OnboardingStepId | null {
      const ctx = this.context
      const next = STEPS.find(
        (step) => step.applies(ctx) && !step.settled(ctx) && !ctx.skipped.includes(step.id),
      )
      return next?.id ?? null
    },

    /**
     * True when a mandatory step is unsettled — i.e. login.
     *
     * Only login blocks. A declined permission, a skipped portrait or a failed profile
     * confirmation must never keep a participant out of a safety app (PRD 005 §6).
     */
    blocked(): boolean {
      const ctx = this.context
      return STEPS.some((step) => step.mandatory && step.applies(ctx) && !step.settled(ctx))
    },
  },
  actions: {
    /** "Not now" for a skippable step: it steps aside for this flow, and nothing more. */
    skip(id: OnboardingStepId) {
      if (!this.skipped.includes(id)) this.skipped.push(id)
    },
    markComplete() {
      this.complete = true
      writeComplete(true)
    },
    /** Used by the dev/QA override (task 139) and by sign-out. */
    reset() {
      this.complete = false
      this.skipped = []
      writeComplete(false)
    },
  },
})
