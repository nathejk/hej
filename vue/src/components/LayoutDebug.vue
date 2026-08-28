<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

// Diagnostic overlay for the iOS standalone layout questions that a screenshot cannot
// answer. Gated by the BFF's show_layout_debug (see @/config/runtime) — off unless
// SHOW_LAYOUT_DEBUG is set.
//
// It exists because those questions were repeatedly settled by guesswork: measuring a
// scaled screenshot to decide whether a blank strip was our layout reserving space or
// iOS drawing its own chrome produced a confidently wrong conclusion more than once.
//
// `mainTop` is the decisive value: on a full-bleed route the map should start at the
// very top of the screen, so 0 means a blank band above it is iOS chrome (status bar
// style / manifest), while ~the top inset means something in our own layout is still
// reserving it.

const vp = ref('')
const scr = ref('')
const dpr = ref('')
const insets = ref('')
const mainTop = ref('')
const mode = ref('')

// Safe-area insets cannot be read from JS: `env()` is only valid in CSS values, and
// reading a custom property that references it returns the unsubstituted token. So
// resolve them by measuring an off-screen probe whose padding is set from env() and
// reading back the computed pixel values.
function readInsets(): string {
  const probe = document.createElement('div')
  probe.style.cssText = [
    'position:fixed',
    'top:0',
    'left:0',
    'width:0',
    'height:0',
    'visibility:hidden',
    'pointer-events:none',
    'padding-top:env(safe-area-inset-top)',
    'padding-right:env(safe-area-inset-right)',
    'padding-bottom:env(safe-area-inset-bottom)',
    'padding-left:env(safe-area-inset-left)',
  ].join(';')
  document.body.appendChild(probe)
  const cs = getComputedStyle(probe)
  const px = (v: string) => Math.round(Number.parseFloat(v) || 0)
  const out = `${px(cs.paddingTop)}/${px(cs.paddingRight)}/${px(cs.paddingBottom)}/${px(cs.paddingLeft)}`
  probe.remove()
  return out
}

function sample() {
  vp.value = `${window.innerWidth}x${window.innerHeight}`
  scr.value = `${window.screen.width}x${window.screen.height}`
  dpr.value = String(window.devicePixelRatio)
  insets.value = readInsets()

  const main = document.querySelector('main')
  mainTop.value = main ? String(Math.round(main.getBoundingClientRect().top)) : '-'

  mode.value = window.matchMedia('(display-mode: standalone)').matches ? 'standalone' : 'browser'
}

let frame = 0
function schedule() {
  cancelAnimationFrame(frame)
  // Next frame, so a resize/orientation change has actually been laid out before it is
  // measured — sampling synchronously reports the pre-change geometry.
  frame = requestAnimationFrame(sample)
}

onMounted(() => {
  schedule()
  window.addEventListener('resize', schedule)
  window.addEventListener('orientationchange', schedule)
})

onBeforeUnmount(() => {
  cancelAnimationFrame(frame)
  window.removeEventListener('resize', schedule)
  window.removeEventListener('orientationchange', schedule)
})
</script>

<template>
  <!-- Teleported and fixed, so it is outside the shell's flow entirely and cannot move
       or resize any of the geometry it is measuring — the overlay must not perturb its
       own subject. pointer-events-none keeps it from swallowing taps; aria-hidden keeps
       it out of the accessibility tree.

       Pinned to the middle-left rather than a corner: the top band and the bottom nav
       are usually what is under investigation, so corners are the worst place to sit.

       One <div> per line rather than newlines in a single text node: Vue's template
       compiler condenses whitespace, so the newline version collapsed into one wide
       strip that covered more of the screen than it needed to. -->
  <Teleport to="body">
    <div
      class="pointer-events-none fixed top-1/2 left-0 z-[70] -translate-y-1/2 rounded-r bg-slate-900/80 px-1.5 py-1 font-mono text-[9px] leading-[1.5] text-white"
      aria-hidden="true"
    >
      <div>vp {{ vp }}</div>
      <div>scr {{ scr }} @{{ dpr }}</div>
      <div>sa {{ insets }}</div>
      <div>main.top {{ mainTop }}</div>
      <div>{{ mode }}</div>
    </div>
  </Teleport>
</template>
