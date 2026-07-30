<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { HeartHandshake } from 'lucide-vue-next'
import { fetchWrapper } from '@/helpers'

// Minimal home view for the skeleton. It pings the BFF healthcheck to prove the
// SPA ↔ BFF wiring (Vite proxies /api → the api container in dev). The real
// app shell (bottom nav, role-based routing) lands in later tasks.
// Icons come from Lucide (lucide-vue-next) per the repo convention in .rules.
type Health = { status: string; system_info: { environment: string } }

const status = ref<string>('checking…')

onMounted(async () => {
  try {
    const health = await fetchWrapper.get<Health>('/api/healthcheck')
    status.value = `${health.status} (${health.system_info.environment})`
  } catch (err) {
    status.value = err instanceof Error ? `unreachable: ${err.message}` : 'unreachable'
  }
})
</script>

<template>
  <main class="flex h-full flex-col items-center justify-center gap-4 p-6 text-center">
    <HeartHandshake class="h-12 w-12 text-slate-700" />
    <h1 class="text-3xl font-bold">Hej Nathejk</h1>
    <p class="text-slate-500">In-event companion app — skeleton.</p>
    <p class="text-sm">
      BFF healthcheck: <span class="font-mono">{{ status }}</span>
    </p>
  </main>
</template>
