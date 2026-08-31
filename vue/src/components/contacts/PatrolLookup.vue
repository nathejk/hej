<script setup lang="ts">
// "Slå patrulje op" — the crew-only patrol lookup (PRD 007 §7, task 168).
//
// The only path by which a spejder's details are reachable in this app. Everything about it is
// the opposite of the directory it sits next to, and each difference is load-bearing rather than
// stylistic:
//
//   - **Live, never cached.** Results live in this component's state for as long as the panel is
//     open and are discarded when it closes. Nothing is written to a store, to localStorage, or
//     to the service worker's caches. That is what keeps ~557 spejder thumbnails off crew
//     devices, and it is enforced at the BFF too (`Cache-Control: no-store`).
//   - **No recent-lookups list.** Tempting, trivial to add, and it would accumulate into exactly
//     the browsable index of minors' faces this design exists to avoid. If you are about to add
//     one, read PRD 007 §8 first.
//   - **No prefix search, no suggestions, no patrol picker.** The number must be typed in full.
//     Partial matching would turn one permitted question ("show me patrol 138") into an
//     enumeration tool ("show me every patrol starting with 1"). The BFF matches exactly for the
//     same reason.
//   - **A separate entry from the directory search.** One "smart" field that also accepted patrol
//     numbers would make them browsable by accident.
//
// With no signal this does not work, and it says so. That is an accepted cost: the samarit's
// fallback is the radio and HQ, which is how it works today.
import { computed, ref, watch } from 'vue'
import { Phone, Search, WifiOff, X } from '@lucide/vue'

import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Input } from '@/components/ui/input'
import { hasLeftRace, isInOurCare, memberStatusLabel } from '@/config/memberStatus'
import { HttpError, NetworkError, fetchWrapper, formatPhone } from '@/helpers'

interface PatrolMember {
  id: string
  name: string
  status?: string
  phone?: string
  hasPortrait: boolean
}

interface PatrolResponse {
  number: string
  members: PatrolMember[] | null
}

const open = ref(false)
const number = ref('')
const loading = ref(false)
// Not persisted anywhere on purpose — see the header. Closing the panel drops them.
const members = ref<PatrolMember[] | null>(null)
const shownNumber = ref('')
const notFound = ref(false)
const offline = ref(false)
const failed = ref(false)

// Closing clears everything. A panel that reopened with the previous patrol still in it would be
// a recent-lookups list with extra steps.
watch(open, (isOpen) => {
  if (!isOpen) reset()
})

function reset() {
  number.value = ''
  members.value = null
  shownNumber.value = ''
  notFound.value = false
  offline.value = false
  failed.value = false
  loading.value = false
}

const canSubmit = computed(() => number.value.trim().length > 0 && !loading.value)

async function submit() {
  const query = number.value.trim()
  if (!query || loading.value) return

  loading.value = true
  members.value = null
  notFound.value = false
  offline.value = false
  failed.value = false

  try {
    const data = await fetchWrapper.get<PatrolResponse>(
      `/api/contacts/patrols/${encodeURIComponent(query)}`,
    )
    members.value = data.members ?? []
    shownNumber.value = data.number || query
  } catch (err) {
    if (err instanceof NetworkError) {
      // The motivating scenario — 03:00 in woodland — is exactly where this lands. Say so
      // plainly and point at the radio, rather than showing an empty patrol that reads as
      // "nobody is in it".
      offline.value = true
    } else if (err instanceof HttpError && (err.status === 404 || err.status === 403)) {
      // One neutral answer for both. The BFF already makes them indistinguishable so the
      // endpoint cannot be used to map the numbering; the UI must not undo that by phrasing
      // them differently.
      notFound.value = true
    } else {
      failed.value = true
    }
  } finally {
    loading.value = false
  }
}

function photoUrl(member: PatrolMember): string | undefined {
  if (!member.hasPortrait) return undefined
  // Scoped to the patrol, not just the person: the BFF requires the member to belong to the
  // number in the path, so a person id alone cannot fetch a face.
  return `/api/contacts/patrols/${encodeURIComponent(shownNumber.value)}/photo/${encodeURIComponent(member.id)}?size=thumb`
}

function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return '?'
  if (parts.length === 1) return parts[0].slice(0, 1).toUpperCase()
  return (parts[0].slice(0, 1) + parts[parts.length - 1].slice(0, 1)).toUpperCase()
}
</script>

<template>
  <div>
    <!-- Deliberately a secondary, distinct entry rather than part of the search field above: a
         patrol is a different thing, asked for in a different way. -->
    <button
      type="button"
      class="flex w-full items-center gap-2 px-4 py-2 text-left text-sm text-slate-400"
      @click="open = true"
    >
      <Search class="size-4 text-slate-500" />
      Slå patrulje op
    </button>

    <!-- A plain overlay rather than the `drawer` primitive: this panel must be dismissible with
         no ceremony and must not animate content in from off-screen while somebody is reading a
         face in the dark. -->
    <div v-if="open" class="fixed inset-0 z-50 flex flex-col bg-slate-950 text-slate-100">
      <header class="flex items-center gap-2 border-b border-slate-800 px-3 py-3">
        <form class="flex flex-1 items-center gap-2" @submit.prevent="submit">
          <Input
            v-model="number"
            type="text"
            inputmode="numeric"
            autocomplete="off"
            enterkeyhint="search"
            placeholder="Patruljenummer"
            aria-label="Patruljenummer"
            autofocus
            class="border-slate-800 bg-slate-900 text-slate-100 placeholder:text-slate-500"
          />
          <button
            type="submit"
            class="shrink-0 rounded-md border border-slate-700 px-3 py-2 text-sm disabled:opacity-40"
            :disabled="!canSubmit"
          >
            Slå op
          </button>
        </form>
        <button
          type="button"
          class="shrink-0 p-2 text-slate-400"
          aria-label="Luk"
          @click="open = false"
        >
          <X class="size-5" />
        </button>
      </header>

      <div class="flex-1 overflow-y-auto">
        <p v-if="loading" class="px-4 py-10 text-center text-sm text-slate-400">Slår op …</p>

        <div v-else-if="offline" class="space-y-2 px-6 py-10 text-center">
          <WifiOff class="mx-auto size-6 text-amber-400" />
          <p class="text-sm text-slate-200">Kræver forbindelse</p>
          <!-- The honest fallback, named rather than implied. -->
          <p class="text-sm text-slate-400">Brug radioen eller ring til HQ.</p>
        </div>

        <p v-else-if="notFound" class="px-4 py-10 text-center text-sm text-slate-400">
          Ingen patrulje med det nummer.
        </p>

        <p v-else-if="failed" class="px-4 py-10 text-center text-sm text-slate-400">
          Opslaget mislykkedes. Prøv igen.
        </p>

        <template v-else-if="members">
          <p class="px-4 py-3 text-xs uppercase tracking-wide text-slate-500">
            Patrulje {{ shownNumber }} · {{ members.length }}
            {{ members.length === 1 ? 'medlem' : 'medlemmer' }}
          </p>

          <div
            v-for="member in members"
            :key="member.id"
            class="flex items-center gap-3 border-b border-slate-800/60 px-4 py-3 last:border-b-0"
            :class="{ 'opacity-70': hasLeftRace(member.status ?? '') }"
          >
            <Avatar size="lg">
              <AvatarImage
                v-if="member.hasPortrait"
                :src="photoUrl(member) ?? ''"
                :alt="member.name"
              />
              <AvatarFallback class="bg-slate-700 text-slate-200">
                {{ initials(member.name) }}
              </AvatarFallback>
            </Avatar>

            <div class="min-w-0 flex-1">
              <p class="truncate text-[15px] leading-tight">{{ member.name }}</p>
              <!-- The full status, not the directory's single bit: this is a live read, and "in
                   our care" is exactly what a samarit needs before setting off. -->
              <p
                v-if="member.status"
                class="truncate text-xs leading-tight"
                :class="
                  hasLeftRace(member.status)
                    ? 'text-amber-400'
                    : isInOurCare(member.status)
                      ? 'text-sky-300'
                      : 'text-slate-400'
                "
              >
                {{ memberStatusLabel(member.status) }}
              </p>
            </div>

            <a
              v-if="member.phone"
              :href="`tel:${member.phone}`"
              class="flex shrink-0 items-center gap-1.5 text-sm tabular-nums text-slate-300"
              :aria-label="`Ring til ${member.name}`"
            >
              <Phone class="size-3.5 text-slate-500" />
              <span class="hidden sm:inline">{{ formatPhone(member.phone) }}</span>
            </a>
          </div>
        </template>

        <p v-else class="px-6 py-10 text-center text-sm text-slate-500">
          Indtast hele patruljenummeret.
        </p>
      </div>
    </div>
  </div>
</template>
