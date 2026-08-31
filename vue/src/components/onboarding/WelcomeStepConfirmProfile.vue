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

const emit = defineEmits<{ done: []; skip: [] }>()

// Two modes. `confirm` is the default: the masked number with two digits to supply. `correct` is
// what a member who cannot recognise it switches to — the field opens up and they type the whole
// number instead (task 148).
//
// One component rather than two, because the acknowledgement, the copy explaining *why* the number
// matters, and the error handling are the same in both; only the input differs. Splitting them
// would duplicate the part that took the most care to word.
const mode = ref<'confirm' | 'correct'>('confirm')

const digits = ref('')
const replacement = ref('')
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

// Both are required to advance (PRD 005 §6): the input proves the member engaged with the number,
// the checkbox is the acknowledgement itself. Either alone would be weaker than it looks — an input
// without the tick is a memory test nobody agreed to anything with, and a tick without the input is
// a checkbox people click through.
//
// In `correct` mode "engaged" means a plausible Danish number: 8 digits, optionally +45-prefixed.
// Validated properly server-side; this only decides when the button lights up, and being loose here
// is deliberate — a member fighting a form that rejects their own parent's number is worse than one
// who gets a clear message back.
const canSubmit = computed(() => {
  if (busy.value || !acknowledged.value) return false
  if (mode.value === 'confirm') return /^\d{2}$/.test(digits.value)
  return /^(\+?45)?\s*(\d\s*){8}$/.test(replacement.value.trim())
})

// Switching to the correcting mode. The field starts **empty**, deliberately not prefilled with the
// registered number: prefilling invites editing one digit of a number the member has just said they
// do not recognise, and would make "corrected" indistinguishable from "retyped what we already
// had". An empty field asks the real question — what number should we call?
function startCorrecting() {
  mode.value = 'correct'
  acknowledged.value = false
  error.value = ''
}

async function submit() {
  if (!canSubmit.value) return
  busy.value = true
  error.value = ''
  try {
    if (mode.value === 'correct') {
      await profile.setGuardian(replacement.value)
    } else {
      await profile.confirm(digits.value)
    }
    emit('done')
  } catch (err) {
    if (err instanceof HttpError && err.status === 400) {
      // Not a scolding in either mode. Wrong digits most likely means the number on file is not one
      // this member knows — which is what the correcting mode is for, and it is one tap away.
      error.value =
        mode.value === 'correct'
          ? 'Det ser ikke ud som et telefonnummer. Tjek det, og prøv igen.'
          : 'De to cifre passer ikke til nummeret, vi har.'
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
        <!--
          Kept visible in correcting mode too, labelled as what we hold. The member is replacing it,
          so seeing it is useful — "ah, that's my dad's old number" is exactly the recognition this
          step is trying to provoke, and it may still jog the right answer.
        -->
        <div v-if="maskedParent" class="flex justify-between gap-4">
          <span class="text-slate-500">Forælder/værge</span>
          <span class="text-right font-medium tracking-wide text-slate-800">{{ maskedParent }}</span>
        </div>
      </CardContent>
    </Card>

    <form class="flex flex-col gap-4" @submit.prevent="submit">
      <!-- Default: prove you recognise the number we hold. -->
      <div v-if="mode === 'confirm'" class="flex flex-col gap-2">
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

      <!--
        The field opened up (task 148). Empty, not prefilled: see startCorrecting().

        `type="tel"` and `inputmode="tel"` so a phone shows the number pad, and no `autocomplete`
        — the browser's saved value here would be the *member's own* number, which is the one
        number this field must not end up holding.
      -->
      <div v-else class="flex flex-col gap-2">
        <Label for="parent-phone">Skriv nummeret på din forælder eller værge</Label>
        <Input
          id="parent-phone"
          v-model="replacement"
          type="tel"
          inputmode="tel"
          autocomplete="off"
          class="text-lg"
          placeholder="+45 …"
        />
        <p class="text-xs leading-relaxed text-slate-500">
          Det skal være et nummer, vi kan ringe til under løbet — ikke dit eget. Spørg gerne
          din forælder, hvis du er i tvivl.
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
        {{ mode === 'correct' ? 'Gem og bekræft' : 'Bekræft' }}
      </button>

      <!--
        Cannot confirm? Fix it. This is the primary way out of a failed recognition, and it is a
        better outcome than any flag: the person standing here is the one most likely to know their
        own guardian's number (task 148).

        Not worded as a failure, because it is not one — a member not knowing the number is
        expected: young scouts, a guardian who changed number, two households with different
        numbers on file.
      -->
      <button
        v-if="mode === 'confirm'"
        type="button"
        class="px-2 py-1 text-sm text-slate-500"
        @click="startCorrecting"
      >
        Nummeret er forkert, eller jeg kender det ikke
      </button>
      <button
        v-else
        type="button"
        class="px-2 py-1 text-sm text-slate-500"
        @click="mode = 'confirm'"
      >
        Tilbage — jeg prøver de to cifre igen
      </button>

      <!--
        Last resort: they know no number at all. Skips the step, records nothing, and lets them into
        the app — only login is mandatory (PRD 005 §6). `confirmation_required` stays true
        server-side, so they are asked again next time rather than quietly written off.
      -->
      <button type="button" class="px-2 py-1 text-xs text-slate-400" @click="emit('skip')">
        Jeg kender ikke nummeret — spring over
      </button>
      <p class="text-center text-xs leading-relaxed text-slate-400">
        Springer du over, spørger vi dig igen næste gang. Du kan også sige det til din leder.
      </p>
    </form>
  </div>
</template>
