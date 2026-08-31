<script setup lang="ts">
import { Camera } from '@lucide/vue'

import PhotoCapture from '@/components/profile/PhotoCapture.vue'
import { usePortraitCapture } from '@/composables/usePortraitCapture'

// The portrait step of onboarding (PRD 005 §5 step 3).
//
// **Wraps PRD 003's `PhotoCapture.vue`; it must not be forked.** That component owns the
// camera, the circular face guide, the retake affordance, the confirm-before-upload step and
// the `<input type="file" capture="user">` fallback — plus the distinction between "no secure
// context" and "permission denied", which matters on a dev stack reached over plain http where
// camera, service worker and geolocation all vanish at once. Reimplementing any of it here
// would give us two capture UIs that drift, and people will not accept a photo they did not
// get to approve.
//
// The purpose is stated, not negotiated: **consent is already held from sign-up** (PRD 005 §11
// / task 102), the basis is identification during the race, and portraits are purged after the
// event. So there is deliberately **no consent checkbox and no consent text** here — adding one
// would duplicate a permission already held and imply the sign-up consent was insufficient.
//
// **Skippable.** Only login is mandatory (PRD 005 §6), and the profile page is where a member
// can add a portrait later. It runs for every user with no portrait, *including* those whose
// profile confirmation was skipped because they had already started the event: verification says
// something about the guardian number and nothing about whether there is a face on file, and a
// member already on the trail without a portrait is exactly the person personnel will fail to
// identify at 03:00 (PRD 005 §11, 2026-08-25).

const emit = defineEmits<{ done: []; skip: [] }>()

// Capture, upload, retry-the-same-photo and the error copy are shared with the post-onboarding
// nudge (task 146) — see the composable for why `pending` matters.
const { capturing, uploading, error, pending, open, upload, retry } = usePortraitCapture()

async function send(blob: Blob) {
  // Never blocks: on failure the retry is offered and "spring over" stays available.
  if (await upload(blob)) emit('done')
}
</script>

<template>
  <div class="flex h-full flex-col justify-center gap-6">
    <div class="flex flex-col items-center gap-4 text-center">
      <div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-slate-900 text-white">
        <Camera class="h-7 w-7" aria-hidden="true" />
      </div>
      <h1 class="font-nathejk text-3xl tracking-wide">Tag et billede af dig selv</h1>
      <!--
        The purpose, in the terms that make it make sense to a participant: much of the race
        runs at night, and the people they meet need to know who they are talking to. Not a
        profile picture — an operational one.
      -->
      <p class="text-sm leading-relaxed text-slate-600">
        Billedet bruges til at genkende dig under løbet. Meget af Nathejk foregår i mørke, hvor
        det er svært at se ansigter — så samarit, guide og postmandskab kan se, hvem de står
        overfor, uden at du skal stave dit navn.
      </p>
    </div>

    <div class="flex flex-col items-stretch gap-2">
      <button
        type="button"
        class="rounded-lg bg-slate-900 px-4 py-3 font-medium text-white"
        @click="open"
      >
        Tag billede
      </button>

      <p v-if="error" class="text-center text-sm text-red-600" role="alert">{{ error }}</p>
      <button
        v-if="error && pending"
        type="button"
        class="px-2 py-2 text-sm font-medium text-slate-700"
        :disabled="uploading"
        @click="retry"
      >
        Prøv igen
      </button>

      <button type="button" class="px-2 py-2 text-sm text-slate-500" @click="emit('skip')">
        Spring over
      </button>
      <p class="text-center text-xs leading-relaxed text-slate-400">
        Du kan altid tilføje billedet senere under Min profil.
      </p>
    </div>

    <!-- PRD 003's capture component, unforked: camera, face guide, retake, confirm, and the
         file-input fallback when the camera is denied or unavailable. -->
    <PhotoCapture v-if="capturing" @captured="send" @cancel="capturing = false" />
  </div>
</template>
