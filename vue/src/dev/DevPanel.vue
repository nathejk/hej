<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import {
  DEV_KEY_PREFIX,
  DEV_PRESETS,
  applyPreset,
  readDevDevice,
  type DevDevice,
} from '@/dev/devDevice'
import {
  installPlatform,
  isMobileDevice,
  isStandalone,
  type PlatformEnv,
  type PlatformNavigator,
} from '@/helpers/platform'
import { detectPlatform } from '@/config/permissions'
import { gatesEnabled } from '@/config/gates'
import { installGateEnabled } from '@/config/runtime'

// The developer-facing control surface for PRD 014's simulation layer. Rendered only under
// `import.meta.env.DEV` (see App.vue), so it is absent from a production bundle entirely.
//
// # The readout is the point, not the toggles
//
// Tasks 206/207 give the app a way to lie about the hardware, and a *silent* lie is a
// debugging trap: an override left on last week produces a bug the developer then hunts in
// application code. So this panel's first job is to make the lie impossible to miss — the
// collapsed handle names the active profile, and the readout shows the simulated value
// alongside the real one wherever they differ. "Simulated iOS (real: MacIntel, no touch)" is
// the useful line; "iOS" alone is the trap.
//
// It also gathers the three switches that between them decide whether the app is reachable at
// all — the served `install_gate`, `?nogate=`'s stored bypass, and the resulting
// `gatesEnabled()` — which were previously only discoverable by reading localStorage by hand.
//
// # Deliberately not product UI
//
// Monospace, high-contrast, no brand font, and **no shadcn-vue component**. This is the one
// place in the repo where `.rules`' "prefer a standard shadcn-vue component whenever one
// exists" is inverted, and the reason is that fitting in is the anti-requirement: an overlay
// that looks like the app will eventually be mistaken for it in a screenshot, a demo or a bug
// report. `components/ui/drawer/` would fit mechanically and is deliberately not used.
//
// # Privacy
//
// This panel displays **no member data**. Not a name, not a role, and never a guardian phone
// number (`.rules`). The only personal datum it may ever show is the phone number the
// developer is themselves typing into the login form (task 216). The dev database holds real
// personal data about minors replayed from the broker, and a debug overlay is exactly the
// surface that rule exists to pre-empt.

const PANEL_OPEN_KEY = `${DEV_KEY_PREFIX}panel`

const open = ref(false)
const profile = ref<DevDevice | null>(null)

// The real environment, unsimulated. This is what the injectable `PlatformEnv` seam in
// @/helpers/platform is for: the panel can ask "what would you say without the profile?"
// without the profile having to be cleared first.
function realEnv(): PlatformEnv {
  return {
    navigator: navigator as unknown as PlatformNavigator,
    matchMedia: (query: string) => window.matchMedia(query),
  }
}

const presets = Object.keys(DEV_PRESETS)

// Effective values: what the app currently believes. These go through the ordinary default
// environment, so they include the simulation when one is active.
const effective = computed(() => ({
  mobile: isMobileDevice(),
  standalone: isStandalone(),
  install: installPlatform(),
  guidance: detectPlatform(),
}))

const real = computed(() => {
  const env = realEnv()
  return {
    mobile: isMobileDevice(env),
    standalone: isStandalone(env),
    install: installPlatform(env),
  }
})

const simulating = computed(() => profile.value !== null)

/** Short label for the collapsed handle: which preset-ish thing is being pretended. */
const label = computed(() => {
  const p = profile.value
  if (!p) return 'DEV'
  return `SIM ${p.platform}${p.standalone ? '' : ' tab'}`
})

const bypassed = ref(false)

// Referenced through a local, because template expressions cannot see Vite's build-time
// globals.
const buildId = __BUILD_ID__

function refresh() {
  profile.value = readDevDevice()
  try {
    bypassed.value = localStorage.getItem('hej.gates.bypass') === '1'
  } catch {
    bypassed.value = false
  }
}

function toggle() {
  open.value = !open.value
  try {
    localStorage.setItem(PANEL_OPEN_KEY, open.value ? '1' : '0')
  } catch {
    // Dev-only nuisance.
  }
}

// A device change reloads, and that is not laziness. The router gate decided the current
// route from the *old* answers, so a live switch would leave a page on screen that the gate
// would no longer allow — the app would look like it had ignored the toggle. A reload re-runs
// the whole chain, which is exactly what is being tested.
function choose(name: string) {
  applyPreset(name)
  window.location.reload()
}

onMounted(() => {
  try {
    open.value = localStorage.getItem(PANEL_OPEN_KEY) === '1'
  } catch {
    open.value = false
  }
  refresh()
})
</script>

<template>
  <!-- Teleported so it sits outside the shell's flow and cannot perturb the layout it sits
       on top of. Bottom-left, because LayoutDebug holds the middle-left and the bottom-right
       is where the nav's affordances live. Interactive, so unlike LayoutDebug it must NOT be
       pointer-events-none. aria-hidden: it is not part of the product's accessibility tree. -->
  <Teleport to="body">
    <div
      class="fixed bottom-0 left-0 z-[80] max-h-[70vh] max-w-[min(22rem,95vw)] overflow-auto rounded-tr border-t border-r border-lime-400 bg-black/90 font-mono text-[10px] leading-[1.6] text-lime-300"
      aria-hidden="true"
    >
      <button
        type="button"
        class="flex w-full items-center gap-2 px-2 py-1 text-left font-bold tracking-wide uppercase"
        :class="simulating ? 'bg-lime-400 text-black' : 'text-lime-300'"
        @click="toggle"
      >
        <span>{{ label }}</span>
        <span class="ml-auto">{{ open ? '▾' : '▸' }}</span>
      </button>

      <div v-if="open" class="space-y-2 px-2 pt-1 pb-2">
        <!-- State first, controls second: the question "what am I currently pretending to
             be?" is asked far more often than "change it". -->
        <div>
          <div class="text-lime-500">device</div>
          <div>
            mobile
            <b>{{ effective.mobile }}</b>
            <span v-if="simulating && effective.mobile !== real.mobile" class="text-amber-400">
              (real {{ real.mobile }})
            </span>
          </div>
          <div>
            standalone
            <b>{{ effective.standalone }}</b>
            <span
              v-if="simulating && effective.standalone !== real.standalone"
              class="text-amber-400"
            >
              (real {{ real.standalone }})
            </span>
          </div>
          <div>
            install
            <b>{{ effective.install }}</b>
            <span v-if="simulating && effective.install !== real.install" class="text-amber-400">
              (real {{ real.install }})
            </span>
          </div>
          <div>guidance <b>{{ effective.guidance }}</b></div>
          <div v-if="simulating" class="text-amber-400">simulated — not this machine</div>
        </div>

        <!-- The three switches that decide whether the app is reachable at all. `bypass` and
             `gates` are NOT the same thing as the simulation above, and conflating them is the
             mistake PRD 014 exists to correct: the bypass switches the gates off (onboarding
             redirect included), while a profile leaves them on and changes what they see. -->
        <div>
          <div class="text-lime-500">gates</div>
          <div>enabled <b>{{ gatesEnabled() }}</b></div>
          <div>install_gate <b>{{ installGateEnabled }}</b></div>
          <div>
            ?nogate bypass <b>{{ bypassed }}</b>
            <span v-if="bypassed" class="text-amber-400">— onboarding is NOT being tested</span>
          </div>
        </div>

        <div>
          <div class="text-lime-500">simulate</div>
          <div class="flex flex-wrap gap-1 pt-0.5">
            <button
              v-for="name in presets"
              :key="name"
              type="button"
              class="border border-lime-700 px-1 py-0.5 hover:bg-lime-400 hover:text-black"
              :class="{ 'bg-lime-400 text-black': profile?.platform === name }"
              @click="choose(name)"
            >
              {{ name }}
            </button>
          </div>
        </div>

        <div class="text-lime-600">build {{ buildId }}</div>
      </div>
    </div>
  </Teleport>
</template>
