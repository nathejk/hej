<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import WelcomeStepLogin from '@/components/onboarding/WelcomeStepLogin.vue'
import WelcomeStepConfirmProfile from '@/components/onboarding/WelcomeStepConfirmProfile.vue'
import WelcomeStepPortrait from '@/components/onboarding/WelcomeStepPortrait.vue'
import WelcomeStepLocation from '@/components/onboarding/WelcomeStepLocation.vue'
import WelcomeStepNotifications from '@/components/onboarding/WelcomeStepNotifications.vue'
import { Progress } from '@/components/ui/progress'
import { APP_NAME } from '@/config/brand'
import { useOnboardingStore, type OnboardingStepId } from '@/stores/onboarding.store'
import { useProfileStore } from '@/stores/profile.store'
import { useSessionStore } from '@/stores/session.store'

// The onboarding shell (PRD 005 §7): it asks the store which step is current, renders that
// step, and shows how far through the flow the user is.
//
// **It owns no step logic.** Which step is next, which steps apply to this user, and whether
// onboarding is finished are all the store's answers, derived from session/permission/profile
// state rather than a persisted cursor (PRD 005 §8). If this file starts branching on role or
// permission state, that rule is broken and a user who kills the app mid-flow resumes in the
// wrong place.
//
// The step *order* is likewise the store's: PRD 010's `vehicle` slot and PRD 009's
// `offline-sync` slot are absent until those PRDs are approved, and adding them is a data
// change there, not a template change here.
//
// `fullBleed` is deliberately NOT set on this route. It is tempting — the map uses it to lose
// the header — but it only suppresses the header *inside* `showShell`, and `showShell` is
// already false on onboarding routes. Setting it would be a no-op that looks like the
// mechanism keeping the chrome away, and would then mislead whoever next changes `showShell`.

const onboarding = useOnboardingStore()
const profile = useProfileStore()
const session = useSessionStore()
const router = useRouter()

// The step components, keyed by the store's ids. A map rather than a chain of v-ifs so the
// sequence stays the store's business: a new step is an entry here plus a descriptor there.
const stepComponents = {
  login: WelcomeStepLogin,
  'confirm-profile': WelcomeStepConfirmProfile,
  portrait: WelcomeStepPortrait,
  location: WelcomeStepLocation,
  notifications: WelcomeStepNotifications,
} as const

// The step on screen.
//
// **Latched**, and that is the point of it. The store decides the *order* and what is still
// unsettled; this decides what the user is looking at right now, and a mounted step is never
// yanked away because state resolved underneath it.
//
// Without the latch the flow tore itself down mid-run (task 144): the notifications step syncs
// permission and subscription when it mounts, so an already-granted permission settled the step
// milliseconds after it appeared, `currentStep` went null, and onboarding "completed" — from the
// user's side, the photo step was the last thing they saw before landing in the app.
//
// `undefined` means "not decided yet", which is deliberately distinct from `null` ("nothing left
// to do"), because the second one completes onboarding and the first must not.
const shown = ref<OnboardingStepId | null | undefined>(undefined)

// Fill it in once at the start, and after each step hands back control. Never otherwise.
watch(
  () => onboarding.currentStep,
  (next) => {
    if (shown.value === undefined) shown.value = next
  },
  { immediate: true },
)

const current = computed(() => shown.value ?? null)

const steps = computed(() => onboarding.steps)

// Progress counts only the steps that apply to *this* user. A bandit who never sees the
// spejder-only confirmation step must not be told "trin 2 af 6" and then never shown step 2.
const currentIndex = computed(() => {
  const id = current.value
  if (!id) return steps.value.length
  const index = steps.value.findIndex((step) => step.id === id)
  return index === -1 ? 0 : index
})

const percent = computed(() => {
  const total = steps.value.length
  if (total === 0) return 100
  return Math.round((currentIndex.value / total) * 100)
})

// The profile is what `confirmation_required` and `hasPhoto` come from, so the flow cannot
// decide anything past login without it. Fetched once here rather than in the steps: two steps
// reading the same endpoint on mount would issue two requests for one answer.
async function loadProfile() {
  if (session.isAuthenticated) await profile.ensureLoaded()
}

onMounted(loadProfile)
watch(() => session.isAuthenticated, loadProfile)

// When there is no unsettled step left, onboarding is done. Marked complete (per device) and
// out to the app — routed by path so the shell does not need to know the first destination's
// name, the same way the login view used to.
//
// Watches the *latched* step, not the store's answer: completion is "the user finished the last
// step", not "the state happens to look finished".
watch(
  shown,
  async (step) => {
    if (step !== null) return
    if (!session.isAuthenticated) return
    onboarding.markComplete()
    await router.replace({ path: '/' })
  },
  { immediate: true },
)

// Hand control back to the machine: whatever the step just changed (a session, a granted
// permission, an uploaded portrait) is now in the stores, so ask what is left.
async function advance() {
  // The profile decides two of the remaining steps (`confirmation_required`, `has_photo`), and
  // straight after login it may still be in flight — deciding against an unloaded profile would
  // skip the confirmation step. `ensureLoaded()` awaits an in-flight request.
  await loadProfile()
  shown.value = onboarding.currentStep
}

async function onSkip(id: OnboardingStepId) {
  onboarding.skip(id)
  await advance()
}
</script>

<template>
  <!--
    Owns its own scrolling, for the same reason as the install wall (task 149): `main.css` sets
    `overflow: hidden` on html/body, scrolling belongs to the shell's `<main>`, and this route
    renders outside the shell — so without a container here the content cannot be reached.
    `Card` has `overflow-hidden`, so a shrunken flex child clips silently rather than spilling.

    It matters more here than on the wall, because of the keyboard. Focusing the phone number or
    the guardian number shrinks the usable area to a few hundred pixels — precisely when the member
    needs to see the field they are typing in *and* the button that submits it. A scroll container
    is also what lets the engine bring a focused input into view at all.

    `min-h-full` on the inner column, not `h-full`: centred when it fits, scrollable when it does
    not.
  -->
  <main class="h-full overflow-y-auto [overscroll-behavior:contain]">
    <div
      class="mx-auto flex min-h-full w-full max-w-sm flex-col gap-6 px-6"
      style="padding-top: var(--sat); padding-bottom: var(--sab)"
    >
      <header class="flex shrink-0 flex-col gap-3 pt-8">
        <h1 class="font-nathejk text-center text-2xl tracking-wide">{{ APP_NAME }}</h1>
        <Progress v-if="steps.length > 1" :model-value="percent" aria-label="Fremgang" />
        <p v-if="current" class="text-center text-xs text-slate-400">
          Trin {{ currentIndex + 1 }} af {{ steps.length }}
        </p>
      </header>

      <div class="flex flex-1 flex-col justify-center pb-8">
        <component
          v-if="current"
          :is="stepComponents[current]"
          @done="advance"
          @skip="current && onSkip(current)"
        />
      </div>
    </div>
  </main>
</template>
