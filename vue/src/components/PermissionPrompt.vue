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
withDefaults(
  defineProps<{
    title: string
    message: string
    cta: string
    icon?: Component
    moreTo?: RouteLocationRaw
    moreLabel?: string
  }>(),
  { moreLabel: 'Læs mere' },
)
const emit = defineEmits<{ accept: []; dismiss: [] }>()
</script>

<template>
  <div class="rounded-xl border border-slate-200 bg-white p-4 shadow-xs">
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
            Ikke nu
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
