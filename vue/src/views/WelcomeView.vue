<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import WelcomeStepLogin from '@/components/onboarding/WelcomeStepLogin.vue'
import WelcomeStepConfirmProfile from '@/components/onboarding/WelcomeStepConfirmProfile.vue'
import WelcomeStepConfirmProblem from '@/components/onboarding/WelcomeStepConfirmProblem.vue'
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

// The confirmation step's "wrong number / I don't know it" screen (task 128) is a detour
// *within* a step rather than a step of its own: it is not a question every user is asked, and
// giving it a slot in the sequence would make the progress count wrong for everyone else.
const reportingProblem = ref(false)

const current = computed(() => onboarding.currentStep)

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
watch(
  current,
  async (step) => {
    if (step !== null) return
    if (!session.isAuthenticated) return
    onboarding.markComplete()
    await router.replace({ path: '/' })
  },
  { immediate: true },
)

function onStepDone() {
  // Nothing to advance: the store derives the next step from state the step itself changed
  // (a session, a granted permission, an uploaded portrait). This exists so a step can say
  // "I'm finished" without knowing what follows.
  reportingProblem.value = false
}

function onSkip(id: OnboardingStepId) {
  onboarding.skip(id)
}
</script>

<template>
  <main
    class="mx-auto flex h-full w-full max-w-sm flex-col gap-6 px-6"
    style="padding-top: var(--sat); padding-bottom: var(--sab)"
  >
    <header class="flex flex-col gap-3 pt-8">
      <h1 class="font-nathejk text-center text-2xl tracking-wide">{{ APP_NAME }}</h1>
      <Progress v-if="steps.length > 1" :model-value="percent" aria-label="Fremgang" />
      <p v-if="current" class="text-center text-xs text-slate-400">
        Trin {{ currentIndex + 1 }} af {{ steps.length }}
      </p>
    </header>

    <div class="flex flex-1 flex-col justify-center pb-8">
      <!-- The confirmation detour, when the member says the number is wrong or unknown. -->
      <WelcomeStepConfirmProblem
        v-if="current === 'confirm-profile' && reportingProblem"
        @done="onStepDone"
        @back="reportingProblem = false"
      />

      <component
        v-else-if="current"
        :is="stepComponents[current]"
        @done="onStepDone"
        @skip="onSkip(current)"
        @report="reportingProblem = true"
      />
    </div>
  </main>
</template>
