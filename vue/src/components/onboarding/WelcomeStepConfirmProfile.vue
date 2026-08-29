<script setup lang="ts">
import { computed, ref } from 'vue'
import { ShieldCheck } from '@lucide/vue'

import { Card, CardContent } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { HttpError } from '@/helpers'
import { formatPhone } from '@/helpers'
import { useProfileStore } from '@/stores/profile.store'

// First-login step: the member looks at what Nathejk has on file and acknowledges that the
// parent/guardian emergency number can actually be reached (PRD 005 §5 step 2, §6, §7).
//
// **Spejder only.** `phoneParent` exists on spejder and on no other population — null means
// *not applicable*, not missing. A bandit or crew member has no guardian number by design, so
// this step never renders an empty guardian field as though data were absent: doing so would
// generate support calls and, worse, teach organizers to ignore the flag that matters.
//
// Whether confirmation is *required* is not decided here. The BFF derives
// `confirmation_required` from "has verified" OR "has started the event", and the client must
// not reimplement that rule (PRD 005 §8) — this component only renders when the store says so.
//
// **The masking is a recognition device, not a confidentiality control** (PRD 005 §11,
// 2026-08-30). `GET /api/me/profile` returns `phone_parent` in full to its owner, exactly as
// PRD 003 shipped it, so the two hidden digits are in the network response and this step is
// not tamper-proof. It is not meant to be: nobody is being authenticated, and the number is
// the member's own guardian's. What it does is make them *look*.
//
// Masking the LAST two digits rather than the first is what makes it a recall check instead of
// a copying exercise — a member who cannot complete it has just discovered that the number on
// file is not one they know (PRD 005 §11, 2026-08-25).

const profile = useProfileStore()

const emit = defineEmits<{ done: []; report: [] }>()

const digits = ref('')
const acknowledged = ref(false)
const busy = ref(false)
const error = ref('')

const details = computed(() => profile.details)

// "11 22 33 **" — the formatted number with its last two digits replaced. Rendered as text;
// there is deliberately no input holding the number itself.
const maskedParent = computed(() => {
  const raw = details.value?.phoneParent
  if (!raw) return ''
  const formatted = formatPhone(raw)
  return formatted.replace(/\d{2}$/, '**')
})

// Both are required to advance (PRD 005 §6): the digits prove the member recognised the
// number, the checkbox is the acknowledgement itself. Either alone would be weaker than it
// looks — digits without the tick is a memory test nobody agreed to anything with, and a tick
// without digits is a checkbox people click through.
const canSubmit = computed(() => /^\d{2}$/.test(digits.value) && acknowledged.value && !busy.value)

async function submit() {
  if (!canSubmit.value) return
  busy.value = true
  error.value = ''
  try {
    await profile.confirm(digits.value)
    emit('done')
  } catch (err) {
    if (err instanceof HttpError && err.status === 400) {
      // Not a scolding: the likely explanation is that the number on file is not the one
      // this member knows, which is exactly the situation the failure paths (task 128) are
      // for.
      error.value = 'De to cifre passer ikke til nummeret, vi har.'
    } else if (err instanceof HttpError && err.status === 409) {
      // Already confirmed — from another device, or a double submit. Nothing is wrong.
      emit('done')
    } else if (err instanceof HttpError && err.status === 429) {
      error.value = 'For mange forsøg. Prøv igen om lidt.'
    } else {
      error.value = 'Kunne ikke gemmes. Prøv igen.'
    }
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <header class="flex flex-col items-center gap-4 text-center">
      <div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-slate-900 text-white">
        <ShieldCheck class="h-7 w-7" aria-hidden="true" />
      </div>
      <h1 class="font-nathejk text-3xl tracking-wide">Tjek dine oplysninger</h1>
      <!--
        Both reasons, deliberately. "I nødstilfælde" alone understates how routinely the
        number gets used and invites a shrug: a 14-year-old who does not expect to get hurt
        has no reason to care about an emergency number, but does understand going home
        early (PRD 005 §11).
      -->
      <p class="text-sm leading-relaxed text-slate-600">
        Vi skal kunne få fat på en voksen, hvis der sker noget — og hvis du stopper undervejs
        og skal hentes. Derfor beder vi dig lige bekræfte nummeret.
      </p>
    </header>

    <!-- Read-only. Editing in the app is out of scope (PRD 005 §6). -->
    <Card>
      <CardContent class="flex flex-col gap-2 text-sm">
        <div v-if="details" class="flex justify-between gap-4">
          <span class="text-slate-500">Navn</span>
          <span class="text-right font-medium text-slate-800">{{ details.name }}</span>
        </div>
        <div v-if="details?.team || details?.section" class="flex justify-between gap-4">
          <span class="text-slate-500">Hold</span>
          <span class="text-right text-slate-800">{{ details?.team || details?.section }}</span>
        </div>
        <div v-if="details?.phone" class="flex justify-between gap-4">
          <span class="text-slate-500">Din telefon</span>
          <span class="text-right text-slate-800">{{ formatPhone(details.phone) }}</span>
        </div>
        <div v-if="maskedParent" class="flex justify-between gap-4">
          <span class="text-slate-500">Forælder/værge</span>
          <span class="text-right font-medium tracking-wide text-slate-800">{{ maskedParent }}</span>
        </div>
      </CardContent>
    </Card>

    <form class="flex flex-col gap-4" @submit.prevent="submit">
      <div class="flex flex-col gap-2">
        <Label for="parent-digits">Skriv de sidste to cifre i nummeret</Label>
        <Input
          id="parent-digits"
          v-model="digits"
          inputmode="numeric"
          autocomplete="off"
          maxlength="2"
          class="w-24 text-center text-2xl tracking-[0.4em]"
          placeholder="••"
        />
        <p class="text-xs leading-relaxed text-slate-500">
          Kender du ikke nummeret udenad, så spørg din forælder — det er hele pointen med
          tjekket.
        </p>
      </div>

      <div class="flex items-start gap-3">
        <Checkbox id="parent-ack" v-model="acknowledged" class="mt-0.5" />
        <Label for="parent-ack" class="items-start text-sm leading-relaxed font-normal">
          Dette nummer kan kontaktes i løbet af Nathejk
        </Label>
      </div>

      <p v-if="error" class="text-sm text-red-600" role="alert">{{ error }}</p>

      <button
        type="submit"
        :disabled="!canSubmit"
        class="rounded-lg bg-slate-900 px-4 py-3 font-medium text-white transition disabled:opacity-50"
      >
        Bekræft
      </button>

      <!-- The non-punitive ways out live in task 128, which owns this affordance's copy. -->
      <button type="button" class="px-2 py-1 text-sm text-slate-500" @click="emit('report')">
        Nummeret er forkert, eller jeg kender det ikke
      </button>
    </form>
  </div>
</template>
