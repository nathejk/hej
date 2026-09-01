<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue'
import { ArrowLeft, Phone, WifiOff } from '@lucide/vue'

import { HttpError, NetworkError } from '@/helpers'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import ProfileChooser from '@/components/auth/ProfileChooser.vue'
import { useSessionStore } from '@/stores/session.store'
import { useAppStore } from '@/stores/app.store'
import { NODTELEFON, NODTELEFON_DISPLAY } from '@/config/contact'

// The credential step of onboarding: phone → SMS PIN → session (PRD 005 §7).
//
// This is a *move* of the former LoginView, not a reimplementation. It is not a form but a
// small state machine — phone → pin → choose — with a resend cooldown, an offline branch and
// specific error mappings, plus the shared-number chooser that exists because roughly one
// number in eight belongs to more than one person (task 079) and guessing would sign someone
// in as a sibling. Writing a second login inside /welcome would have produced two subtly
// different ones that then drift apart.
//
// Reworking the login mechanism is a non-goal (PRD 005 §4): the PIN length, the cooldown, the
// anti-enumeration behaviour (the PIN step always follows the phone step whether or not the
// number is recognised) and the nødtelefon fallback are unchanged.
//
// **Login is the only mandatory step** (PRD 005 §6), so there is deliberately no skip and no
// "continue anyway" here. The escape hatches in this PRD live on the install wall.
//
// Layout, safe-area padding and the version string belong to the /welcome shell — this
// component renders the credential UI and nothing else, so two components are not both
// padding for --sat/--sab.
const RESEND_COOLDOWN_SECONDS = 60

const session = useSessionStore()
const app = useAppStore()

const emit = defineEmits<{ done: [] }>()

const step = ref<'phone' | 'pin' | 'choose'>('phone')
const phone = ref('')
const pin = ref('')
const error = ref('')
const busy = ref(false)

const cooldown = ref(0)
let cooldownTimer: ReturnType<typeof setInterval> | undefined

const canResend = computed(() => cooldown.value === 0 && !busy.value)

function startCooldown() {
  cooldown.value = RESEND_COOLDOWN_SECONDS
  clearInterval(cooldownTimer)
  cooldownTimer = setInterval(() => {
    cooldown.value -= 1
    if (cooldown.value <= 0) {
      clearInterval(cooldownTimer)
    }
  }, 1000)
}

onUnmounted(() => clearInterval(cooldownTimer))

async function sendPin() {
  error.value = ''
  busy.value = true
  try {
    await session.requestPin(phone.value)
    step.value = 'pin'
    startCooldown()
  } catch (err) {
    error.value = messageFor(err)
  } finally {
    busy.value = false
  }
}

async function resendPin() {
  if (!canResend.value) return
  pin.value = ''
  await sendPin()
}

async function submitPin() {
  error.value = ''
  busy.value = true
  try {
    const identity = await session.verify(phone.value, pin.value)
    if (identity === null) {
      // Verification succeeded, but the number belongs to several people. Ask which
      // one rather than guessing — a wrong guess would sign someone in as a sibling.
      step.value = 'choose'
      return
    }
    // Hand back to the flow instead of navigating. A first-time spejder's next screen is
    // profile confirmation, not the map — where the flow goes next is the onboarding
    // store's decision, and the final redirect is the shell's.
    emit('done')
  } catch (err) {
    if (err instanceof HttpError && err.status === 401) {
      error.value = 'Koden passer ikke. Prøv igen.'
    } else if (err instanceof HttpError && err.status === 429) {
      error.value = 'For mange forsøg. Bed om en ny kode og prøv igen.'
    } else {
      error.value = messageFor(err)
    }
  } finally {
    busy.value = false
  }
}

async function choose(userId: string) {
  error.value = ''
  busy.value = true
  try {
    await session.choose(userId)
    emit('done')
  } catch (err) {
    if (err instanceof HttpError && err.status === 401) {
      // The token lives about a minute, so this is most often simply too slow.
      error.value = 'Login udløb. Bed om en ny kode.'
      step.value = 'phone'
      session.clearChoice()
    } else {
      error.value = messageFor(err)
    }
  } finally {
    busy.value = false
  }
}

// Signing in needs the network — the only secret is an SMS PIN — so unlike the rest
// of the app this screen genuinely cannot work offline (task 090). Say so plainly
// instead of letting the user retype their number into a failing request.
const offline = computed(() => !app.online)

// A network failure has to read as "no signal", not as "wrong number". Its message
// also flips the connectivity flag, since a failed request is better evidence than
// navigator.onLine.
function messageFor(err: unknown): string {
  if (err instanceof NetworkError) {
    app.setOnline(false)
    return 'Ingen forbindelse. Find et sted med signal og prøv igen.'
  }
  return err instanceof Error ? err.message : 'Noget gik galt. Prøv igen.'
}

function changeNumber() {
  step.value = 'phone'
  pin.value = ''
  error.value = ''
  cooldown.value = 0
  clearInterval(cooldownTimer)
  session.clearChoice()
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <p class="text-center text-sm text-slate-500">Log ind med dit telefonnummer</p>

    <Alert
      v-if="offline"
      class="border-amber-300 bg-amber-50 text-amber-900"
      data-testid="login-offline"
    >
      <WifiOff aria-hidden="true" />
      <AlertTitle>Ingen forbindelse</AlertTitle>
      <AlertDescription class="text-amber-800">
        Du skal have signal for at logge ind, fordi koden kommer som SMS. Prøv et sted
        med bedre dækning.
      </AlertDescription>
    </Alert>

    <!-- Step 1: phone number -->
    <form v-if="step === 'phone'" class="flex flex-col gap-3" @submit.prevent="sendPin">
      <label class="text-sm font-medium" for="phone">Telefonnummer</label>
      <input
        id="phone"
        v-model="phone"
        type="tel"
        inputmode="tel"
        autocomplete="tel"
        autofocus
        class="rounded-lg border border-slate-300 px-4 py-3 text-lg"
        placeholder="+45 …"
      />
      <button
        type="submit"
        :disabled="busy || !phone"
        class="rounded-lg bg-slate-900 px-4 py-3 font-medium text-white transition disabled:opacity-50"
      >
        Send kode
      </button>
    </form>

    <!-- Step 2: PIN -->
    <form v-else-if="step === 'pin'" class="flex flex-col gap-3" @submit.prevent="submitPin">
      <label class="text-sm font-medium" for="pin">Kode fra SMS</label>
      <input
        id="pin"
        v-model="pin"
        type="text"
        inputmode="numeric"
        autocomplete="one-time-code"
        maxlength="6"
        autofocus
        class="rounded-lg border border-slate-300 px-4 py-3 text-center text-2xl tracking-[0.5em]"
        placeholder="••••••"
      />
      <button
        type="submit"
        :disabled="busy || !pin"
        class="rounded-lg bg-slate-900 px-4 py-3 font-medium text-white transition disabled:opacity-50"
      >
        Log ind
      </button>

      <div class="flex items-center justify-between text-sm">
        <button type="button" class="flex items-center gap-1 text-slate-500" @click="changeNumber">
          <ArrowLeft class="h-4 w-4" />
          Skift nummer
        </button>
        <button
          type="button"
          class="text-slate-500 disabled:opacity-50"
          :disabled="!canResend"
          @click="resendPin"
        >
          {{ cooldown > 0 ? `Send igen (${cooldown}s)` : 'Send igen' }}
        </button>
      </div>

      <p class="mt-2 text-center text-xs leading-relaxed text-slate-500">
        Kender vi dig, har vi sendt dig en SMS. Får du ingen SMS, og synes du vi
        burde kende dig, så
        <a :href="`tel:${NODTELEFON}`" class="inline-flex items-center gap-1 font-medium text-slate-700 underline">
          <Phone class="h-3.5 w-3.5" />
          ring til nødtelefonen ({{ NODTELEFON_DISPLAY }})</a
        >.
      </p>
    </form>

    <!-- Step 3: who are you? Only for a phone number shared by several people. The list itself is
         shared with the profile switcher (task 181), so both surfaces identify a person the same
         way. -->
    <div v-else class="flex flex-col gap-3">
      <p class="text-sm text-slate-600">
        Nummeret er registreret til flere. Hvem er du?
      </p>

      <ProfileChooser :candidates="session.choiceCandidates" :busy="busy" @choose="choose" />

      <button
        type="button"
        class="mt-1 flex items-center gap-1 self-start text-sm text-slate-500"
        @click="changeNumber"
      >
        <ArrowLeft class="h-4 w-4" />
        Skift nummer
      </button>

      <p class="mt-2 text-center text-xs leading-relaxed text-slate-500">
        Står du ikke på listen, så
        <a :href="`tel:${NODTELEFON}`" class="inline-flex items-center gap-1 font-medium text-slate-700 underline">
          <Phone class="h-3.5 w-3.5" />
          ring til nødtelefonen ({{ NODTELEFON_DISPLAY }})</a
        >.
      </p>
    </div>

    <p v-if="error" class="text-center text-sm text-red-600" role="alert">{{ error }}</p>
  </div>
</template>
