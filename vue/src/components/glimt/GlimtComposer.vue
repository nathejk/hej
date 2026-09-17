<script setup lang="ts">
// The composer: pick media, write a caption, choose who sees it (PRD 019 §7, task 317).
//
// # A drawer, not a route
//
// The feed stays behind it, which matters more than it sounds: a member who opens the composer and
// changes their mind should be back where they were, not navigated somewhere and then back. `Drawer`
// is the repo's standard overlay for anything substantial, and its swipe-to-dismiss is what makes
// backing out cheap.
//
// # Library-first, camera second — a deliberate departure from the portrait path
//
// Task 317 says to reuse the portrait's capture path. Read literally that would be wrong here, and
// the reason is worth stating: `PhotoCapture.vue` is a live viewfinder with `capture="user"`, framed
// for a face, producing exactly one image. A glimt is a scene, usually already on the phone, usually
// several at once — "that is how most glimt will actually be made" (PRD 019 §7). So the primary
// control is a **multi-select file input**, with a separate camera button using
// `capture="environment"` for the rear camera. What is reused is the *fallback discipline* the
// portrait established: the file input is always reachable, never only after a failure.
//
// # The audience choice is the most important thing on this screen
//
// A visible three-option list, narrowest first and selected by default. Never a dropdown: with no
// approval queue in front of the public scope (PRD 019 §0), the consequence line under "Offentligt"
// is the only thing between a child and the open web, and a collapsed control hides it.
import { computed, ref } from 'vue'
import { Camera, ImagePlus, Loader2, X } from '@lucide/vue'

import { Button } from '@/components/ui/button'
import {
  Drawer,
  DrawerClose,
  DrawerContent,
  DrawerDescription,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
} from '@/components/ui/drawer'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  CONSENT_NOTE,
  DEFAULT_AUDIENCE,
  attributionNote,
  audienceOptions,
  type Audience,
} from '@/components/glimt/audienceChoice'
import { cn } from '@/helpers'
import { useGlimtStore } from '@/stores/glimt.store'
import { useSessionStore } from '@/stores/session.store'

const MAX_ITEMS = 10
const MAX_CAPTION = 280

const glimt = useGlimtStore()
const session = useSessionStore()

const open = defineModel<boolean>('open', { default: false })

const emit = defineEmits<{ shared: [] }>()

interface Draft {
  file: File
  /** Object URL for the thumbnail. Revoked when the item is removed or the draft is cleared. */
  preview: string
}

const items = ref<Draft[]>([])
const caption = ref('')
const audience = ref<Audience>(DEFAULT_AUDIENCE)
const sharing = ref(false)
const error = ref('')

const options = computed(() => audienceOptions(session.role))
const attribution = computed(() => attributionNote(session.role))

const remaining = computed(() => MAX_CAPTION - caption.value.length)
const canShare = computed(() => items.value.length > 0 && !sharing.value)

function onPick(event: Event) {
  const input = event.target as HTMLInputElement
  const picked = Array.from(input.files ?? [])
  // Silently truncate rather than refuse the lot: a member who selected fifteen photos meant to
  // share photos, and rejecting the whole selection over the limit is the least useful response.
  const room = MAX_ITEMS - items.value.length
  for (const file of picked.slice(0, room)) {
    items.value.push({
      file,
      preview: URL.createObjectURL(file),
    })
  }
  if (picked.length > room) {
    error.value = `Et glimt kan højst have ${MAX_ITEMS} filer. De første ${room} er valgt.`
  }
  // Reset, or picking the same file twice in a row does nothing.
  input.value = ''
}

function remove(index: number) {
  const [dropped] = items.value.splice(index, 1)
  if (dropped) URL.revokeObjectURL(dropped.preview)
}

function reset() {
  for (const item of items.value) URL.revokeObjectURL(item.preview)
  items.value = []
  caption.value = ''
  audience.value = DEFAULT_AUDIENCE
  error.value = ''
}

async function share() {
  if (!canShare.value) return
  sharing.value = true
  error.value = ''
  try {
    // Queued first, then sent (task 314). The files reach IndexedDB *before* anything is attempted,
    // which is what makes PRD 019 §5's promise true: the post survives a failed upload, a locked
    // phone and an app the OS killed. The drawer closes either way — a post is accepted, and
    // delivery is our problem rather than the member's.
    const result = await glimt.queue({
      caption: caption.value.trim(),
      audience: audience.value,
      files: items.value.map((i) => ({ blob: i.file, name: i.file.name })),
    })

    if (!result.queued && !result.sent) {
      // Neither stored nor sent: this platform has no outbox and the network refused. The only case
      // where a member must be kept here, because closing the drawer really would lose the photos.
      error.value = glimt.error || 'Glimtet kunne ikke sendes. Prøv igen.'
      return
    }

    reset()
    open.value = false
    emit('shared')
  } finally {
    sharing.value = false
  }
}
</script>

<template>
  <Drawer v-model:open="open" @update:open="(o) => !o && !sharing && reset()">
    <DrawerContent>
      <DrawerHeader>
        <DrawerTitle>Del et glimt</DrawerTitle>
        <DrawerDescription>{{ attribution }}</DrawerDescription>
      </DrawerHeader>

      <div class="flex max-h-[65vh] flex-col gap-4 overflow-y-auto px-4">
        <!-- Media first: the picture is what a member came to share, and the rest is about it. -->
        <section class="flex flex-col gap-2">
          <div v-if="items.length" class="flex gap-2 overflow-x-auto pb-1">
            <div
              v-for="(item, index) in items"
              :key="item.preview"
              class="relative size-20 shrink-0 overflow-hidden rounded-md bg-muted"
            >
              <img :src="item.preview" alt="" class="h-full w-full object-cover" />
              <!-- ≥44px: removing the wrong photo with a thumb in the dark is the mistake this
                   size prevents. Offset so it does not cover the image it belongs to. -->
              <button
                type="button"
                class="absolute -right-1 -top-1 flex size-11 items-center justify-center"
                :aria-label="`Fjern fil ${index + 1}`"
                @click="remove(index)"
              >
                <span class="flex size-6 items-center justify-center rounded-full bg-background shadow">
                  <X class="size-4" aria-hidden="true" />
                </span>
              </button>
            </div>
          </div>

          <div class="flex gap-2">
            <!-- Library, multi-select: the primary path. -->
            <Label
              :class="
                cn(
                  'flex min-h-11 flex-1 cursor-pointer items-center justify-center gap-2',
                  'rounded-md border border-input px-3 text-sm font-medium',
                )
              "
            >
              <ImagePlus class="size-5" aria-hidden="true" />
              Vælg billeder
              <input
                type="file"
                accept="image/*"
                multiple
                class="sr-only"
                @change="onPick"
              />
            </Label>

            <!-- Camera, rear-facing. `capture="environment"` because a glimt is a scene, not a
                 selfie — the portrait's `capture="user"` would be actively wrong here. -->
            <Label
              :class="
                cn(
                  'flex min-h-11 cursor-pointer items-center justify-center gap-2',
                  'rounded-md border border-input px-3 text-sm font-medium',
                )
              "
            >
              <Camera class="size-5" aria-hidden="true" />
              <span class="sr-only">Tag et billede</span>
              <input
                type="file"
                accept="image/*"
                capture="environment"
                class="sr-only"
                @change="onPick"
              />
            </Label>
          </div>
        </section>

        <section class="flex flex-col gap-1.5">
          <Label for="glimt-caption">Tekst (valgfrit)</Label>
          <Input
            id="glimt-caption"
            v-model="caption"
            :maxlength="MAX_CAPTION"
            placeholder="Hvad sker der?"
          />
          <p v-if="remaining < 40" class="text-right text-xs text-muted-foreground">
            {{ remaining }}
          </p>
        </section>

        <!-- The audience. A visible list, never a dropdown: the consequence lines are the point,
             and a collapsed control hides exactly the sentence that matters most. -->
        <section class="flex flex-col gap-2">
          <h3 class="text-sm font-medium">Hvem skal se det?</h3>
          <button
            v-for="option in options"
            :key="option.value"
            type="button"
            role="radio"
            :aria-checked="audience === option.value"
            :class="
              cn(
                'flex min-h-11 flex-col items-start gap-0.5 rounded-md border px-3 py-2 text-left',
                audience === option.value ? 'border-primary bg-primary/5' : 'border-input',
              )
            "
            @click="audience = option.value"
          >
            <span class="text-sm font-medium">{{ option.label }}</span>
            <span
              :class="
                cn(
                  'text-xs',
                  option.reachesOutside ? 'text-destructive' : 'text-muted-foreground',
                )
              "
            >
              {{ option.consequence }}
            </span>
          </button>
        </section>

        <!-- One quiet line. It was three — consent, the Team-section reach, and the retention window —
             and three grey notes under the audience choice read as boilerplate, which is how the one
             that matters gets skipped along with the rest (maintainer direction, 2026-09-17). The other
             two are on /privatliv, where there is room to explain them.

             Not a checkbox: a forced tick trains people to tick it, and this has to do its work at the
             moment of deciding. -->
        <section class="flex flex-col gap-1 text-xs text-muted-foreground">
          <p>{{ CONSENT_NOTE }}</p>
        </section>

        <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
      </div>

      <DrawerFooter>
        <Button size="lg" :disabled="!canShare" @click="share">
          <Loader2 v-if="sharing" class="size-5 animate-spin" aria-hidden="true" />
          {{ sharing ? 'Deler…' : 'Del' }}
        </Button>
        <DrawerClose as-child>
          <Button variant="outline" size="lg" :disabled="sharing">Annuller</Button>
        </DrawerClose>
      </DrawerFooter>
    </DrawerContent>
  </Drawer>
</template>
