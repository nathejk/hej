<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { KeyRound, Phone, ArrowLeft, ChevronRight, WifiOff } from '@lucide/vue'
import { HttpError, NetworkError } from '@/helpers'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { useSessionStore } from '@/stores/session.store'
import { useAppStore } from '@/stores/app.store'
import { NODTELEFON, NODTELEFON_DISPLAY } from '@/config/contact'
import { APP_NAME } from '@/config/brand'

// Three-step phone login: phone → PIN → session, with a `choose` step in between for
// the ~1 in 8 numbers that belong to more than one person (task 079). Passwordless:
// the only secret is the one-time SMS PIN. The PIN step always follows the phone step
// regardless
// of whether the number is recognized (anti-enumeration).
const RESEND_COOLDOWN_SECONDS = 60

const session = useSessionStore()
const app = useAppStore()
const router = useRouter()

const version = __APP_VERSION__

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
    // Landing is defined by the router (/ → maps); routing to '/' keeps this
    // decoupled from the concrete first-destination name.
    await router.replace({ path: '/' })
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
    await router.replace({ path: '/' })
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
  <main
    class="mx-auto flex h-full w-full max-w-sm flex-col justify-center gap-8 px-6"
    style="padding-top: env(safe-area-inset-top); padding-bottom: env(safe-area-inset-bottom)"
  >
    <header class="flex flex-col items-center gap-3 text-center">
      <div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-slate-900 text-white">
        <KeyRound class="h-7 w-7" />
      </div>
      <h1 class="font-nathejk text-3xl tracking-wide">{{ APP_NAME }}</h1>
      <p class="text-sm text-slate-500">Log ind med dit telefonnummer</p>
    </header>

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

    <!-- Step 3: who are you? Only for a phone number shared by several people. -->
    <div v-else class="flex flex-col gap-3">
      <p class="text-sm text-slate-600">
        Nummeret er registreret til flere. Hvem er du?
      </p>

      <button
        v-for="c in session.choiceCandidates"
        :key="c.user_id"
        type="button"
        :disabled="busy"
        class="flex items-center justify-between rounded-lg border border-slate-300 px-4 py-3 text-left transition disabled:opacity-50"
        @click="choose(c.user_id)"
      >
        <span>
          <span class="font-medium">{{ c.name }}</span>
          <!-- Affiliation is what actually disambiguates: two siblings share a
               patrulje but not a name, two crew may share a name but not a section. -->
          <span v-if="c.team || c.section" class="block text-sm text-slate-500">
            {{ c.team || c.section }}
          </span>
        </span>
        <ChevronRight class="h-4 w-4 shrink-0 text-slate-400" />
      </button>

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

    <p class="text-center text-xs text-slate-300">v{{ version }}</p>
  </main>
</template>
