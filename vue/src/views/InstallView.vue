<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Download, Smartphone, HousePlus } from '@lucide/vue'

import InstallInstructions from '@/components/onboarding/InstallInstructions.vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { APP_NAME } from '@/config/brand'
import { installPlatform } from '@/helpers/platform'
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
const router = useRouter()

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

// The escape hatch (task 121). Load-bearing, not a courtesy for power users: PRD 005 §11
// chose an aggressive detection tie-break — ambiguous devices classify as mobile —
// *because* this exists, and PRD 005 §6 states the rule outright: no lockout, every gate
// has a user-reachable escape hatch. During an event this is a safety app, and a false
// negative in device or install detection must never be the reason somebody cannot reach
// it.
//
// It unblocks the install gate only; the user lands in the normal login flow, not past it.
//
// Deliberately NOT implemented here: rate-limiting or an expiry. PRD 005 §12 leaves that
// open, and the store records *when* the override was set so a policy can be added later
// without a migration.
async function continueInBrowser() {
  install.setContinueInBrowser(true)
  await router.replace({ name: 'login' })
}
</script>

<template>
  <main
    class="mx-auto flex h-full w-full max-w-sm flex-col justify-center gap-6 px-6"
    style="padding-top: var(--sat); padding-bottom: var(--sab)"
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
      The escape hatch. Two hard edges on its prominence, and it has to sit between them:

      - Not so hidden that support cannot talk a user through it over the phone in a noisy
        field. Hence plain visible text at the bottom, no gesture and no repeated taps.
      - Not so prominent that it reads as an equal alternative to installing, or it becomes
        the default path and the wall stops working (PRD 005 §9 targets < 2% of sessions).

      The trade-off is stated rather than hidden, so the choice is made knowingly instead of
      discovered later when a notification never arrives.
    -->
    <div class="flex flex-col items-center gap-1">
      <Button variant="link" size="sm" class="text-slate-500" @click="continueInBrowser">
        Fortsæt i browseren
      </Button>
      <p class="max-w-xs text-center text-xs leading-relaxed text-slate-400">
        Så virker appen dårligere: du får ingen beskeder fra løbet på iPhone, og den virker
        ikke uden signal. Du kan altid installere den bagefter under <strong>Min
        profil</strong>.
      </p>
    </div>
  </main>
</template>
