<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { Camera, X } from '@lucide/vue'

import PhotoCapture from '@/components/profile/PhotoCapture.vue'
import { usePortraitCapture } from '@/composables/usePortraitCapture'
import { showPortraitNudge } from '@/config/nudge'
import { useAppStore } from '@/stores/app.store'
import { useProfileStore } from '@/stores/profile.store'

// The post-onboarding portrait nudge (PRD 005 §6, task 146).
//
// The onboarding step is skippable, and PRD 005 §11 is explicit that asking once is not enough: the
// members most likely to skip are exactly the ones who then stay unidentifiable, and the photo
// exists so that a samarit at 03:00 can tell who they are talking to. So the ask comes back.
//
// Three properties, each of which is the answer to a way this could go wrong:
//
//   * **Dismissible per session**, in memory. Permanent dismissal recreates the one-shot problem
//     under a different name; no dismissal at all trains people to ignore it.
//   * **Ends for good once a portrait exists**, driven by `hasPhoto` rather than a stored flag —
//     so it cannot get stuck on for someone who has already complied.
//   * **In the shell's normal content flow**, never an overlay. It pushes content down and can be
//     read at a glance; it cannot cover anything. `showPortraitNudge` decides where, and
//     `config/nudge.ts` carries the reasoning for each exclusion.
//
// It is the app's only *active* nudge surface, per PRD 005 §6. The initials fallback on the user
// menu's avatar is passive and does not count — that comment in `UserMenu.vue` predates this.

const app = useAppStore()
const profile = useProfileStore()
const route = useRoute()

const { capturing, uploading, error, pending, open, cancel, upload, retry } = usePortraitCapture()

const visible = computed(() =>
  showPortraitNudge({
    hasPhoto: profile.hasPhoto,
    profileLoaded: profile.loaded,
    dismissed: app.portraitNudgeDismissed,
    routeName: typeof route.name === 'string' ? route.name : null,
    fullBleed: route.meta.fullBleed === true,
  }),
)

// No `emit('done')` equivalent and nothing to advance: a successful upload sets `hasPhoto`, which
// makes `visible` false. The nudge removing itself because the thing it asked for now exists is the
// whole design — there is no completion state to record.
</script>

<template>
  <div v-if="visible" class="px-4 pt-3">
    <div class="flex items-start gap-3 rounded-xl border border-slate-200 bg-white p-3 shadow-xs">
      <Camera class="mt-0.5 h-5 w-5 shrink-0 text-slate-500" aria-hidden="true" />

      <div class="min-w-0 flex-1">
        <p class="text-sm font-medium text-slate-800">Mangler dit billede</p>
        <!--
          States the purpose rather than just asking. "Tag et billede" alone reads as vanity; the
          reason it matters is that much of the race happens in the dark, and the people who may
          need to find you cannot recognise a name badge at 03:00.
        -->
        <p class="mt-1 text-xs leading-relaxed text-slate-500">
          Samarit, guide og postmandskab bruger billedet til at genkende dig i mørket. Det tager
          et øjeblik.
        </p>

        <p v-if="error" class="mt-2 text-xs text-red-600" role="alert">{{ error }}</p>

        <div class="mt-2 flex items-center gap-2">
          <button
            type="button"
            class="rounded-lg bg-slate-900 px-3 py-1.5 text-sm font-medium text-white"
            @click="open"
          >
            Tag billede
          </button>
          <button
            v-if="error && pending"
            type="button"
            class="px-2 py-1.5 text-sm font-medium text-slate-700 disabled:opacity-50"
            :disabled="uploading"
            @click="retry"
          >
            Prøv igen
          </button>
        </div>
      </div>

      <!--
        Dismissal is a real button with a label, not a bare glyph: it is the affordance that keeps
        this from being nagging, so it must be obvious and reachable. 44px tap target per task 010.
      -->
      <button
        type="button"
        class="-m-2 flex min-h-11 min-w-11 items-center justify-center rounded-full text-slate-400"
        aria-label="Skjul påmindelsen"
        @click="app.dismissPortraitNudge()"
      >
        <X class="h-4 w-4" aria-hidden="true" />
      </button>
    </div>

    <!-- PRD 003's capture component, unforked: camera, face guide, retake, confirm, fallback. -->
    <PhotoCapture v-if="capturing" @captured="upload" @cancel="cancel" />
  </div>
</template>
