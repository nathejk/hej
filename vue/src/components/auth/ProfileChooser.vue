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
import { computed } from 'vue'
import { ChevronRight } from '@lucide/vue'

import { candidateRows } from '@/helpers/profileCandidates'
import type { ChoiceCandidate } from '@/stores/session.store'

const props = defineProps<{
  candidates: ChoiceCandidate[]
  /** Both callers await a request on selection, so both need the list to stop accepting taps. */
  busy?: boolean
  /** Rendered on a dark surface by the switcher, on the light onboarding card by login. */
  dark?: boolean
}>()

const emit = defineEmits<{ choose: [userId: string] }>()

// The display rules — which discriminator to show, and numbering rows that are still identical —
// live in helpers/profileCandidates so they can be tested without mounting anything.
const rows = computed(() => candidateRows(props.candidates))
</script>

<template>
  <div class="flex flex-col gap-3">
    <button
      v-for="row in rows"
      :key="row.userId"
      type="button"
      :disabled="busy"
      class="flex items-center justify-between rounded-lg border px-4 py-3 text-left transition disabled:opacity-50"
      :class="dark ? 'border-slate-700 text-slate-100' : 'border-slate-300'"
      @click="emit('choose', row.userId)"
    >
      <span>
        <span class="font-medium">{{ row.name }}</span>
        <span
          v-if="row.subtitle"
          class="block text-sm"
          :class="dark ? 'text-slate-400' : 'text-slate-500'"
        >
          {{ row.subtitle }}
        </span>
      </span>
      <ChevronRight class="h-4 w-4 shrink-0" :class="dark ? 'text-slate-500' : 'text-slate-400'" />
    </button>
  </div>
</template>
