<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

// Diagnostic overlay for the iOS standalone layout questions that a screenshot cannot
// answer. Gated by the BFF's show_layout_debug (see @/config/runtime) — off unless
// SHOW_LAYOUT_DEBUG is set.
//
// It exists because those questions were repeatedly settled by guesswork: measuring a
// scaled screenshot to decide whether a blank strip was our layout reserving space or
// iOS drawing its own chrome produced a confidently wrong conclusion more than once.
//
// The two derived values are the ones worth reading:
//
//   app.h     the shell's height, and the document's scrollable height next to it. They
//             must be equal: if the document is taller, the whole shell can be dragged.
//   nav.gap   distance from the bottom of the nav to the bottom of the screen. This is
//             what the bottom inset protects, so it must stay >= the raw sa.bottom (34 on
//             a device with a home indicator) even after the padding reduction.
//   main.top  where the shell's <main> starts. On a full-bleed route it should be 0; a
//             non-zero value means something in our own layout is reserving space.

const route = useRoute()

const vp = ref('')
const scr = ref('')
const dpr = ref('')
const insets = ref('')
const effective = ref('')
const mainTop = ref('')
const mode = ref('')
const appH = ref('')
const navGap = ref('')
const where = ref('')

// Safe-area insets cannot be read from JS: `env()` is only valid in CSS values, and
// reading a custom property that references it returns the unsubstituted token. So
// resolve them by measuring an off-screen probe whose padding is set from env() and
// reading back the computed pixel values.
//
// Deliberately reads the RAW `env()` values, not our `var(--sat)` custom properties:
// this overlay's job is to report what the platform says, so that the correction applied
// in @/helpers/safeArea can be checked against it rather than assumed. The `sat` line
// below shows the corrected value for comparison.
function readInsets(): { text: string; top: number } {
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
  const top = px(cs.paddingTop)
  const text = `${top}/${px(cs.paddingRight)}/${px(cs.paddingBottom)}/${px(cs.paddingLeft)}`
  probe.remove()
  return { text, top }
}

function sample() {
  vp.value = `${window.innerWidth}x${window.innerHeight}`
  scr.value = `${window.screen.width}x${window.screen.height}`
  dpr.value = String(window.devicePixelRatio)

  const { text, top } = readInsets()
  insets.value = text

  // What the app is actually using.
  const rootStyle = getComputedStyle(document.documentElement)
  const v = (name: string) => Math.round(Number.parseFloat(rootStyle.getPropertyValue(name)) || 0)
  effective.value = `${v('--sat')}/${v('--sar')}/${v('--sab')}/${v('--sal')}`

  const el = document.getElementById('app')
  // Shell height vs the document's scroll height. A difference is document overflow, i.e.
  // the whole app becomes draggable — the failure mode that reverting the height hack fixed.
  const h = el ? Math.round(el.getBoundingClientRect().height) : 0
  appH.value = `${h} doc ${document.documentElement.scrollHeight}`

  // How much clear space is left below the nav, in screen terms. The bottom inset is
  // reduced by the viewport's shortfall (see @/helpers/safeArea), so this is the check
  // that the reduction did not eat into the home-indicator clearance it assumes.
  const nav = document.querySelector('nav[aria-label]')
  if (nav) {
    const bottom = nav.getBoundingClientRect().bottom
    navGap.value = `${Math.round(window.screen.height - bottom)}`
  } else {
    navGap.value = 'no nav'
  }

  void top

  const main = document.querySelector('main')
  mainTop.value = main ? String(Math.round(main.getBoundingClientRect().top)) : 'no main'

  mode.value = window.matchMedia('(display-mode: standalone)').matches ? 'standalone' : 'browser'
  // Which route the numbers describe. Without this, a stale sample from another screen
  // is indistinguishable from a real measurement of the current one — the mistake that
  // made the first reading of main.top misleading.
  where.value = String(route.name ?? '?')
}

let frame = 0
function schedule() {
  cancelAnimationFrame(frame)
  // Next frame, so a resize/orientation/route change has actually been laid out before
  // it is measured — sampling synchronously reports the pre-change geometry.
  frame = requestAnimationFrame(sample)
}

// Re-sample on navigation, not just on resize. The shell's geometry differs per route
// (full-bleed routes drop the header), so a value captured once at startup describes
// whichever screen happened to be mounted then.
watch(() => route.fullPath, schedule)

onMounted(() => {
  schedule()
  window.addEventListener('resize', schedule)
  window.addEventListener('orientationchange', schedule)
  document.addEventListener('visibilitychange', schedule)
})

onBeforeUnmount(() => {
  cancelAnimationFrame(frame)
  window.removeEventListener('resize', schedule)
  window.removeEventListener('orientationchange', schedule)
  document.removeEventListener('visibilitychange', schedule)
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
      <div>use {{ effective }}</div>
      <div>app.h {{ appH }}</div>
      <div>nav.gap {{ navGap }}</div>
      <div>main.top {{ mainTop }}</div>
      <div>{{ mode }} / {{ where }}</div>
    </div>
  </Teleport>
</template>
