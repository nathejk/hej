<script setup lang="ts">
import { computed, ref } from 'vue'
import { Download, Smartphone, HousePlus } from '@lucide/vue'

import InstallInstructions from '@/components/onboarding/InstallInstructions.vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { APP_NAME } from '@/config/brand'
import { installPlatform } from '@/helpers/platform'
import { WEBSITE_PAGE } from '@/router/gates'
import { useInstallStore } from '@/stores/install.store'

// The install wall: what a mobile visitor sees when the app is not running standalone.
//
// The app is not usable in a browser tab — on iOS, Web Push is only available to
// home-screen web apps at all, and a tab is far more likely to be closed or evicted
// mid-event — so this tab's only job is to get the app installed (PRD 005 §1).
//
// The view assumes it owns the whole viewport: the top bar and BottomNav are hidden for
// this route in App.vue. `fullBleed` is deliberately not used, because it only suppresses
// the header *inside* `showShell` and would be a no-op here (PRD 005 §7).

const install = useInstallStore()

// The platform is read once: it cannot change during the life of this view, and
// `installPlatform()` is a pure function over `navigator` (task 116).
const platform = installPlatform()

const prompting = ref(false)

// Whether to offer the one-tap button at all. `canPrompt` goes false once the native
// prompt has been shown, whichever way the user answered — the event is single-use — so
// this flips to the manual instructions afterwards rather than leaving a dead button. A
// user who dismissed the native dialog still needs a route forward.
const canOneTap = computed(() => install.canPrompt && !prompting.value)

async function promptInstall() {
  prompting.value = true
  try {
    await install.promptInstall()
  } finally {
    prompting.value = false
  }
}

</script>

<template>
  <!--
    THIS VIEW OWNS ITS OWN SCROLLING, and it has to.

    `main.css` sets `overflow: hidden` on html and body on purpose (so the whole app cannot be
    dragged), which means scrolling in this app belongs to the shell's `<main>`. This route renders
    *outside* the shell — no top bar, no bottom nav — so nothing above it provides a scroll
    container, and without one the content simply cannot be reached.

    That is not hypothetical: on a real iPhone in Safari the viewport is ~695px (Safari's own chrome
    takes ~157px of the 852px screen), the instructions are taller than that, and step 4 was clipped
    mid-sentence — on the one screen whose entire purpose is those instructions, on the platform
    where they are the only way in. `Card` has `overflow-hidden`, so a shrunken flex child clipped
    silently rather than spilling visibly (task 149).

    `min-h-full` on the inner column rather than `h-full` is what makes it behave in both
    directions: centred when it fits, top-aligned and scrollable when it does not. The safe-area
    insets go on the inner element, because padding on a scroll container is not part of its
    scrollable area on every engine.
  -->
  <main class="h-full overflow-y-auto [overscroll-behavior:contain]">
    <div
      class="mx-auto flex min-h-full w-full max-w-sm flex-col justify-center gap-6 px-6"
      style="padding-top: calc(var(--sat) + 2rem); padding-bottom: calc(var(--sab) + 2rem)"
    >
    <header class="flex flex-col items-center gap-3 text-center">
      <div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-slate-900 text-white">
        <Smartphone class="h-7 w-7" aria-hidden="true" />
      </div>
      <h1 class="font-nathejk text-3xl tracking-wide">{{ APP_NAME }}</h1>
      <p class="text-sm leading-relaxed text-slate-500">
        {{ APP_NAME }} skal ligge på din hjemmeskærm, før du kan logge ind. Det er den
        eneste måde, du kan få beskeder fra løbet — og appen virker også, når der ikke er
        signal i skoven.
      </p>
    </header>

    <!--
      Chromium's one-tap install. The only platform where this exists: iOS fires no
      beforeinstallprompt at all (see InstallInstructions).
    -->
    <div v-if="canOneTap" class="flex flex-col gap-3">
      <Button size="lg" class="w-full" :disabled="prompting" @click="promptInstall">
        <Download aria-hidden="true" />
        Installér app
      </Button>
      <p class="text-center text-xs leading-relaxed text-slate-500">
        Din browser spørger, om appen må lægges på hjemmeskærmen. Sig ja, luk denne fane, og
        åbn {{ APP_NAME }} derfra.
      </p>
    </div>

    <!-- No prompt available: iOS Safari, Android non-Chrome, or an in-app webview. -->
    <Card v-else>
      <CardContent>
        <InstallInstructions :platform="platform" />
      </CardContent>
    </Card>

    <!--
      "Already installed", and it is mandatory rather than a nicety.

      Installation CANNOT be reliably detected from a browser tab. `getInstalledRelatedApps()`
      is Chromium-only and, despite the name, reports related *native* apps — it is not a
      "is my PWA installed" query — and on iOS there is no install-accepted event at all.
      So a user may have installed the app a minute ago and this tab has no way to know.
      Without this, the wall's failure mode is someone who did exactly what was asked
      staring at a screen telling them to do it again.

      It is text and not a button on purpose: a tab cannot launch its own standalone
      instance, so a button here could only pretend to work. Saying plainly what to do is
      more use than an affordance that does nothing.
    -->
    <div class="flex flex-col items-center gap-1.5 border-t border-slate-200 pt-5 text-center">
      <p class="inline-flex items-center gap-1.5 text-sm font-medium text-slate-700">
        <HousePlus class="h-4 w-4 shrink-0 text-slate-400" aria-hidden="true" />
        Har du allerede installeret appen?
      </p>
      <p class="text-xs leading-relaxed text-slate-500">
        Luk denne fane og åbn <strong>{{ APP_NAME }}</strong> fra hjemmeskærmen. Så er du
        det rigtige sted, og du bliver ikke spurgt igen.
      </p>
    </div>

    <!--
      Out to the public website.

      This replaces the former "Fortsæt i browseren" escape hatch, which let a browser tab into
      the login flow. There is no login outside the installed app any more (task 143), so that
      destination no longer exists — and a link promising to "continue" would be promising the
      app, which is precisely what a browser tab cannot give.

      A plain <a>, not a router link: the website is a separate document outside this app.

      Consequence worth knowing, since it removes what PRD 005 §6 called the no-lockout
      guarantee: a phone that genuinely cannot install a PWA now has no route to a login at all.
      That is accepted (task 143) — such a device cannot run push, background sync or a reliable
      service worker either, so the app's core features are already unavailable to it, and the
      in-app-webview case is handled by telling the user to open the link in Safari or Chrome.
    -->
    <div class="flex flex-col items-center gap-1">
      <a :href="WEBSITE_PAGE" class="px-2 py-2 text-sm text-slate-500 underline underline-offset-2">
        Gå til hjemmesiden
      </a>
      <p class="max-w-xs text-center text-xs leading-relaxed text-slate-400">
        På hjemmesiden kan du læse om Nathejk. Selve appen — kort, kontakter og beskeder — virker
        kun, når den ligger på hjemmeskærmen.
      </p>
      </div>
    </div>
  </main>
</template>
