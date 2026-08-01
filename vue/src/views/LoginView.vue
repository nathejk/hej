<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { KeyRound, Phone, ArrowLeft } from 'lucide-vue-next'
import { HttpError } from '@/helpers'
import { useSessionStore } from '@/stores/session.store'
import { NODTELEFON, NODTELEFON_DISPLAY } from '@/config/contact'

// Two-step phone login: phone → PIN → session. Passwordless: the only secret is
// the one-time SMS PIN. The PIN step always follows the phone step regardless
// of whether the number is recognized (anti-enumeration).
const RESEND_COOLDOWN_SECONDS = 60

const session = useSessionStore()
const router = useRouter()

const step = ref<'phone' | 'pin'>('phone')
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
    error.value = err instanceof Error ? err.message : 'Noget gik galt. Prøv igen.'
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
    await session.verify(phone.value, pin.value)
    await router.replace({ name: 'home' })
  } catch (err) {
    if (err instanceof HttpError && err.status === 401) {
      error.value = 'Koden passer ikke. Prøv igen.'
    } else if (err instanceof HttpError && err.status === 429) {
      error.value = 'For mange forsøg. Bed om en ny kode og prøv igen.'
    } else {
      error.value = err instanceof Error ? err.message : 'Noget gik galt. Prøv igen.'
    }
  } finally {
    busy.value = false
  }
}

function changeNumber() {
  step.value = 'phone'
  pin.value = ''
  error.value = ''
  cooldown.value = 0
  clearInterval(cooldownTimer)
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
      <h1 class="text-2xl font-bold">Hej Nathejk</h1>
      <p class="text-sm text-slate-500">Log ind med dit telefonnummer</p>
    </header>

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
    <form v-else class="flex flex-col gap-3" @submit.prevent="submitPin">
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

    <p v-if="error" class="text-center text-sm text-red-600" role="alert">{{ error }}</p>
  </main>
</template>
