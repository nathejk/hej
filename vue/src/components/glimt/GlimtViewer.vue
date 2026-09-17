<script setup lang="ts">
// The full-screen viewer: one glimt's media, large (PRD 019 §7, task 318).
//
// # No auto-advance
//
// It is a dialog you swipe, not a tape that plays. PRD 019 is explicit that a glimt is "not a
// vertical auto-advancing tape" — that is the Snapchat behaviour the feature is an alternative to, and
// it turns looking at a photograph into a thing with a deadline.
//
// # Full media here, thumbnails everywhere else
//
// This is the only surface that asks for the full-size image, which is what makes the cache split in
// task 315 work: the grid and the feed pull thumbnails by the thousand, and the ~30 kB versions are
// fetched one at a time by somebody who has chosen to look closely.
//
// # `object-contain`, unlike the feed
//
// The feed and the grid crop to a shared shape so cards do not change height. Here there is nothing
// below to shove around and the whole point is to see the photograph, so nothing is cropped. This is
// where a member checks what they actually shared.
import { computed, ref, watch } from 'vue'
import { Download, X } from '@lucide/vue'

import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import { cn } from '@/helpers'
import { glimtMediaUrl, type Glimt } from '@/stores/glimt.store'
import { attributionLine, mediaAltText, OWN_ATTRIBUTION } from '@/components/glimt/glimtPresentation'

const props = defineProps<{
  glimt: Glimt | null
  /** Which item to open on. */
  ordinal: number
}>()

const open = defineModel<boolean>('open', { default: false })

const index = ref(0)

// Re-seeded whenever a new glimt is opened, so opening item 3 of one glimt does not leave the viewer
// on item 3 of the next.
watch(
  () => [props.glimt?.id, props.ordinal] as const,
  () => {
    const items = props.glimt?.media ?? []
    const found = items.findIndex((m) => m.ordinal === props.ordinal)
    index.value = found >= 0 ? found : 0
  },
  { immediate: true },
)

const items = computed(() => props.glimt?.media ?? [])
const current = computed(() => items.value[index.value] ?? null)
const many = computed(() => items.value.length > 1)

const attribution = computed(() => {
  if (!props.glimt) return ''
  return props.glimt.own ? OWN_ATTRIBUTION : attributionLine(props.glimt.hold)
})

function step(by: number) {
  if (!many.value) return
  const n = items.value.length
  // Wraps, like the feed's strip: there is no end to arrive at in a handful of photographs from one
  // moment, and stopping dead reads as the control having broken.
  index.value = (index.value + by + n) % n
}

/**
 * The download URL for the current item.
 *
 * Full size, not the thumbnail — somebody saving a photograph wants the photograph.
 */
const downloadUrl = computed(() =>
  props.glimt && current.value ? glimtMediaUrl(props.glimt.id, current.value.ordinal, 'full') : '',
)

/**
 * A filename worth having in a camera roll.
 *
 * Named after the hold and the date rather than the glimt's uuid, because the uuid means nothing to
 * the person who now has this file among their own photographs.
 */
const downloadName = computed(() => {
  if (!props.glimt) return 'glimt.jpg'
  const date = new Date(props.glimt.createdAt)
  const stamp = Number.isNaN(date.getTime()) ? '' : date.toISOString().slice(0, 10)
  const hold = (props.glimt.hold.name || props.glimt.hold.number || 'glimt')
    .toLowerCase()
    .replace(/[^a-z0-9æøå]+/gi, '-')
    .replace(/^-|-$/g, '')
  const n = (current.value?.ordinal ?? 0) + 1
  return `nathejk-${stamp}-${hold}-${n}.jpg`
})
</script>

<template>
  <Dialog v-model:open="open">
    <!-- Edge to edge and black: a photograph is the content, and chrome around it in a dark forest is
         just glare. `showCloseButton` is off because the default close sits inside the padding this
         dialog does not have; there is an explicit one below instead. -->
    <DialogContent
      class="max-w-none border-0 bg-black p-0 sm:max-w-none"
      :show-close-button="false"
    >
      <!-- Required for the dialog's accessible name. Visually hidden: the attribution is drawn in
           the header below, and announcing it twice is noise. -->
      <DialogTitle class="sr-only">
        {{ attribution ? `Glimt fra ${attribution}` : 'Glimt' }}
      </DialogTitle>

      <div class="relative flex h-[90vh] w-screen flex-col">
        <header class="flex items-center gap-2 px-3 py-2 text-white">
          <p class="min-w-0 flex-1 truncate text-sm font-medium">{{ attribution }}</p>
          <p v-if="many" class="shrink-0 text-xs text-white/70">
            {{ index + 1 }} / {{ items.length }}
          </p>

          <!-- Save. A plain anchor `download`, not `showSaveFilePicker`, which is not on the iOS 16.4
               baseline. On iOS Safari this opens the share sheet, which is where "Save to Photos"
               lives; long-press on the image is the fallback iOS users already know.
               **Unverified on a device** — see task 318. -->
          <a
            v-if="downloadUrl"
            :href="downloadUrl"
            :download="downloadName"
            class="flex size-11 items-center justify-center rounded-md text-white hover:bg-white/10"
            aria-label="Gem billedet"
          >
            <Download class="size-5" aria-hidden="true" />
          </a>

          <Button
            variant="ghost"
            size="icon"
            class="size-11 shrink-0 text-white hover:bg-white/10 hover:text-white"
            aria-label="Luk"
            @click="open = false"
          >
            <X class="size-5" aria-hidden="true" />
          </Button>
        </header>

        <!-- The photograph. `object-contain`, so nothing is cropped: this is the surface where a
             member checks what they actually shared. -->
        <div class="relative min-h-0 flex-1">
          <template v-if="glimt && current">
            <video
              v-if="current.kind === 'video'"
              :src="glimtMediaUrl(glimt.id, current.ordinal, 'full')"
              class="size-full object-contain"
              controls
              playsinline
              muted
            />
            <img
              v-else
              :src="glimtMediaUrl(glimt.id, current.ordinal, 'full')"
              :alt="mediaAltText(glimt, current.ordinal)"
              class="size-full object-contain"
              decoding="async"
            />
          </template>

          <!-- Stepping controls. Always visible here, not `pointer-coarse:hidden` like the feed's:
               the feed's arrows duplicate a swipe on the card, whereas this dialog has no swipe of
               its own, so on a phone these are the only way through. -->
          <template v-if="many">
            <button
              type="button"
              class="absolute left-1 top-1/2 flex size-12 -translate-y-1/2 items-center justify-center rounded-full bg-black/40 text-2xl text-white"
              aria-label="Forrige billede"
              @click="step(-1)"
            >
              ‹
            </button>
            <button
              type="button"
              class="absolute right-1 top-1/2 flex size-12 -translate-y-1/2 items-center justify-center rounded-full bg-black/40 text-2xl text-white"
              aria-label="Næste billede"
              @click="step(1)"
            >
              ›
            </button>
          </template>
        </div>

        <!-- The caption, under the photograph rather than over it: text on an image is unreadable to
             a screen reader and often to a person. -->
        <p
          v-if="glimt?.caption"
          :class="cn('max-h-24 overflow-y-auto px-3 pb-3 text-sm text-white/90')"
        >
          {{ glimt.caption }}
        </p>
      </div>
    </DialogContent>
  </Dialog>
</template>
