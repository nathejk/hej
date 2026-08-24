<script setup lang="ts">
import type { Component } from 'vue'

// Soft in-app pre-prompt shown before the native permission dialog, so a
// decline doesn't permanently burn the browser permission. The parent view
// decides when to show it (contextually) and what happens on accept/dismiss.
defineProps<{ title: string; message: string; cta: string; icon?: Component }>()
const emit = defineEmits<{ accept: []; dismiss: [] }>()
</script>

<template>
  <div class="rounded-xl border border-slate-200 bg-white p-4 shadow-xs">
    <div class="flex items-start gap-3">
      <component :is="icon" v-if="icon" class="mt-0.5 h-5 w-5 shrink-0 text-slate-500" aria-hidden="true" />
      <div class="flex-1">
        <p class="font-medium text-slate-800">{{ title }}</p>
        <p class="mt-1 text-sm text-slate-500">{{ message }}</p>
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
