<script setup lang="ts">
// The portrait section of the profile page (PRD 003 §7): current photo or a placeholder,
// tapping opens the capture surface, upload shows progress **on the portrait** rather
// than in a modal.
//
// Composes the shadcn-vue `avatar` primitive. PRD 007 composes the same primitive for its
// thumbnails rather than reusing this component — this one owns an upload flow, which is
// not something a read-only identification grid should inherit.
import { ref } from 'vue'
import { Camera, RefreshCw } from '@lucide/vue'
import { useProfileStore } from '@/stores/profile.store'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import PhotoCapture from '@/components/profile/PhotoCapture.vue'

const profile = useProfileStore()

const capturing = ref(false)
const uploading = ref(false)
const error = ref('')
// Kept so "Prøv igen" retries the same photo rather than making the user retake it —
// the upload is what failed, not the picture.
const pending = ref<Blob | null>(null)

async function upload(blob: Blob) {
  pending.value = blob
  uploading.value = true
  error.value = ''
  try {
    await profile.uploadPhoto(blob)
    pending.value = null
  } catch (err) {
    // The store rethrows on purpose: reporting success here would leave the user
    // believing there is a photo on file when there is not, and this is a safety
    // feature. The BFF's message is shown when it has one — it is written in Danish and
    // is more specific than anything this component could guess.
    error.value = err instanceof Error && err.message ? err.message : 'Billedet blev ikke gemt.'
  } finally {
    uploading.value = false
  }
}

function onCaptured(blob: Blob) {
  capturing.value = false
  void upload(blob)
}

function retry() {
  if (pending.value) void upload(pending.value)
}
</script>

<template>
  <section>
    <div class="flex items-center gap-4">
      <!-- The portrait itself is the button, per PRD 003 §7 ("tapping the portrait opens
           the capture surface"). -->
      <button
        type="button"
        class="relative rounded-full focus-visible:ring-2 focus-visible:ring-slate-400 focus-visible:outline-none"
        :aria-label="profile.photoUrl ? 'Skift dit billede' : 'Tag et billede af dig selv'"
        :disabled="uploading"
        @click="capturing = true"
      >
        <Avatar class="size-24">
          <AvatarImage v-if="profile.photoUrl" :src="profile.photoUrl" :alt="`Billede af ${profile.details?.name ?? 'dig'}`" />
          <AvatarFallback>
            <Camera class="h-7 w-7 text-slate-400" aria-hidden="true" />
          </AvatarFallback>
        </Avatar>

        <!-- Inline progress, not a modal spinner: the thing that is busy is the
             portrait, and a modal would hide the rest of a page that still works. -->
        <span
          v-if="uploading"
          class="absolute inset-0 flex items-center justify-center rounded-full bg-black/50 text-xs font-medium text-white"
        >
          Gemmer …
        </span>
      </button>

      <div class="min-w-0 flex-1">
        <h2 class="font-nathejk text-lg text-slate-900">Mit billede</h2>
        <p class="mt-1 text-sm text-slate-600">
          Billedet bruges til at genkende dig under løbet — meget af det foregår i mørke.
        </p>
        <button
          type="button"
          class="mt-2 inline-flex min-h-9 items-center gap-1.5 rounded-lg bg-slate-900 px-3 py-1.5 text-sm font-medium text-white disabled:opacity-50"
          :disabled="uploading"
          @click="capturing = true"
        >
          <component :is="profile.photoUrl ? RefreshCw : Camera" class="h-4 w-4" aria-hidden="true" />
          {{ profile.photoUrl ? 'Tag et nyt' : 'Tag et billede' }}
        </button>
      </div>
    </div>

    <p v-if="error" class="mt-3 rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-900">
      {{ error }}
      <button v-if="pending" type="button" class="ml-1 font-medium underline" @click="retry">
        Prøv igen
      </button>
    </p>

    <PhotoCapture v-if="capturing" @captured="onCaptured" @cancel="capturing = false" />
  </section>
</template>
