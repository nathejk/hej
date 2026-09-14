<script setup lang="ts">
import { computed, ref } from 'vue'
import { ShieldCheck } from '@lucide/vue'

import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
  InputGroupText,
} from '@/components/ui/input-group'
import { Label } from '@/components/ui/label'
import { HttpError } from '@/helpers'
import { formatPhone } from '@/helpers'
import { contactCheckFailure, useProfileStore } from '@/stores/profile.store'

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
//
// The step **opens in `correct`** when there is no number to recall — an unregistered contact
// number, or one the BFF blanked because it was really the member's own (PRD 015, task 229). There
// are no digits to remember in that state, and rendering the recall UI would show an empty number
// followed by two crosses, which reads as a bug.
const mode = ref<'confirm' | 'correct'>(profile.details?.phoneParent ? 'confirm' : 'correct')

const digits = ref('')
const replacement = ref('')
const acknowledged = ref(false)
const busy = ref(false)
const error = ref('')

// How many recall attempts are left, as the *server* last reported them (PRD 015, task 227). Null
// until the first miss.
//
// Not counted here. A client-side counter is reset by a reload, so the limit would not exist — and
// two counters would disagree about which attempt this is, with the client's being the one a member
// can clear.
const attemptsLeft = ref<number | null>(null)

const details = computed(() => profile.details)

// "11 22 33 **" — the formatted number with its last two digits replaced. Rendered as text;
// there is deliberately no input holding the number itself.
const maskedParent = computed(() => {
  const raw = details.value?.phoneParent
  if (!raw) return ''
  const formatted = formatPhone(raw)
  return formatted.replace(/\d{2}$/, '**')
})

// The same number with the last two digits *removed* rather than masked, so the input can sit
// where they belong and the member reads one continuous number instead of a number and a puzzle
// somewhere else on the page.
const parentPrefix = computed(() => {
  const raw = details.value?.phoneParent
  if (!raw) return ''
  return formatPhone(raw).replace(/\d{2}$/, '').trimEnd()
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
    // A wrong pair of digits, with the server's own count of what is left (PRD 015, task 227).
    const failed = mode.value === 'confirm' ? contactCheckFailure(err) : null
    if (failed) {
      attemptsLeft.value = failed.attemptsRemaining
      if (failed.checkClosed) {
        // Three misses: the step is over and the server has already recorded the outcome, so this
        // does not call skip() — that would publish a second event for one give-up.
        //
        // Emitted as a skip rather than shown as an error, because that is what happened: nobody
        // did anything wrong, and the member goes into the app. WelcomeView's copy for the skip
        // path already says we will ask again next time.
        emit('skip')
        return
      }
      error.value = failed.message
      digits.value = ''
      return
    }

    if (err instanceof HttpError && err.status === 400) {
      // Not a scolding in either mode. Wrong digits most likely means the number on file is not one
      // this member knows — which is what the correcting mode is for, and it is one tap away.
      error.value =
        mode.value === 'correct'
          ? 'Det ser ikke ud som et telefonnummer. Tjek det, og prøv igen.'
          : 'De to cifre passer ikke til nummeret, vi har.'
    } else if (err instanceof HttpError && err.status === 409) {
      // Nothing left to do: already confirmed on another device, a double submit, the check closed
      // earlier in this session, or — in `correct` mode — the member has started and the number is
      // settled at check-in (PRD 015, task 231). None of those is a failure the member can act on,
      // so they go through.
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

// Giving up, from the "spring over" button. Records the outcome so the check-in counter can see
// that this member was asked and could not answer (PRD 015, task 228) — then lets them through
// regardless of whether that recording succeeded.
async function giveUp() {
  busy.value = true
  try {
    await profile.skipContactCheck()
  } finally {
    busy.value = false
    emit('skip')
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
      </CardContent>
    </Card>

    <!--
      The guardian number sits *outside* the card of registered facts and slightly lifted, because
      it is not one more row to skim: it is the thing this step is about. Next to "Navn" and "Din
      telefon" it reads as data; on its own, highlighted, it reads as a question.

      Kept visible in correcting mode too, labelled as what we hold. The member is replacing it,
      so seeing it is useful — "ah, that's my dad's old number" is exactly the recognition this
      step is trying to provoke, and it may still jog the right answer.
    -->
    <div
      v-if="maskedParent"
      class="flex flex-col gap-2 rounded-xl border border-slate-200 bg-slate-50 px-4 py-3"
    >
      <span class="text-xs font-medium tracking-wide text-slate-500 uppercase">
        Forælder/værge
      </span>
      <!--
        The instruction sits directly above the number it applies to, not below the input: a member
        who has already started typing does not need it, and one who has not is looking at the
        number. Same reason the digits are typed in place — read the line, then complete it.
      -->
      <p v-if="mode === 'confirm'" class="text-xs leading-relaxed text-slate-500">
        Udfyld de sidste to cifre. Kender du ikke nummeret udenad, så spørg din forælder.
      </p>
      <!--
        Confirm mode: the two missing digits are typed *in place*, at the end of the number, so
        the member completes a number rather than answering a quiz about one. That is the whole
        recognition device — see the masking note above.
      -->
      <div v-if="mode === 'confirm'" class="flex items-center justify-center gap-3">
        <span class="text-xl font-medium tracking-wide text-slate-900 tabular-nums">
          {{ parentPrefix }}
        </span>
        <Label for="parent-digits" class="sr-only">Skriv de sidste to cifre i nummeret</Label>
        <Input
          id="parent-digits"
          v-model="digits"
          inputmode="numeric"
          autocomplete="off"
          maxlength="2"
          class="h-11 w-20 bg-white text-center text-xl tracking-[0.3em] tabular-nums"
          placeholder="••"
        />
      </div>
      <span
        v-else-if="maskedParent"
        class="text-center text-xl font-medium tracking-wide text-slate-900 tabular-nums"
      >
        {{ maskedParent }}
      </span>
      <!--
        Nothing to show: the register holds no contact number, or the BFF blanked one that was
        really the member's own (PRD 015, task 229). The step opened straight into correction mode,
        so this says what is missing rather than rendering an empty number with two crosses after
        it. It deliberately does not hint at what the number should be — and never suggests the
        member's own.
      -->
      <span v-else class="text-center text-sm text-slate-500">
        Vi har ikke noget nummer på en voksen for dig.
      </span>

      <!--
        The way out of a failed recognition, kept next to the number it is about (task 148): the
        member who does not recognise this line should not have to scroll past a form to say so.

        Not worded as a failure, because it is not one — a member not knowing the number is
        expected: young scouts, a guardian who changed number, two households with different
        numbers on file. So it names the action, not the problem.
      -->
      <button
        v-if="mode === 'confirm'"
        type="button"
        class="self-center text-sm text-slate-500 underline underline-offset-2"
        @click="startCorrecting"
      >
        Skriv andet nummer
      </button>
    </div>

    <form class="flex flex-col gap-4" @submit.prevent="submit">
      <!--
        The field opened up (task 148). Empty, not prefilled: see startCorrecting().

        `type="tel"` and `inputmode="tel"` so a phone shows the number pad, and no `autocomplete`
        — the browser's saved value here would be the *member's own* number, which is the one
        number this field must not end up holding.
      -->
      <div v-if="mode === 'correct'" class="flex flex-col gap-2">
        <Label for="parent-phone">Skriv nummeret på din forælder eller værge</Label>
        <!--
          The +45 is a fixed addon, not something to type: every number this app calls is Danish,
          and a member typing a prefix into the field is how you get "004512345678" and
          "+45 12 34 56 78" in the same column. The placeholder shows the shape we want — eight
          digits, nothing else.
        -->
        <InputGroup>
          <InputGroupInput
            id="parent-phone"
            v-model="replacement"
            type="tel"
            inputmode="tel"
            autocomplete="off"
            placeholder="××××××××"
          />
          <InputGroupAddon>
            <InputGroupText>+45</InputGroupText>
          </InputGroupAddon>
        </InputGroup>
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

      <!--
        The error, and — while the member still has tries left — how many. The count comes from the
        server's response, never from a tally kept here: a client-side counter is cleared by a
        reload, so the limit it enforces would not exist.

        `role="alert"` on the wrapper so both lines are announced together; a screen reader user
        hearing "det passer ikke" without "du kan prøve to gange mere" gets the worse half.
      -->
      <div v-if="error" class="flex flex-col gap-1" role="alert">
        <p class="text-sm text-red-600">{{ error }}</p>
        <p v-if="attemptsLeft !== null && attemptsLeft > 0" class="text-xs leading-relaxed text-slate-500">
          {{
            attemptsLeft === 1
              ? 'Du kan prøve én gang mere — eller skrive et andet nummer.'
              : 'Du kan prøve to gange mere — eller skrive et andet nummer.'
          }}
        </p>
      </div>

      <button
        type="submit"
        :disabled="!canSubmit"
        class="rounded-lg bg-slate-900 px-4 py-3 font-medium text-white transition disabled:opacity-50"
      >
        {{ mode === 'correct' ? 'Gem og bekræft' : 'Bekræft' }}
      </button>

      <!--
        Back out of correcting. The way *into* it now lives in the number box above (task 148),
        next to the number the member failed to recognise. Hidden when there is nothing to go back
        to: with no number on file the recall mode has no digits to offer.
      -->
      <Button
        v-if="mode === 'correct' && details?.phoneParent"
        type="button"
        variant="outline"
        class="text-slate-600"
        @click="mode = 'confirm'"
      >
        Tilbage — jeg prøver de to cifre igen
      </Button>

      <!--
        Last resort: they know no number at all. Lets them into the app — only login is mandatory
        (PRD 005 §6) — and now *records* that they were asked and could not answer, so the check-in
        counter can see it (PRD 015, task 228). `confirmation_required` stays true server-side, so
        they are asked again next time rather than quietly written off.

        The recording is best-effort: `skipContactCheck` never throws, because no signal in a forest
        must not turn giving up into a dead end.
      -->
      <Button type="button" variant="outline" class="text-slate-500" :disabled="busy" @click="giveUp">
        Jeg kender ikke nummeret — spring over
      </Button>
    </form>
  </div>
</template>
