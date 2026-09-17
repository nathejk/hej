import { ref } from 'vue'

import type { Glimt } from '@/stores/glimt.store'

// Opening the full-screen viewer (PRD 019 §7, task 318).
//
// Three refs and two functions, extracted only because **two surfaces open the same viewer** — the feed
// (task 316) and the hold grid (task 326) — and duplicating the state in both invites them to drift in
// the one way that matters: which item you land on.
//
// Deliberately not a Pinia store. Nothing outside the component tree needs to know a photograph is
// open, and it must not survive navigation: a viewer left open in the store would reappear over the
// next page.
export function useOpenGlimt() {
  const isOpen = ref(false)
  const glimt = ref<Glimt | null>(null)
  const ordinal = ref(0)

  /**
   * Open the viewer on one item of one glimt.
   *
   * The ordinal is passed rather than assumed to be the first, because that is the whole difference
   * between a viewer and a slideshow: tapping the third photograph in a grid opens the third one.
   */
  function open(entry: Glimt, at = 0) {
    glimt.value = entry
    ordinal.value = at
    isOpen.value = true
  }

  function close() {
    isOpen.value = false
  }

  return { isOpen, glimt, ordinal, open, close }
}
