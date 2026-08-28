<script setup lang="ts">
// Min profil (PRD 003): the details Nathejk holds about you, your portrait, and
// what this device is allowed to do.
//
// Reached from the user menu in the top bar (PRD 003 §7, decided 2026-08-28) — NOT
// from the bottom nav, so it is deliberately absent from config/navigation.ts.
//
// There is no sign-out button here on purpose: it lives in that same user menu, and
// PRD 005 requires exactly one sign-out action in the app.
import { onMounted } from 'vue'
import { useProfileStore } from '@/stores/profile.store'
import { ROLE_LABELS } from '@/config/roles'

const profile = useProfileStore()

onMounted(() => void profile.ensureLoaded())
</script>

<template>
  <div class="space-y-6 px-4 pt-4 pb-4">
    <header>
      <h1 class="font-nathejk text-2xl text-slate-900">Min profil</h1>
      <p v-if="profile.details" class="mt-1 text-sm text-slate-500">
        {{ profile.details.name }} · {{ ROLE_LABELS[profile.details.role] }}
        <template v-if="profile.details.team"> · {{ profile.details.team }}</template>
        <template v-else-if="profile.details.section"> · {{ profile.details.section }}</template>
      </p>
    </header>

    <!-- A failed fetch must still leave a usable page: the sections below are where the
         device-permission rows land (task 099), and those read local state that works
         perfectly well while the network does not. -->
    <p v-if="profile.error" class="rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-900">
      {{ profile.error }}
    </p>

    <section>
      <h2 class="font-nathejk text-lg text-slate-900">Mine oplysninger</h2>
      <p v-if="profile.loading" class="mt-2 text-sm text-slate-500">Henter …</p>
      <!-- Filled in by task 098. -->
    </section>

    <section>
      <h2 class="font-nathejk text-lg text-slate-900">På denne enhed</h2>
      <!-- Filled in by task 099. -->
    </section>
  </div>
</template>
