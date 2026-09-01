<script setup lang="ts">
// The candidate list for a phone number carrying several profiles (PRD 012, task 181).
//
// Shared by two surfaces that must not disagree about how a person is identified: the login chooser
// (PRD 006 / task 079) and the profile switcher. Two lists could drift, and this is the list somebody
// uses to prove which of two profiles is theirs — a name that reads the same on both is the point.
//
// # Why the second line matters
//
// The affiliation is not decoration. A first name alone is often not enough — two siblings, or a
// parent and child with similar names — and "Patrulje Ravnene" versus "Samarit" is usually the thing
// that makes the choice obvious. The BFF sends exactly one of team/section and omits it when empty,
// so this renders whichever arrives without branching on role.
//
// Knows nothing about *why* it is being shown: no step logic, no "Skift nummer" (that belongs to the
// login flow, where changing number is a meaningful action). It takes candidates and emits an id.
import { ChevronRight } from '@lucide/vue'

import type { ChoiceCandidate } from '@/stores/session.store'

defineProps<{
  candidates: ChoiceCandidate[]
  /** Both callers await a request on selection, so both need the list to stop accepting taps. */
  busy?: boolean
  /** Rendered on a dark surface by the switcher, on the light onboarding card by login. */
  dark?: boolean
}>()

const emit = defineEmits<{ choose: [userId: string] }>()
</script>

<template>
  <div class="flex flex-col gap-3">
    <button
      v-for="c in candidates"
      :key="c.user_id"
      type="button"
      :disabled="busy"
      class="flex items-center justify-between rounded-lg border px-4 py-3 text-left transition disabled:opacity-50"
      :class="dark ? 'border-slate-700 text-slate-100' : 'border-slate-300'"
      @click="emit('choose', c.user_id)"
    >
      <span>
        <span class="font-medium">{{ c.name }}</span>
        <span
          v-if="c.team || c.section"
          class="block text-sm"
          :class="dark ? 'text-slate-400' : 'text-slate-500'"
        >
          {{ c.team || c.section }}
        </span>
      </span>
      <ChevronRight class="h-4 w-4 shrink-0" :class="dark ? 'text-slate-500' : 'text-slate-400'" />
    </button>
  </div>
</template>
