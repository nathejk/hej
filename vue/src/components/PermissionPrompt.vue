<script setup lang="ts">
import type { Component } from 'vue'
import type { RouteLocationRaw } from 'vue-router'
import { RouterLink } from 'vue-router'

// Soft in-app pre-prompt shown before the native permission dialog, so a
// decline doesn't permanently burn the browser permission. The parent view
// decides when to show it (contextually) and what happens on accept/dismiss.
//
// `moreTo`/`moreLabel` are optional: a prompt that asks for something it cannot fully
// explain in two lines needs somewhere to point (task 085).
//
// Two presentations, one component (PRD 005 §7 — this PRD owns the API):
//
// - `compact` (the default) is the card used inline by the map's repair affordance
//   (PRD 002) and the profile page's status rows (PRD 003). Omitting the prop is the
//   previous behaviour exactly; those call sites are consumers of this API, not co-owners,
//   and were not edited.
// - `page` is the full-screen onboarding explanation shown before a native dialog in the
//   /welcome flow (task 131).
//
// It is a **presentation** switch and must stay one. The variant changes how the
// explanation is laid out, never what is asked or what happens on accept — if that starts
// to differ, the difference belongs in the calling step, not in a branch here.
//
// `blocked` covers the case task 101 shipped guidance for: the permission is already denied
// at OS level, so no dialog will ever appear again. An accept button there does nothing but
// make the app look broken, so the slot for the platform's own settings guidance replaces
// it.
withDefaults(
  defineProps<{
    title: string
    message: string
    cta: string
    icon?: Component
    moreTo?: RouteLocationRaw
    moreLabel?: string
    variant?: 'compact' | 'page'
    /**
     * Label for the dismiss action. Overridable rather than branched on `variant`,
     * because the same emit means "ikke nu" in a page full of other content and "spring
     * over" in a linear onboarding flow — and that is a copy decision belonging to the
     * caller.
     */
    dismissLabel?: string
    /** The permission is denied at OS level: show guidance, not a dead accept button. */
    blocked?: boolean
    /** Task 101's platform-specific settings guidance, shown when `blocked`. */
    blockedGuidance?: string
  }>(),
  { moreLabel: 'Læs mere', variant: 'compact', dismissLabel: 'Ikke nu' },
)
const emit = defineEmits<{ accept: []; dismiss: [] }>()
</script>

<template>
  <!--
    Full-screen onboarding variant. No page chrome and no --sat/--sab padding of its own:
    it renders inside the /welcome shell, which already provides both plus the progress
    indicator. A second safe-area inset here would show up as a stripe of dead space under
    the notch.
  -->
  <div v-if="variant === 'page'" class="flex h-full flex-col justify-center gap-6">
    <div class="flex flex-col items-center gap-4 text-center">
      <div
        v-if="icon"
        class="flex h-14 w-14 items-center justify-center rounded-2xl bg-slate-900 text-white"
      >
        <component :is="icon" class="h-7 w-7" aria-hidden="true" />
      </div>
      <h1 class="font-nathejk text-3xl tracking-wide">{{ title }}</h1>
      <p class="text-sm leading-relaxed text-slate-600">{{ message }}</p>
      <RouterLink
        v-if="moreTo"
        :to="moreTo"
        class="text-sm text-slate-500 underline underline-offset-2"
      >
        {{ moreLabel }}
      </RouterLink>
    </div>

    <div class="flex flex-col items-stretch gap-2">
      <!--
        Blocked at OS level: the dialog will not reappear, so the only useful thing on
        screen is how to undo it in settings (task 101). Offering "Tillad" here would be a
        button that provably cannot work.
      -->
      <p v-if="blocked" class="text-center text-sm leading-relaxed text-slate-500">
        {{ blockedGuidance }}
      </p>
      <button
        v-else
        type="button"
        class="rounded-lg bg-slate-900 px-4 py-3 font-medium text-white"
        @click="emit('accept')"
      >
        {{ cta }}
      </button>
      <button type="button" class="px-2 py-2 text-sm text-slate-500" @click="emit('dismiss')">
        {{ dismissLabel }}
      </button>
    </div>
  </div>

  <!-- Compact card: unchanged from before this variant existed. -->
  <div v-else class="rounded-xl border border-slate-200 bg-white p-4 shadow-xs">
    <div class="flex items-start gap-3">
      <component :is="icon" v-if="icon" class="mt-0.5 h-5 w-5 shrink-0 text-slate-500" aria-hidden="true" />
      <div class="flex-1">
        <p class="font-medium text-slate-800">{{ title }}</p>
        <p class="mt-1 text-sm text-slate-500">{{ message }}</p>
        <!--
          Optional link to the fuller explanation. The prompt has to be short enough to read
          on a phone in the dark, but the location prompt now asks for something bigger than
          it can describe in two lines — the route is recorded and sent to the organizers —
          so it must be able to point somewhere (task 085).
        -->
        <RouterLink
          v-if="moreTo"
          :to="moreTo"
          class="mt-1 inline-block text-sm text-slate-500 underline underline-offset-2"
        >
          {{ moreLabel }}
        </RouterLink>
        <div class="mt-3 flex items-center gap-2">
          <button
            type="button"
            class="rounded-lg bg-slate-900 px-3 py-1.5 text-sm font-medium text-white"
            @click="emit('accept')"
          >
            {{ cta }}
          </button>
          <button type="button" class="px-2 py-1.5 text-sm text-slate-500" @click="emit('dismiss')">
            {{ dismissLabel }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
