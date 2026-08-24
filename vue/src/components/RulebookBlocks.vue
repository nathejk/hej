<script setup lang="ts">
import { AlertTriangle, Info, Phone } from '@lucide/vue'
import type { RulebookBlock } from '@/config/rulebook'

// Renders the rulebook's structured content blocks. Kept separate from the view
// so the intro (always visible) and the collapsible sections share one renderer.
defineProps<{ blocks: RulebookBlock[] }>()
</script>

<template>
  <div class="space-y-4 text-[15px] leading-relaxed text-slate-700">
    <template v-for="(block, i) in blocks" :key="i">
      <p v-if="block.kind === 'text'">{{ block.text }}</p>

      <h3
        v-else-if="block.kind === 'subheading'"
        class="pt-2 text-sm font-semibold tracking-wide text-slate-900 uppercase"
      >
        {{ block.text }}
      </h3>

      <div v-else-if="block.kind === 'list'" class="space-y-2">
        <p v-if="block.lead">{{ block.lead }}</p>
        <ul class="space-y-2">
          <li v-for="item in block.items" :key="item" class="flex gap-3">
            <span
              class="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-slate-400"
              aria-hidden="true"
            />
            <span>{{ item }}</span>
          </li>
        </ul>
      </div>

      <aside
        v-else-if="block.kind === 'callout'"
        class="flex gap-3 rounded-lg border p-3"
        :class="
          block.tone === 'warning'
            ? 'border-amber-300 bg-amber-50 text-amber-900'
            : 'border-slate-200 bg-slate-50 text-slate-700'
        "
      >
        <component
          :is="block.tone === 'warning' ? AlertTriangle : Info"
          class="mt-0.5 h-5 w-5 shrink-0"
          :class="block.tone === 'warning' ? 'text-amber-600' : 'text-slate-500'"
          aria-hidden="true"
        />
        <div class="space-y-1">
          <p v-if="block.title" class="font-semibold">{{ block.title }}</p>
          <p>{{ block.text }}</p>
        </div>
      </aside>

      <aside
        v-else-if="block.kind === 'phone'"
        class="space-y-3 rounded-lg border border-slate-200 bg-slate-50 p-3"
      >
        <p class="font-semibold text-slate-900">{{ block.label }}</p>
        <p>{{ block.text }}</p>
        <!-- Tap-to-call rather than plain text: this is read on a phone, often
             at night, and the number must be dialable in one tap. -->
        <a
          :href="`tel:${block.phone}`"
          class="flex min-h-11 items-center justify-center gap-2 rounded-md bg-slate-900 px-4 font-semibold text-white"
        >
          <Phone class="h-4 w-4" aria-hidden="true" />
          {{ block.display }}
        </a>
      </aside>
    </template>
  </div>
</template>
