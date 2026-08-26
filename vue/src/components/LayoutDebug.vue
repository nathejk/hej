<script setup lang="ts">
// TEMPORARY layout diagnostic (task 036).
//
// Delete this component once the bottom-spacing issue is resolved. It exists because the
// excess space below the bottom nav could not be reproduced off-device — desktop Chromium
// reports zero safe-area insets and does not share iOS's standalone viewport behaviour — and
// two hypotheses have already been wrong. Guessing a third time is worse than measuring.
//
// Everything here is read from the live document, so one screenshot of this panel answers
// which layer is over-reserving.
import { onMounted, ref } from 'vue'

const rows = ref<[string, string][]>([])

// env() cannot be read with getComputedStyle directly, so measure it with a probe element
// whose height *is* the inset.
function measureInset(side: 'top' | 'bottom'): number {
  const probe = document.createElement('div')
  probe.style.cssText = `position:fixed;left:0;width:0;visibility:hidden;height:env(safe-area-inset-${side})`
  document.body.appendChild(probe)
  const h = probe.getBoundingClientRect().height
  probe.remove()
  return h
}

function rect(selector: string): DOMRect | null {
  const el = document.querySelector(selector)
  return el ? el.getBoundingClientRect() : null
}

onMounted(() => {
  const px = (n: number | undefined) => (n === undefined ? '—' : `${Math.round(n * 10) / 10}px`)

  const nav = rect('nav[aria-label="Hovednavigation"]')
  // The flex column that holds header + main + nav.
  const shell = rect('#app > div')
  const vv = window.visualViewport

  // Root font size, because every nav dimension is in rem: `min-h-[3.25rem]`, `py-2`,
  // `text-xs`. If iOS is scaling text up (accessibility Larger Text, or Display Zoom), the
  // whole nav row grows and would look exactly like an over-reserved inset.
  const rootFont = getComputedStyle(document.documentElement).fontSize
  const navEl = document.querySelector('nav[aria-label="Hovednavigation"]')
  const navStyle = navEl ? getComputedStyle(navEl) : null
  const firstItem = rect('nav[aria-label="Hovednavigation"] > *')

  const out: [string, string][] = [
    ['display-mode standalone', String(window.matchMedia('(display-mode: standalone)').matches)],
    ['navigator.standalone', String((navigator as { standalone?: boolean }).standalone ?? '—')],
    ['root font-size', rootFont],
    ['devicePixelRatio', String(window.devicePixelRatio)],
    ['screen.height', px(window.screen.height)],
    ['window.innerHeight', px(window.innerHeight)],
    ['documentElement.clientHeight', px(document.documentElement.clientHeight)],
    ['visualViewport.height', px(vv?.height)],
    ['visualViewport.offsetTop', px(vv?.offsetTop)],
    ['inset-top (env)', px(measureInset('top'))],
    ['inset-bottom (env)', px(measureInset('bottom'))],
    ['nav computed padding-bottom', navStyle?.paddingBottom ?? '—'],
    ['nav item height (row)', px(firstItem?.height)],
    ['shell height', px(shell?.height)],
    ['shell bottom', px(shell?.bottom)],
    ['nav height', px(nav?.height)],
    ['nav top', px(nav?.top)],
    ['nav bottom', px(nav?.bottom)],
    ['gap: innerHeight − nav.bottom', px(nav ? window.innerHeight - nav.bottom : undefined)],
    ['body scrollHeight', px(document.body.scrollHeight)],
  ]
  rows.value = out
})
</script>

<template>
  <section class="rounded-xl border border-amber-300 bg-amber-50 p-4">
    <h2 class="font-medium text-amber-900">Layout-diagnostik (midlertidig)</h2>
    <p class="mt-1 text-xs text-amber-800">
      Bruges til at finde den ekstra plads under menuen. Fjernes igen.
    </p>
    <dl class="mt-3 space-y-1">
      <div v-for="[k, v] of rows" :key="k" class="flex justify-between gap-3 text-xs">
        <dt class="text-amber-900">{{ k }}</dt>
        <dd class="font-mono text-amber-950">{{ v }}</dd>
      </div>
    </dl>
  </section>
</template>
