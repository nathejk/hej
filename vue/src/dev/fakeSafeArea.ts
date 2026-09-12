import { ref } from 'vue'

import {
  SAFE_AREA_VARS,
  reapplySafeArea,
  setDevInsetProvider,
  type SimulatedInsets,
} from '@/helpers/safeArea'

// Simulated safe-area insets (PRD 014, task 211).
//
// On a laptop every inset is 0, so the class of bug they exist to prevent — content under the
// status bar, the bottom nav under the home indicator — is invisible until the build reaches a
// handset. `LayoutDebug` can already *report* the values; it cannot produce them.
//
// # The numbers are measured, not invented
//
// The portrait preset is the reading recorded in `@/helpers/safeArea`'s header from an iPhone 16
// in standalone: screen 393x852, innerHeight 793, insets 59/0/34/0 — hence a shortfall of 59.
// Carrying the shortfall with the insets matters: it is what makes the simulation reproduce the
// **bottom-inset reduction** that module performs, so `--sab` comes out at 0 exactly as it does
// on the real device. A preset that invented a shortfall of 0 would show 34px of bottom padding
// no iPhone actually gets, and "the bottom nav takes up too much space" would look like a bug
// that had come back.
//
// The landscape numbers are the standard notch-in-landscape shape: no top inset, symmetric
// left/right, a reduced bottom. Those are the insets nobody remembers, which is the argument for
// having the preset at all.

export type SafeAreaPreset = 'none' | 'iphone-portrait' | 'iphone-landscape' | 'ipad'

const PRESETS: Record<Exclude<SafeAreaPreset, 'none'>, SimulatedInsets> = {
  'iphone-portrait': { insets: { top: 59, right: 0, bottom: 34, left: 0 }, shortfall: 59 },
  'iphone-landscape': { insets: { top: 0, right: 47, bottom: 21, left: 47 }, shortfall: 0 },
  ipad: { insets: { top: 24, right: 0, bottom: 20, left: 0 }, shortfall: 0 },
}

const active = ref<SafeAreaPreset>('none')

/** Which preset is being simulated, for the panel's readout. */
export const safeAreaPreset = active

export function initFakeSafeArea() {
  setDevInsetProvider(() => (active.value === 'none' ? null : PRESETS[active.value]))
}

/**
 * Switches preset and re-applies.
 *
 * Returning to `none` has to remove the inline properties explicitly, and the reason is worth
 * knowing: `insetVars()` **discards an all-zero reading** (see its comment — writing a static 0
 * over a live `env()` seed is how the top bar once ended up behind the status bar). On a laptop
 * the real reading *is* all-zero, so `apply()` writes nothing and the simulated values would
 * survive switching off. Removing the inline properties restores main.css's live seeds, which is
 * the correct "no override" state rather than a frozen 0.
 */
export function setSafeAreaPreset(preset: SafeAreaPreset) {
  active.value = preset
  if (preset === 'none') {
    const root = document.documentElement.style
    for (const name of SAFE_AREA_VARS) root.removeProperty(name)
  }
  reapplySafeArea()
}

export const SAFE_AREA_PRESETS: SafeAreaPreset[] = [
  'none',
  'iphone-portrait',
  'iphone-landscape',
  'ipad',
]

/** Exported for the spec, which checks the presets against the real inset rule. */
export const SIMULATED_INSETS = PRESETS
