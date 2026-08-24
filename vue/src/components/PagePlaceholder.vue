<script setup lang="ts">
import type { Component } from 'vue'

// Shared placeholder for skeleton pages. Real content lands in per-feature PRDs.
defineProps<{ title: string; icon?: Component }>()
</script>

<template>
  <section class="flex h-full flex-col items-center justify-center gap-3 p-6 text-center">
    <component :is="icon" v-if="icon" class="h-10 w-10 text-slate-400" />
    <h1 class="font-nathejk text-2xl tracking-wide text-slate-700">{{ title }}</h1>
    <p class="text-sm text-slate-400">Kommer snart.</p>
  </section>
</template>
