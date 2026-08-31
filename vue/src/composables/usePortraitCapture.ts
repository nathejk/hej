import { ref } from 'vue'

import { useProfileStore } from '@/stores/profile.store'

// Capture-and-upload a portrait, shared by the two places PRD 005 asks for one: the onboarding
// step (task 129) and the post-onboarding nudge (task 146).
//
// Extracted because both need the same four pieces of state and the same failure handling, and the
// interesting part is not the happy path — it is `pending`, which is easy to leave out and costs a
// tired teenager a retake every time the network drops.
//
// It does **not** own the camera. `components/profile/PhotoCapture.vue` (PRD 003) does, and stays
// the single implementation: this only handles what happens to the blob it emits.
//
// `ProfilePhoto.vue` deliberately still has its own copy. It is PRD 003's shipped component with a
// different surrounding contract (it renders the current portrait, and its errors belong to a page
// rather than to a step), so folding it in here would mean editing shipped code to serve a
// refactor rather than a requirement. Worth revisiting if a fourth caller appears.
export function usePortraitCapture() {
  const profile = useProfileStore()

  /** The full-screen capture UI is open. */
  const capturing = ref(false)
  const uploading = ref(false)
  const error = ref('')

  // The last blob we tried to send, kept so a retry re-sends *that photo* rather than making the
  // member take another one. What failed was the upload, not the picture — and this runs over rural
  // mobile data, at night, so failures are expected rather than exceptional.
  const pending = ref<Blob | null>(null)

  function open() {
    error.value = ''
    capturing.value = true
  }

  function cancel() {
    capturing.value = false
  }

  /**
   * Sends a captured photo. Returns whether it landed.
   *
   * Never throws: every caller's next move is to show something, not to unwind. A failure leaves
   * `pending` set so `retry()` is available, and leaves `capturing` open so the member can see what
   * happened rather than being dropped back to a screen with no explanation.
   */
  async function upload(blob: Blob): Promise<boolean> {
    pending.value = blob
    uploading.value = true
    error.value = ''
    try {
      await profile.uploadPhoto(blob)
      capturing.value = false
      pending.value = null
      return true
    } catch {
      error.value = 'Billedet kunne ikke sendes. Prøv igen, eller gør det senere.'
      return false
    } finally {
      uploading.value = false
    }
  }

  /** Re-sends the photo that failed. No-op when there is nothing pending. */
  async function retry(): Promise<boolean> {
    const blob = pending.value
    if (!blob || uploading.value) return false
    return upload(blob)
  }

  return { capturing, uploading, error, pending, open, cancel, upload, retry }
}
