<script setup lang="ts">
import { BookOpen } from '@lucide/vue'
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import RulebookBlocks from '@/components/RulebookBlocks.vue'
import { rulebookIntro, rulebookSections } from '@/config/rulebook'

// The full ruleset is long, so the sections are collapsible (shadcn-vue
// Accordion, type="multiple") — a scout looking up one rule in the dark should
// not have to scroll past five others. The intro and the most important rule are
// always visible.
</script>

<template>
  <article class="mx-auto max-w-2xl px-4 pt-4 pb-8">
    <header class="flex items-start gap-3">
      <BookOpen class="mt-1 h-6 w-6 shrink-0 text-slate-400" aria-hidden="true" />
      <div>
        <p class="text-xs font-semibold tracking-wide text-slate-400 uppercase">Spejdere</p>
        <h1 class="font-nathejk text-3xl tracking-wide text-slate-900">Regler</h1>
      </div>
    </header>

    <RulebookBlocks :blocks="rulebookIntro" class="mt-4" />

    <Accordion type="multiple" class="mt-6" :default-value="[]">
      <AccordionItem v-for="section in rulebookSections" :id="section.id" :key="section.id" :value="section.id">
        <AccordionTrigger class="gap-3 hover:no-underline">
          <component :is="section.icon" class="mt-0.5 h-5 w-5 shrink-0 text-slate-400" aria-hidden="true" />
          <span class="flex-1">
            <span class="font-nathejk block text-lg tracking-wide text-slate-900">{{ section.title }}</span>
            <span class="mt-0.5 block text-sm font-normal text-slate-500">{{ section.summary }}</span>
          </span>
        </AccordionTrigger>
        <AccordionContent>
          <RulebookBlocks :blocks="section.blocks" />
        </AccordionContent>
      </AccordionItem>
    </Accordion>

    <p class="mt-8 text-center text-xs text-slate-400">
      {{ rulebookSections.length }} afsnit · Reglerne fastsættes af løbsledelsen
    </p>
  </article>
</template>
