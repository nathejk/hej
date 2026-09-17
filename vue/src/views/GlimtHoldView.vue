<script setup lang="ts">
// One hold's glimt, as a grid (PRD 019 §0a.1, task 326).
//
// # This is the post-race surface, not a secondary view
//
// During the race a member opens Glimt to see what is happening now, and newest-first is right.
// Afterwards the question changes completely — "show me *our* photos, then our friends' patrulje, then
// everything" — and a chronological feed answers that badly, because a hold's twelve photographs are
// scattered through a thousand. Maintainer direction: spejdere will spend hours doing exactly this.
//
// So: **oldest first**, because a race reads forward in time and somebody reliving an evening wants it
// in the order it happened. The order comes from the BFF (task 319) and is not re-sorted here.
//
// # Thumbnails only
//
// Every tile asks for `?variant=thumb` (~3 kB). Full media is fetched only when a tile is opened, into
// its own smaller cache (task 315). This is the load peak of the whole feature — a thousand people at
// the finish line on the worst network of the weekend — and a grid of full-size images is the
// difference between usable and not.
//
// # A grid, not cards
//
// The feed's card carries attribution, an audience chip and a caption because each glimt arrives out
// of context. Here every tile has the same attribution — it is in the heading — so the chrome would be
// repeated twenty times to say nothing. The caption moves into the viewer, which is where somebody who
// has opened a photograph can read it.
import { computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, ImageOff, Loader2 } from '@lucide/vue'

import { Button } from '@/components/ui/button'
import GlimtViewer from '@/components/glimt/GlimtViewer.vue'
import { attributionLine, holdWord } from '@/components/glimt/glimtPresentation'
import { glimtMediaUrl, useGlimtStore, type Glimt } from '@/stores/glimt.store'
import { useOpenGlimt } from '@/composables/useOpenGlimt'

const route = useRoute()
const router = useRouter()
const glimt = useGlimtStore()

const number = computed(() => String(route.params.number ?? ''))
const entries = computed<Glimt[]>(() => glimt.holdGlimt[number.value] ?? [])

// The heading. Taken from the first glimt's frozen attribution rather than from the index, so the page
// is correct on a direct load — a shared link, or a reload — where the index has never been fetched.
const heading = computed(() => {
  const first = entries.value[0]
  if (first) return attributionLine(first.hold)
  const known = glimt.holds.find((h) => h.number === number.value)
  if (known) return attributionLine({ number: known.number, name: known.name, group: known.group })
  // Nothing loaded yet. The hold *word* is unknown too, so this stays deliberately vague rather than
  // guessing "Patrulje" for what might be a klan.
  return `Hold ${number.value}`
})

// Flat list of every tile in the collection, so the grid is one loop and the viewer can be opened with
// the glimt it belongs to.
const tiles = computed(() =>
  entries.value.flatMap((entry) =>
    entry.media.map((media) => ({
      key: `${entry.id}-${media.ordinal}`,
      entry,
      ordinal: media.ordinal,
      src: glimtMediaUrl(entry.id, media.ordinal, media.hasThumb ? 'thumb' : 'full'),
      kind: media.kind,
    })),
  ),
)

const viewer = useOpenGlimt()

onMounted(load)
// Tapping one hold's attribution from another hold's grid changes the param without remounting.
watch(number, load)

function load() {
  if (!number.value) return
  // Re-fetched on every visit rather than served from `holdGlimt` if present: a hold's collection is
  // the surface people revisit while photographs are still arriving, and the images are cached
  // regardless (task 315), so the cost of being current is a few kilobytes of metadata.
  glimt.fetchHold(number.value)
}
</script>

<template>
  <div class="mx-auto flex w-full max-w-3xl flex-col gap-3 p-3">
    <header class="flex items-center gap-2">
      <Button variant="ghost" size="icon" class="size-11 shrink-0" aria-label="Tilbage" @click="router.back()">
        <ArrowLeft class="size-5" aria-hidden="true" />
      </Button>
      <div class="min-w-0">
        <h1 class="truncate font-nathejk text-2xl tracking-wide">{{ heading }}</h1>
        <p v-if="tiles.length" class="text-xs text-muted-foreground">
          {{ tiles.length }} {{ tiles.length === 1 ? 'billede' : 'billeder' }}
        </p>
      </div>
    </header>

    <p v-if="glimt.error" class="rounded-md bg-muted px-3 py-2 text-sm text-muted-foreground">
      {{ glimt.error }}
    </p>

    <div v-if="glimt.loadingHold && !tiles.length" class="flex justify-center py-10">
      <Loader2 class="size-6 animate-spin text-muted-foreground" aria-hidden="true" />
    </div>

    <!-- Empty is a normal answer, not a fault: it means nothing this hold posted was shared as far as
         the caller. Said plainly rather than left as a blank page. -->
    <div
      v-else-if="!tiles.length"
      class="flex flex-col items-center gap-2 rounded-lg border border-dashed px-6 py-10 text-center"
    >
      <ImageOff class="size-8 text-muted-foreground" aria-hidden="true" />
      <p class="text-sm text-muted-foreground">
        Ingen glimt fra {{ holdWord(entries[0]?.hold.group ?? '') || 'holdet' }} her.
      </p>
    </div>

    <!-- Square tiles, three across on a phone and more on a wider screen. Square rather than the
         card's shared aspect ratio: a grid's job is to be scannable, and a uniform block is what makes
         twenty photographs readable at a glance. -->
    <ul v-else class="grid grid-cols-3 gap-1 sm:grid-cols-4 md:grid-cols-5">
      <li v-for="tile in tiles" :key="tile.key">
        <button
          type="button"
          class="relative block aspect-square w-full overflow-hidden rounded-sm bg-muted"
          @click="viewer.open(tile.entry, tile.ordinal)"
        >
          <img
            :src="tile.src"
            alt=""
            loading="lazy"
            decoding="async"
            class="size-full object-cover object-center"
          />
          <!-- Marked rather than labelled: a play glyph reads at 100px, a word does not. The alt text
               is empty on purpose — the viewer carries the real description, and twenty "Glimt fra
               Patrulje 42" in a row is noise to a screen reader, not information. -->
          <span
            v-if="tile.kind === 'video'"
            class="pointer-events-none absolute inset-0 flex items-center justify-center text-white drop-shadow"
            aria-hidden="true"
          >
            ▶
          </span>
        </button>
      </li>
    </ul>

    <GlimtViewer v-model:open="viewer.isOpen.value" :glimt="viewer.glimt.value" :ordinal="viewer.ordinal.value" />
  </div>
</template>
