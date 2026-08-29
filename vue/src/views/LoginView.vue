<script setup lang="ts">
// TEMPORARY SHIM — deleted by task 126, together with the `/login` route.
//
// The login logic now lives in `components/onboarding/WelcomeStepLogin.vue`, because the
// credential step belongs inside `/welcome` (PRD 005 §7). This file remains only so the
// still-registered `/login` route keeps pointing at a real module: task 125 (the move) and
// task 126 (removing the route and repointing the router guard and `App.vue`'s `signOut()`)
// are separate commits, and deleting the view before the route would leave a route
// resolving to a missing chunk.
//
// Do not build anything on this file.
import { useRouter } from 'vue-router'
import { KeyRound } from '@lucide/vue'

import WelcomeStepLogin from '@/components/onboarding/WelcomeStepLogin.vue'
import { APP_NAME } from '@/config/brand'

const router = useRouter()
const version = __APP_VERSION__

// Same destination as before the move: routed by path so this does not need to know the
// first destination's name.
async function done() {
  await router.replace({ path: '/' })
}
</script>

<template>
  <main
    class="mx-auto flex h-full w-full max-w-sm flex-col justify-center gap-8 px-6"
    style="padding-top: var(--sat); padding-bottom: var(--sab)"
  >
    <header class="flex flex-col items-center gap-3 text-center">
      <div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-slate-900 text-white">
        <KeyRound class="h-7 w-7" />
      </div>
      <h1 class="font-nathejk text-3xl tracking-wide">{{ APP_NAME }}</h1>
    </header>

    <WelcomeStepLogin @done="done" />

    <p class="text-center text-xs text-slate-300">v{{ version }}</p>
  </main>
</template>
