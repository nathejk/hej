<script setup lang="ts">
import { Share, PlusSquare, MoreHorizontal, Globe, TriangleAlert } from '@lucide/vue'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import type { InstallPlatform } from '@/helpers/platform'
import { APP_NAME } from '@/config/brand'

// The fallback half of the install wall (task 119): what to do when there is no
// `beforeinstallprompt` to offer. Three genuinely different situations, not three wordings
// of one.
//
// **iOS fires neither `beforeinstallprompt` nor any install-accepted event.** So on
// `ios-safari` there is no one-tap button to show and — just as importantly — no event to
// switch this screen into a success state when the user succeeds. The copy is therefore
// written to read as complete on its own rather than as a screen waiting for something to
// happen; the confirmation is the user opening the app from their home screen, which the
// wall's "jeg har allerede installeret appen" affordance covers.
//
// Accessibility is a requirement here, not a nicety (PRD 005 §6): every step is real text
// in an ordered list, the icons are `aria-hidden` illustration *alongside* the words and
// never the sole carrier of a step, and nothing needs a swipe, long-press or drag.
// Screenshots are deliberately absent: they are invisible to a screen reader, they go
// stale with every OS release, and they are the usual way add-to-home-screen instructions
// rot.
defineProps<{ platform: InstallPlatform }>()
</script>

<template>
  <!--
    In-app browser (Facebook, Instagram, …). Installing is *impossible* here: an embedded
    webview has no add-to-home-screen item and receives no install prompt, so instructions
    of the usual shape would be actively misleading. The only correct advice is to leave
    the webview.

    First-class rather than an afterthought, because event links get shared in Facebook
    groups — in practice this is one of the most likely variants to be hit.
  -->
  <Alert v-if="platform === 'webview'" class="border-amber-300 bg-amber-50 text-amber-900">
    <TriangleAlert aria-hidden="true" />
    <AlertTitle>Åbn siden i din browser</AlertTitle>
    <AlertDescription class="text-amber-800">
      <p>
        Du ser siden inde i en anden app (fx Facebook eller Instagram). Her kan
        {{ APP_NAME }} ikke installeres — det kan kun gøres i en rigtig browser.
      </p>
      <ol class="mt-3 flex list-decimal flex-col gap-2 pl-5">
        <li>
          <span class="inline-flex flex-wrap items-center gap-1.5">
            <MoreHorizontal class="h-4 w-4 shrink-0" aria-hidden="true" />
            Tryk på menuen med de tre punkter
          </span>
        </li>
        <li>Vælg <strong>Åbn i browser</strong> (Safari på iPhone, Chrome på Android)</li>
        <li>Følg vejledningen der for at lægge appen på hjemmeskærmen</li>
      </ol>
    </AlertDescription>
  </Alert>

  <!--
    iOS/iPadOS. Every browser on iOS is WebKit, but only Safari can add to the home
    screen, so this variant also has to be able to tell a Chrome-on-iOS user to switch.

    The Share control is named by what the user sees and located explicitly: on iPhone it
    sits in the *bottom* toolbar, which is not where people look for it.
  -->
  <div v-else-if="platform === 'ios-safari'" class="flex flex-col gap-3">
    <h2 class="font-nathejk text-xl tracking-wide">Læg appen på hjemmeskærmen</h2>
    <ol class="flex list-decimal flex-col gap-3 pl-5 text-sm leading-relaxed text-slate-600">
      <li>
        <span class="inline-flex flex-wrap items-center gap-1.5">
          <Share class="h-4 w-4 shrink-0 text-slate-500" aria-hidden="true" />
          Tryk på <strong>Del</strong> — firkanten med pilen op, nederst i Safari
        </span>
      </li>
      <li>
        <span class="inline-flex flex-wrap items-center gap-1.5">
          <PlusSquare class="h-4 w-4 shrink-0 text-slate-500" aria-hidden="true" />
          Rul ned og vælg <strong>Tilføj til hjemmeskærm</strong>
        </span>
      </li>
      <li>Tryk <strong>Tilføj</strong> øverst til højre</li>
      <li>Luk Safari og åbn <strong>{{ APP_NAME }}</strong> fra hjemmeskærmen</li>
    </ol>
    <p class="text-xs leading-relaxed text-slate-500">
      Bruger du Chrome eller Firefox på din iPhone, findes <strong>Tilføj til
      hjemmeskærm</strong> ikke. Åbn siden i <strong>Safari</strong> i stedet.
    </p>
  </div>

  <!--
    Android, non-Chrome (Firefox, older Samsung Internet, …). The menu item exists in most
    of these but its label and position vary by browser and version, so we deliberately do
    not claim an exact path we cannot guarantee — a wrong menu path is worse than a
    described one, because the user concludes the app is broken rather than that the menu
    moved. Opening in Chrome is offered as the route we *can* stand behind.
  -->
  <div v-else class="flex flex-col gap-3">
    <h2 class="font-nathejk text-xl tracking-wide">Læg appen på hjemmeskærmen</h2>
    <ol class="flex list-decimal flex-col gap-3 pl-5 text-sm leading-relaxed text-slate-600">
      <li>
        <span class="inline-flex flex-wrap items-center gap-1.5">
          <MoreHorizontal class="h-4 w-4 shrink-0 text-slate-500" aria-hidden="true" />
          Åbn browserens menu
        </span>
      </li>
      <li>
        Vælg punktet, der hedder noget i retning af <strong>Installér app</strong> eller
        <strong>Tilføj til hjemmeskærm</strong> — det står forskellige steder i forskellige
        browsere
      </li>
      <li>Åbn <strong>{{ APP_NAME }}</strong> fra hjemmeskærmen</li>
    </ol>
    <p class="inline-flex flex-wrap items-center gap-1.5 text-xs leading-relaxed text-slate-500">
      <Globe class="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
      Kan du ikke finde punktet, så åbn siden i <strong>Chrome</strong>, hvor appen kan
      installeres med et enkelt tryk.
    </p>
  </div>
</template>
