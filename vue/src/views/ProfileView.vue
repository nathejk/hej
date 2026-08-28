<script setup lang="ts">
// Min profil (PRD 003): the details Nathejk holds about you, your portrait, and
// what this device is allowed to do.
//
// Reached from the user menu in the top bar (PRD 003 §7, decided 2026-08-28) — NOT
// from the bottom nav, so it is deliberately absent from config/navigation.ts.
//
// There is no sign-out button here on purpose: it lives in that same user menu, and
// PRD 005 requires exactly one sign-out action in the app.
import { computed, onMounted } from 'vue'
import { useProfileStore } from '@/stores/profile.store'
import { ROLE_LABELS } from '@/config/roles'
import { formatPhone } from '@/helpers'

const profile = useProfileStore()

// Crew are seeded without an address and the app has no reason to show one, so an
// empty address is a normal state rather than missing data — but the row still says
// "Ikke registreret" rather than rendering a blank line.
const hasAddress = computed(() => {
  const d = profile.details
  return Boolean(d && (d.address || d.postalCode || d.city))
})

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

      <!-- A definition list, not a form: these fields are read-only (PRD 003 §6), and
           rendering them as disabled inputs would invite people to try to edit them. -->
      <dl
        v-if="profile.details"
        class="mt-2 divide-y divide-slate-100 rounded-xl border border-slate-200 bg-white px-4 shadow-xs"
      >
        <div class="flex items-baseline justify-between gap-4 py-3">
          <dt class="text-sm text-slate-500">Navn</dt>
          <dd class="text-right text-sm text-slate-900">{{ profile.details.name }}</dd>
        </div>

        <div class="flex items-baseline justify-between gap-4 py-3">
          <dt class="text-sm text-slate-500">Adresse</dt>
          <dd class="text-right text-sm text-slate-900">
            <template v-if="hasAddress">
              <span class="block">{{ profile.details.address }}</span>
              <span class="block">{{ profile.details.postalCode }} {{ profile.details.city }}</span>
            </template>
            <span v-else class="text-slate-400">Ikke registreret</span>
          </dd>
        </div>

        <div class="flex items-baseline justify-between gap-4 py-3">
          <dt class="text-sm text-slate-500">Telefon</dt>
          <dd class="text-right text-sm">
            <!-- href keeps the normalized number; only the label is grouped. -->
            <a
              v-if="profile.details.phone"
              :href="`tel:${profile.details.phone}`"
              class="text-slate-900 underline"
            >
              {{ formatPhone(profile.details.phone) }}
            </a>
            <span v-else class="text-slate-400">Ikke registreret</span>
          </dd>
        </div>

        <!-- Hidden entirely when phoneParent is null: that means this population has
             no guardian number at all (bandit, crew, gøgler), and showing them an
             empty row would imply they are missing something. An empty *string* is
             the different case — expected, not registered — and does show. -->
        <div
          v-if="profile.details.phoneParent !== null"
          class="flex items-baseline justify-between gap-4 py-3"
        >
          <dt class="text-sm text-slate-500">Forælders telefon</dt>
          <dd class="text-right text-sm">
            <a
              v-if="profile.details.phoneParent"
              :href="`tel:${profile.details.phoneParent}`"
              class="text-slate-900 underline"
            >
              {{ formatPhone(profile.details.phoneParent) }}
            </a>
            <span v-else class="text-slate-400">Ikke registreret</span>
          </dd>
        </div>
      </dl>

      <!-- Deliberately names no contact channel: PRD 003 §11 "Editability" is still
           open, and inventing a phone number or an email here would be worse than
           saying who to ask. -->
      <p v-if="profile.details" class="mt-2 text-xs text-slate-500">
        Oplysningerne kommer fra jeres tilmelding og kan ikke rettes i appen. Er noget
        forkert, sig det til din leder eller til Nathejk i startområdet.
      </p>
    </section>

    <section>
      <h2 class="font-nathejk text-lg text-slate-900">På denne enhed</h2>
      <!-- Filled in by task 099. -->
    </section>
  </div>
</template>
