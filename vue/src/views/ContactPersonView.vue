<script setup lang="ts">
// One person from the contacts directory (PRD 007 §7, task 167).
//
// Reached by tapping a row. Shows a large avatar and the person's details — and nothing else:
// **no postal address and no guardian number** (maintainer direction, 2026-08-31). Guardian
// numbers never enter the PWA at all, save where a member approves their own; that is a
// `.rules` invariant, and the BFF projects the field out rather than trusting this page not to
// render it.
//
// # No endpoint of its own
//
// This reads the synced directory, so it works offline for exactly the same set of people the
// list does — which is the requirement. A dedicated `GET /api/contacts/people/{id}` would have
// been a second, wider data path to the same fields, and a second place for somebody to add "the
// parent's number, for the samarit". The one case it does not cover is a person reached through
// the patrol lookup, whose details are live and never stored; that is task 168's, and belongs
// there precisely because those records must not be persisted.
//
// # Deep links are gated
//
// The route carries the same `roles` as the pane, so a spejder following a shared link is
// refused by the router guard (task 158) — and the portrait endpoint refuses independently.
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Phone, Star } from '@lucide/vue'

import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { formatPhone } from '@/helpers'
import { useContactsStore } from '@/stores/contacts.store'
import { useFavouritesStore } from '@/stores/favourites.store'

const route = useRoute()
const router = useRouter()
const contacts = useContactsStore()
const favourites = useFavouritesStore()

onMounted(() => {
  contacts.hydrate()
  favourites.hydrate(contacts.storage)
})

const personId = computed(() => String(route.params.personId ?? ''))

// `byId` deduplicates, so a crew bandit listed in two populations resolves to one person here.
const person = computed(() => contacts.byId.get(personId.value))

// Every group they are listed in, so a crew bandit's page says both — that is the fact somebody
// opened the page to learn.
const groupLabels = computed(() => {
  const labels = contacts.entries
    .filter((e) => e.id === personId.value)
    .map((e) => e.groups[e.groups.length - 1]?.label)
    .filter((v): v is string => Boolean(v))
  return [...new Set(labels)]
})

const photoUrl = computed(() =>
  person.value?.portraitVersion
    ? `/api/contacts/people/${encodeURIComponent(person.value.id)}/photo?size=full&v=${person.value.portraitVersion}`
    : undefined,
)

const initials = computed(() => {
  const parts = (person.value?.name ?? '').trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return '?'
  if (parts.length === 1) return parts[0].slice(0, 1).toUpperCase()
  return (parts[0].slice(0, 1) + parts[parts.length - 1].slice(0, 1)).toUpperCase()
})

// The number's presence is the BFF's decision, not this page's: it is sent unless the member is
// `released`. See ContactRow for the reasoning — a member who is out of the race is still often
// somewhere on site and worth reaching.
const callable = computed(() => Boolean(person.value?.phone))
</script>

<template>
  <!-- Same scoped night surface as the list, so moving between them does not flash a white
       screen at someone whose eyes have adapted to the dark. -->
  <section class="dark -mx-4 -mt-4 min-h-full bg-slate-950 pb-6 text-slate-100">
    <header class="flex items-center gap-2 px-2 pt-3">
      <button
        type="button"
        class="flex items-center gap-1 p-2 text-sm text-slate-400"
        @click="router.back()"
      >
        <ArrowLeft class="size-4" />
        Tilbage
      </button>
    </header>

    <!-- One neutral answer for "no such person" and "not visible to you": the list is the only
         way in, so anything else here is a stale link or a probe, and neither deserves a
         distinguishable reply. -->
    <p v-if="!person" class="px-6 py-16 text-center text-sm text-slate-400">
      Personen findes ikke i din liste.
    </p>

    <template v-else>
      <div class="flex flex-col items-center gap-4 px-6 pt-4">
        <!-- The photo at maximum legible size. No action bar: nothing here may imply sharing,
             saving or exporting a photograph of a member. -->
        <Avatar class="size-40 sm:size-56">
          <AvatarImage v-if="photoUrl" :src="photoUrl" :alt="person.name" />
          <AvatarFallback class="bg-slate-800 text-4xl text-slate-300">
            {{ initials }}
          </AvatarFallback>
        </Avatar>

        <div class="space-y-1 text-center">
          <h1 class="font-nathejk text-3xl tracking-wide">{{ person.name }}</h1>
          <p v-if="person.crewFunction" class="text-sm text-slate-400">
            {{ person.crewFunction }}
          </p>
          <p v-if="groupLabels.length > 0" class="text-sm text-slate-400">
            {{ groupLabels.join(' · ') }}
          </p>
          <p v-if="!person.stillInRace" class="text-sm text-amber-400">Ude af løbet</p>
        </div>

        <div class="flex items-center gap-2">
          <a
            v-if="callable"
            :href="`tel:${person.phone}`"
            class="flex items-center gap-2 rounded-md border border-slate-700 px-4 py-2 text-sm tabular-nums text-slate-100"
          >
            <Phone class="size-4 text-slate-400" />
            {{ formatPhone(person.phone!) }}
          </a>

          <button
            type="button"
            class="flex items-center gap-2 rounded-md border border-slate-700 px-4 py-2 text-sm"
            :class="favourites.has(person.id) ? 'text-amber-300' : 'text-slate-300'"
            :aria-pressed="favourites.has(person.id) ? 'true' : 'false'"
            @click="favourites.toggle(person.id)"
          >
            <Star class="size-4" :fill="favourites.has(person.id) ? 'currentColor' : 'none'" />
            {{ favourites.has(person.id) ? 'Favorit' : 'Gør til favorit' }}
          </button>
        </div>
      </div>
    </template>
  </section>
</template>
