<script setup lang="ts">
// Full-screen portrait capture (PRD 003 §7). **Owned here and reused by PRD 005's
// onboarding step — it must not be forked.**
//
// Scope: this component produces a photo. It does not upload, does not know about the
// profile store, and does not ask for consent — consent is already held from sign-up
// (task 102), so the copy explains the *purpose* rather than requesting permission.
// Keeping it upload-free is what lets onboarding reuse it.
//
// The face guide is a circle, matching the fixed circular crop decided in PRD 003 §11.
// It is a guide only: the emitted image is the full frame, so a badly framed shot is
// recoverable without re-taking it.
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Camera, RefreshCw, X } from '@lucide/vue'
import { blockedGuidance } from '@/config/permissions'

const emit = defineEmits<{
  /** A confirmed photo, already downscaled and re-oriented. */
  captured: [blob: Blob]
  cancel: []
}>()

// Longest edge of the emitted image. The server re-encodes and enforces its own limit
// (task 105); doing it here as well is what keeps the upload small on a rural mobile
// connection, which is the whole point of doing it on the client.
const MAX_EDGE = 1024
const JPEG_QUALITY = 0.85

const video = ref<HTMLVideoElement | null>(null)
const stream = ref<MediaStream | null>(null)
const facingMode = ref<'user' | 'environment'>('user')
const hasMultipleCameras = ref(false)
const starting = ref(false)
const error = ref('')
const blocked = ref(false)

// The confirmed-photo step. Holding a Blob plus an object URL rather than a data URL:
// a 1024px JPEG as base64 is ~1.4 MB of string, and this runs on a phone.
const preview = ref<{ blob: Blob; url: string } | null>(null)

const canFlip = computed(() => hasMultipleCameras.value && !preview.value)

// getUserMedia exists only in a **secure context**. Checked explicitly rather than left to
// the catch below, because the failure it produces is a bare TypeError and the message
// would then be the generic "kunne ikke startes" — which sends someone hunting for a
// camera-permission problem that does not exist. The realistic way to hit this is reaching
// a dev stack over plain http (an IP address, say), where the camera, the service worker
// and geolocation all quietly stop existing at once.
const cameraSupported = () =>
  typeof navigator !== 'undefined' && Boolean(navigator.mediaDevices?.getUserMedia)

async function start() {
  if (!cameraSupported()) {
    error.value = window.isSecureContext
      ? 'Denne browser giver ikke adgang til kameraet. Brug knappen nedenfor.'
      : 'Kameraet kræver en sikker forbindelse (https). Brug knappen nedenfor.'
    return
  }

  starting.value = true
  error.value = ''
  blocked.value = false
  try {
    stop()
    const media = await navigator.mediaDevices.getUserMedia({
      // Front camera by default: this is a self-portrait. `facingMode` as a plain value
      // rather than `exact`, so a laptop or a phone with one camera still works instead
      // of failing with OverconstrainedError.
      video: { facingMode: facingMode.value, width: { ideal: 1280 }, height: { ideal: 1280 } },
      audio: false,
    })
    stream.value = media
    if (video.value) {
      video.value.srcObject = media
      // iOS Safari will not autoplay an inline video without an explicit play() after
      // the stream is attached.
      await video.value.play().catch(() => {})
    }
    // Enumerated only after permission is granted: before that, labels and often the
    // device list itself are withheld, so a flip button would appear or not depending
    // on something the user cannot see.
    const devices = await navigator.mediaDevices.enumerateDevices()
    hasMultipleCameras.value = devices.filter((d) => d.kind === 'videoinput').length > 1
  } catch (err) {
    const name = err instanceof DOMException ? err.name : ''
    if (name === 'NotAllowedError' || name === 'SecurityError') {
      // Denied means the browser will not ask again, so an enable button would be a
      // dead end. Task 101 owns the platform-correct instructions.
      blocked.value = true
      error.value = blockedGuidance('camera')
    } else if (name === 'NotFoundError' || name === 'OverconstrainedError') {
      error.value = 'Vi kunne ikke finde et kamera på denne enhed.'
    } else {
      error.value = 'Kameraet kunne ikke startes. Prøv igen, eller brug knappen nedenfor.'
    }
  } finally {
    starting.value = false
  }
}

// stop is called from every exit path — cancel, confirm, unmount, and before a flip.
// A live camera left running is both a battery drain and, on a phone in a pocket, a
// privacy problem: the indicator light stays on.
function stop() {
  stream.value?.getTracks().forEach((track) => track.stop())
  stream.value = null
  if (video.value) video.value.srcObject = null
}

async function flip() {
  facingMode.value = facingMode.value === 'user' ? 'environment' : 'user'
  await start()
}

// shoot draws the current frame to a canvas, downscaled.
//
// Drawing from the live <video> is also what fixes orientation: the element renders the
// frame the way it is displayed, so the rotation an EXIF tag would have described is
// already applied. That matters because the server re-encodes and therefore drops EXIF
// (task 105) — an image relying on the tag would arrive sideways.
async function shoot() {
  const el = video.value
  if (!el || !el.videoWidth) return

  const { videoWidth: w, videoHeight: h } = el
  const scale = Math.min(1, MAX_EDGE / Math.max(w, h))
  const canvas = document.createElement('canvas')
  canvas.width = Math.round(w * scale)
  canvas.height = Math.round(h * scale)

  const ctx = canvas.getContext('2d')
  if (!ctx) {
    error.value = 'Billedet kunne ikke behandles på denne enhed.'
    return
  }
  ctx.drawImage(el, 0, 0, canvas.width, canvas.height)

  const blob = await new Promise<Blob | null>((resolve) =>
    canvas.toBlob(resolve, 'image/jpeg', JPEG_QUALITY),
  )
  if (!blob) {
    error.value = 'Billedet kunne ikke gemmes. Prøv igen.'
    return
  }

  setPreview(blob)
  // The camera is not needed while the user decides. Stopping it here rather than on
  // confirm saves battery and turns the indicator off during a step that can take a
  // while — and `retake()` starts it again.
  stop()
}

function setPreview(blob: Blob) {
  releasePreview()
  preview.value = { blob, url: URL.createObjectURL(blob) }
}

function releasePreview() {
  if (preview.value) URL.revokeObjectURL(preview.value.url)
  preview.value = null
}

async function retake() {
  releasePreview()
  await start()
}

function confirm() {
  if (!preview.value) return
  const { blob } = preview.value
  stop()
  // The URL is revoked, but the Blob survives it — the parent owns the bytes now.
  releasePreview()
  emit('captured', blob)
}

function cancel() {
  stop()
  releasePreview()
  emit('cancel')
}

// Fallback for a browser that cannot do getUserMedia, or where the user has blocked it
// permanently. `capture="user"` asks the OS camera app for a front-facing shot; the file
// is not downscaled here because the server does it anyway, and a File is not a canvas.
function onFallbackFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (file) setPreview(file)
  input.value = ''
}

onUnmounted(() => {
  stop()
  releasePreview()
})

// Started on mount, not during setup: the stream is attached to the <video> element, so
// the ref has to exist by the time getUserMedia resolves. It does either way in practice,
// but depending on that ordering is how a working camera turns into a black rectangle
// after an unrelated refactor.
//
// This component is only mounted in response to the user tapping the portrait, so this is
// not an unprompted camera access.
onMounted(() => {
  void start()
})
</script>

<template>
  <!-- Fixed and above everything: this is a full-screen surface, and on a phone the
       bottom nav must not sit on top of the shutter. -->
  <div class="fixed inset-0 z-50 flex flex-col bg-black text-white">
    <header
      class="flex items-center justify-between px-4 pb-3"
      style="padding-top: calc(var(--sat) + 0.75rem)"
    >
      <p class="font-nathejk text-lg tracking-wide">Tag et billede</p>
      <button
        type="button"
        class="flex min-h-11 min-w-11 items-center justify-center rounded-full"
        aria-label="Luk"
        @click="cancel"
      >
        <X class="h-6 w-6" aria-hidden="true" />
      </button>
    </header>

    <div class="relative min-h-0 flex-1">
      <!-- Preview of the taken shot. -->
      <img
        v-if="preview"
        :src="preview.url"
        alt="Billedet du har taget"
        class="h-full w-full object-contain"
      />

      <template v-else>
        <!-- muted + playsinline are required for inline autoplay on iOS. -->
        <video
          ref="video"
          class="h-full w-full object-cover"
          :class="{ '-scale-x-100': facingMode === 'user' }"
          autoplay
          muted
          playsinline
        ></video>

        <!-- Face guide: the circular framing guide PRD 003 §11 decided on, and the one
             PRD 005's onboarding step assumes. Purely visual — the emitted image is the
             full frame. -->
        <div
          v-if="stream"
          class="pointer-events-none absolute inset-0 flex items-center justify-center"
          aria-hidden="true"
        >
          <div class="aspect-square w-2/3 max-w-72 rounded-full border-2 border-white/70"></div>
        </div>

        <p
          v-if="starting"
          class="absolute inset-0 flex items-center justify-center text-sm text-white/80"
        >
          Starter kamera …
        </p>
      </template>
    </div>

    <div class="space-y-3 px-4 pt-3" style="padding-bottom: calc(var(--sab) + 1rem)">
      <p v-if="error" class="rounded-lg bg-white/10 px-3 py-2 text-sm text-white/90">
        {{ error }}
      </p>
      <p v-else-if="!preview" class="text-center text-sm text-white/70">
        Billedet bruges til at genkende dig under løbet — også når det er mørkt.
      </p>

      <!-- Confirm step. -->
      <div v-if="preview" class="flex items-center gap-3">
        <button
          type="button"
          class="min-h-11 flex-1 rounded-lg bg-white px-4 py-2 font-medium text-slate-900"
          @click="confirm"
        >
          Brug billede
        </button>
        <button
          type="button"
          class="min-h-11 rounded-lg border border-white/40 px-4 py-2"
          @click="retake"
        >
          Tag igen
        </button>
      </div>

      <!-- Capture step. -->
      <div v-else class="flex items-center justify-center gap-6">
        <button
          v-if="canFlip"
          type="button"
          class="flex min-h-11 min-w-11 items-center justify-center rounded-full border border-white/40"
          aria-label="Skift kamera"
          @click="flip"
        >
          <RefreshCw class="h-5 w-5" aria-hidden="true" />
        </button>

        <button
          type="button"
          class="flex h-16 w-16 items-center justify-center rounded-full bg-white text-slate-900 disabled:opacity-40"
          aria-label="Tag billede"
          :disabled="!stream"
          @click="shoot"
        >
          <Camera class="h-7 w-7" aria-hidden="true" />
        </button>

        <!-- Kept the same size as the flip button so the shutter stays centred. -->
        <span v-if="canFlip" class="min-h-11 min-w-11" aria-hidden="true"></span>
      </div>

      <!-- The fallback is always reachable, not only after a failure: on a device where
           the live camera is blocked in settings, this is the only way through, and
           discovering that after an error message is worse than seeing it up front. -->
      <label
        v-if="!preview"
        class="block text-center text-sm text-white/60 underline underline-offset-2"
      >
        Brug telefonens kamera i stedet
        <input type="file" accept="image/*" capture="user" class="sr-only" @change="onFallbackFile" />
      </label>
    </div>
  </div>
</template>
