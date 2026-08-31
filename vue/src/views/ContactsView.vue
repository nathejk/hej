<script setup lang="ts">
// Kontakter — the contacts directory (PRD 007, tasks 163/165).
//
// The crew/bandit/gøgler directory, cached on the device and usable with the radio off. Who is
// listed is decided entirely by the BFF: this view renders what it is given and infers nothing.
// Spejdere are not in it, and do not get this pane at all (task 158).
//
// # Not fullBleed
//
// The list scrolls, and `fullBleed` drops App.vue's `overflow-y-auto` wrapper. The map wants
// that; a list does not.
//
// # Night-legible, scoped to this view
//
// Dark, high-contrast, minimal chrome — used standing up, in the cold, at 03:00, where a
// full-brightness white screen destroys the reader's dark adaptation. Built on the `.dark` token
// block PRD 004 retained, applied to this view's own container: **not** a global theme, and
// `color-scheme` stays `light`, because app-wide dark mode is not supported.
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { RefreshCw, Search, Star, WifiOff } from '@lucide/vue'

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { Input } from '@/components/ui/input'
import ContactRow from '@/components/contacts/ContactRow.vue'
import { useContactsFreshness } from '@/composables/useContactsFreshness'
import { searchContacts } from '@/helpers/contactSearch'
import { useContactsStore, type ContactEntry } from '@/stores/contacts.store'
import { useFavouritesStore } from '@/stores/favourites.store'

const router = useRouter()
const contacts = useContactsStore()
const favourites = useFavouritesStore()

const query = ref('')

// Starts the freshness loop for as long as this pane is mounted, and gives us the initial
// refresh for free (task 162). Scoped here rather than app-wide so a user who never opens the
// pane generates no polling traffic at all.
useContactsFreshness()

onMounted(() => {
  contacts.hydrate()
  favourites.hydrate(contacts.storage)
  favourites.pruneAgainstDirectory()
})

// Folding, field selection and ordering live in `helpers/contactSearch.ts` — pure rules, tested
// without mounting anything, in the same spirit as `config/nudge.ts`.
const searching = computed(() => query.value.trim().length > 0)

const results = computed(() =>
  searchContacts(contacts.entries, query.value, (id) => favourites.has(id)),
)

const favouriteEntries = computed(() =>
  favourites.ids
    .map((id) => contacts.byId.get(id))
    .filter((e): e is ContactEntry => e !== undefined),
)

// The caller's own group opens by default, so the common case needs no interaction. Expansion is
// then the user's business: `v-model` keeps whatever they change it to for the life of the pane.
const openGroups = ref<string[]>([])
const initialisedOpen = ref(false)

const groupViews = computed(() => contacts.groupViews)

// Seeded once the first payload arrives rather than in onMounted, because on a cold start the
// entries land after mount. Guarded by a flag so a background refresh never re-opens a section
// the user deliberately collapsed (task 162's in-place-update requirement).
const groupKey = (population: string, id: string) => `${population}/${id}`

const seedOpenGroups = computed(() => {
  if (!initialisedOpen.value && groupViews.value.length > 0) {
    openGroups.value = groupViews.value
      .filter((v) => v.group.isOwn)
      .map((v) => groupKey(v.population, v.group.id))
    initialisedOpen.value = true
  }
  return openGroups.value
})

const populationLabels: Record<string, string> = {
  bandit: 'Banditter',
  'gøgler': 'Gøglere',
  crew: 'Crew',
}

// One header per population, so a crew member scanning for a colleague can tell a klan section
// from the crew list at a glance.
const sections = computed(() => {
  const order = ['crew', 'bandit', 'gøgler']
  const byPopulation = new Map<string, typeof groupViews.value>()
  for (const view of groupViews.value) {
    const list = byPopulation.get(view.population) ?? []
    list.push(view)
    byPopulation.set(view.population, list)
  }
  return [...byPopulation.entries()]
    .sort((a, b) => order.indexOf(a[0]) - order.indexOf(b[0]))
    .map(([population, views]) => ({
      population,
      label: populationLabels[population] ?? population,
      views,
    }))
})

// The sync line reads as *current* during the race rather than as a timestamp the user has to
// interpret. "Synkroniseret 21:40" invites the question "is that recent?"; "Opdateret nu"
// answers it. An explicit stale state matters more than a precise one: a user who believes they
// have the directory and does not is worse off than one who knows.
const syncLabel = computed(() => {
  if (contacts.syncedAt === null) return 'Ikke hentet endnu'

  const age = Date.now() - contacts.syncedAt
  if (age < 2 * 60_000) return 'Opdateret nu'

  const time = new Date(contacts.syncedAt).toLocaleTimeString('da-DK', {
    hour: '2-digit',
    minute: '2-digit',
  })
  return `Opdateret kl. ${time}`
})

function open(id: string) {
  router.push({ name: 'contact-person', params: { personId: id } })
}
</script>

<template>
  <!-- `dark` on this container only: a scoped night surface, not a global theme. -->
  <section class="dark -mx-4 -mt-4 min-h-full bg-slate-950 pb-4 text-slate-100">
    <header class="sticky top-0 z-10 space-y-3 bg-slate-950/95 px-4 pb-3 pt-4 backdrop-blur">
      <div class="flex items-baseline justify-between gap-3">
        <h1 class="font-nathejk text-2xl tracking-wide">Kontakter</h1>
        <span class="flex items-center gap-1.5 text-xs text-slate-500">
          <RefreshCw v-if="contacts.loading" class="size-3 animate-spin" />
          <WifiOff v-else-if="contacts.error" class="size-3 text-amber-400" />
          {{ contacts.error ? 'Ikke opdateret' : syncLabel }}
        </span>
      </div>

      <!-- Sticky, so it stays reachable with a thumb while scrolling a long list. -->
      <div class="relative">
        <Search class="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-slate-500" />
        <Input
          v-model="query"
          type="search"
          inputmode="search"
          autocomplete="off"
          placeholder="Søg efter navn, klan eller nummer"
          aria-label="Søg i kontakter"
          class="border-slate-800 bg-slate-900 pl-9 text-slate-100 placeholder:text-slate-500"
        />
      </div>
    </header>

    <!-- Nothing synced yet, and nothing to show. Distinct from "no matches" below, because the
         two need different actions from the user (task 169). -->
    <p v-if="!contacts.hasCopy && !contacts.loading" class="px-4 py-10 text-center text-sm text-slate-400">
      Kontakter hentes, når du er online.
    </p>

    <!-- Search results replace the sections entirely: two competing lists on one screen is a
         worse answer than one. -->
    <div v-else-if="searching">
      <p v-if="results.length === 0" class="px-4 py-10 text-center text-sm text-slate-400">
        Ingen match på "{{ query.trim() }}".
      </p>
      <ContactRow
        v-for="entry in results"
        :key="`${entry.population}/${entry.id}`"
        :entry="entry"
        :favourite="favourites.has(entry.id)"
        @open="open"
        @toggle-favourite="favourites.toggle"
      />
    </div>

    <template v-else>
      <!-- Favourites first, and hidden entirely when empty rather than shown as a bare header. -->
      <section v-if="favouriteEntries.length > 0" class="mb-2">
        <h2 class="flex items-center gap-1.5 px-4 py-2 text-xs font-medium uppercase tracking-wide text-slate-500">
          <Star class="size-3" />
          Favoritter
        </h2>
        <ContactRow
          v-for="entry in favouriteEntries"
          :key="`fav/${entry.id}`"
          :entry="entry"
          favourite
          @open="open"
          @toggle-favourite="favourites.toggle"
        />
      </section>

      <section v-for="section in sections" :key="section.population">
        <h2 class="px-4 py-2 text-xs font-medium uppercase tracking-wide text-slate-500">
          {{ section.label }}
        </h2>

        <!-- A single group needs no accordion: crew and gøglere are one list each, and a
             collapsible section containing everything is a control that only ever gets in the
             way. -->
        <template v-if="section.views.length === 1">
          <ContactRow
            v-for="entry in section.views[0].entries"
            :key="`${entry.population}/${entry.id}`"
            :entry="entry"
            :favourite="favourites.has(entry.id)"
            @open="open"
            @toggle-favourite="favourites.toggle"
          />
        </template>

        <Accordion v-else v-model="openGroups" type="multiple" :default-value="seedOpenGroups">
          <AccordionItem
            v-for="view in section.views"
            :key="groupKey(view.population, view.group.id)"
            :value="groupKey(view.population, view.group.id)"
            class="border-slate-800"
          >
            <AccordionTrigger class="px-4 text-slate-200">
              <span class="flex items-baseline gap-2">
                {{ view.group.label }}
                <span class="text-xs font-normal text-slate-500">{{ view.entries.length }}</span>
              </span>
            </AccordionTrigger>
            <AccordionContent>
              <ContactRow
                v-for="entry in view.entries"
                :key="`${entry.population}/${entry.id}`"
                :entry="entry"
                :favourite="favourites.has(entry.id)"
                @open="open"
                @toggle-favourite="favourites.toggle"
              />
            </AccordionContent>
          </AccordionItem>
        </Accordion>
      </section>
    </template>
  </section>
</template>
