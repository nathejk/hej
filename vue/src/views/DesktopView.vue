<script setup lang="ts">
import { Monitor } from '@lucide/vue'

// The desktop placeholder (PRD 005 §4/§7). NOT the desktop website: building that is
// explicitly a non-goal of PRD 005 and belongs to its own PRD. All this page exists to do
// is stop a desktop visitor being dumped at a login form for an app they cannot use.
//
// **The isolation is the point.** This view imports nothing from the mobile app except the
// brand name — no shared layout, no shared components, no stores. That is deliberate: when
// the real desktop site arrives it should be able to replace this file wholesale without
// any chance of regressing the mobile flow. Do not "reuse" the app's header here.
//
// There is no login form, no nav and no participant data, which is also what keeps the page
// free of a data-exposure decision: nothing on it is not already public. PRD 005 §11
// settles that no role logs in on desktop, organizers included — if that changes it is a
// new PRD revisiting the gate, not a flag added here.
//
// Shell chrome is hidden for this route in App.vue; this view does not manage that, and
// does not use `fullBleed` (which only suppresses the header inside `showShell`).
import { APP_NAME } from '@/config/brand'
</script>

<template>
  <main class="flex min-h-full w-full items-center justify-center px-6 py-16">
    <div class="flex max-w-md flex-col items-center gap-4 text-center">
      <div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-slate-900 text-white">
        <Monitor class="h-7 w-7" aria-hidden="true" />
      </div>
      <h1 class="font-nathejk text-4xl tracking-wide">{{ APP_NAME }}</h1>
      <p class="text-base leading-relaxed text-slate-600">
        {{ APP_NAME }} er en app, du bruger på din telefon under løbet — kort, kontakter,
        reglement og beskeder fra Nathejk. Åbn siden på din telefon, og læg den på
        hjemmeskærmen.
      </p>
    </div>
  </main>
</template>
