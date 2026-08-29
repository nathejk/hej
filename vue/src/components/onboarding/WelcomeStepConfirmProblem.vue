<script setup lang="ts">
import { ref } from 'vue'
import { ArrowLeft, PhoneOff } from '@lucide/vue'

import { useProfileStore } from '@/stores/profile.store'

// The non-punitive ways out of the confirmation step (PRD 005 §5, task 128).
//
// A member not knowing their guardian's number is **expected and not rare**: young scouts, a
// guardian who recently changed number, or two households with different numbers on file. It
// is not a failure state and must not be worded as one — so there is no "fejl", no red, and
// no wording that suggests the member did something wrong.
//
// Two reasons, kept distinct because they are different signals to an organizer:
//
// - `wrong`   — "nummeret er forkert": a record to fix.
// - `unknown` — "jeg kender ikke nummeret": a record to check.
//
// Collapsing them would lose exactly the distinction that makes the flag useful. And the
// signal has value **even when the number turns out to be correct**: "this member could not
// confirm their guardian number" tells whoever is holding the phone at 02:00 that this is not
// a number to rely on without calling it first, whatever the register says.
//
// The member continues into the app on every path. Blocking a participant out of a safety app
// over a stale guardian number is the worse failure (PRD 005 §5); only login is mandatory.

const profile = useProfileStore()
const emit = defineEmits<{ done: []; back: [] }>()

const busy = ref(false)
// Whether the report actually reached the BFF. Tracked so the confirmation text can be
// honest: telling a member "Nathejk er informeret" when the request failed offline would be
// a lie that costs an organizer a phone call at 02:00.
const sent = ref<boolean | null>(null)

async function report(reason: 'wrong' | 'unknown') {
  if (busy.value) return
  busy.value = true
  try {
    await profile.reportIncorrect(reason)
    sent.value = true
  } catch {
    // Continue regardless — see above. The flow must not depend on this request.
    sent.value = false
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <header class="flex flex-col items-center gap-4 text-center">
      <div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-slate-100 text-slate-600">
        <PhoneOff class="h-7 w-7" aria-hidden="true" />
      </div>
      <h1 class="font-nathejk text-3xl tracking-wide">Passer nummeret ikke?</h1>
      <p class="text-sm leading-relaxed text-slate-600">
        Det er helt normalt — måske er nummeret skiftet, eller måske kender du det ikke
        udenad. Sig hvad der passer, så kigger Nathejk på det.
      </p>
    </header>

    <div v-if="sent === null" class="flex flex-col gap-3">
      <button
        type="button"
        :disabled="busy"
        class="rounded-lg border border-slate-300 px-4 py-3 text-left transition disabled:opacity-50"
        @click="report('wrong')"
      >
        <span class="block font-medium text-slate-800">Nummeret er forkert</span>
        <span class="block text-sm text-slate-500">
          Jeg kan se, at nummeret ikke er det rigtige
        </span>
      </button>

      <button
        type="button"
        :disabled="busy"
        class="rounded-lg border border-slate-300 px-4 py-3 text-left transition disabled:opacity-50"
        @click="report('unknown')"
      >
        <span class="block font-medium text-slate-800">Jeg kender ikke nummeret</span>
        <span class="block text-sm text-slate-500">
          Jeg kan ikke huske de sidste cifre lige nu
        </span>
      </button>

      <button
        type="button"
        class="mt-1 flex items-center gap-1 self-start px-2 py-1 text-sm text-slate-500"
        @click="emit('back')"
      >
        <ArrowLeft class="h-4 w-4" aria-hidden="true" />
        Tilbage, jeg prøver igen
      </button>
    </div>

    <!--
      A report needs a visible consequence. "Tak" with no stated outcome reads as a dead end,
      and that is why people stop reporting things.

      NOTE: the exact correction channel is an open question (PRD 005 §12, and the same
      question in PRD 003) — phone, email, the patrol leader, or purely this in-app flag. No
      contact is invented here; the copy states the honest minimum, which is that Nathejk has
      been told and will check it. Fill in the channel when the question is answered.
    -->
    <div v-else class="flex flex-col gap-4 text-center">
      <p v-if="sent" class="text-sm leading-relaxed text-slate-600">
        Tak. Nathejk har fået besked og tjekker nummeret inden løbet. Er der noget, der skal
        rettes, hører du fra din leder.
      </p>
      <p v-else class="text-sm leading-relaxed text-slate-600">
        Vi kunne ikke sende beskeden lige nu — der er nok ikke signal. Sig det til din leder
        eller til Nathejk i startområdet, så bliver det rettet.
      </p>
      <button
        type="button"
        class="rounded-lg bg-slate-900 px-4 py-3 font-medium text-white"
        @click="emit('done')"
      >
        Videre
      </button>
    </div>
  </div>
</template>
