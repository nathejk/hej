<script setup lang="ts">
import type { Component } from 'vue'

// One capability's status on this device (PRD 003 §6): icon, label, status text, and
// either an action or guidance.
//
// Same visual language as PermissionPrompt.vue, in a compact row — deliberately not a
// fork of it: that component *asks* for a permission, this one *reports* on it, and
// merging the two would give us a card that sometimes has buttons and sometimes prose.
//
// `status` is always text. Colour alone would fail the accessibility requirement, and a
// bare green dot also fails anyone reading this in bright sun or at 04:00.
defineProps<{
  icon: Component
  label: string
  /** Short state, in Danish: "Til", "Fra", "Ikke understøttet". */
  status: string
  /** Longer explanation, or the platform guidance when the permission is blocked. */
  detail?: string
  /** Label for the action, when there is one. Omit for a read-only row. */
  action?: string
  busy?: boolean
}>()

const emit = defineEmits<{ act: [] }>()
</script>

<template>
  <div class="flex items-start gap-3 py-3">
    <component :is="icon" class="mt-0.5 h-5 w-5 shrink-0 text-slate-500" aria-hidden="true" />

    <div class="min-w-0 flex-1">
      <div class="flex items-baseline justify-between gap-3">
        <p class="text-sm font-medium text-slate-800">{{ label }}</p>
        <p class="shrink-0 text-sm text-slate-500">{{ status }}</p>
      </div>
      <p v-if="detail" class="mt-1 text-xs text-slate-500">{{ detail }}</p>

      <button
        v-if="action"
        type="button"
        class="mt-2 min-h-9 rounded-lg bg-slate-900 px-3 py-1.5 text-sm font-medium text-white disabled:opacity-50"
        :disabled="busy"
        @click="emit('act')"
      >
        {{ action }}
      </button>
    </div>
  </div>
</template>
