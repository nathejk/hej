<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { KeyRound } from 'lucide-vue-next'
import { HttpError } from '@/helpers'
import { useSessionStore } from '@/stores/session.store'

// Minimal but functional login: phone step → PIN step → session. The richer UX
// (resend/cooldown, one-time-code autofill, nødtelefon link, branding polish)
// is layered on in task 007.
const session = useSessionStore()
const router = useRouter()

const step = ref<'phone' | 'pin'>('phone')
const phone = ref('')
const pin = ref('')
const error = ref('')
const busy = ref(false)

async function submitPhone() {
  error.value = ''
  busy.value = true
  try {
    await session.requestPin(phone.value)
    step.value = 'pin'
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Something went wrong'
  } finally {
    busy.value = false
  }
}

async function submitPin() {
  error.value = ''
  busy.value = true
  try {
    await session.verify(phone.value, pin.value)
    await router.replace({ name: 'home' })
  } catch (err) {
    if (err instanceof HttpError && err.status === 401) {
      error.value = 'That PIN did not match. Please try again.'
    } else {
      error.value = err instanceof Error ? err.message : 'Something went wrong'
    }
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <main class="flex h-full flex-col items-center justify-center gap-6 p-6">
    <div class="flex flex-col items-center gap-2 text-center">
      <KeyRound class="h-10 w-10 text-slate-700" />
      <h1 class="text-2xl font-bold">Hej Nathejk</h1>
    </div>

    <form
      v-if="step === 'phone'"
      class="flex w-full max-w-xs flex-col gap-3"
      @submit.prevent="submitPhone"
    >
      <label class="text-sm font-medium" for="phone">Telefonnummer</label>
      <input
        id="phone"
        v-model="phone"
        type="tel"
        inputmode="tel"
        autocomplete="tel"
        class="rounded border border-slate-300 px-3 py-2"
        placeholder="+45 …"
      />
      <button
        type="submit"
        :disabled="busy || !phone"
        class="rounded bg-slate-900 px-3 py-2 font-medium text-white disabled:opacity-50"
      >
        Send kode
      </button>
    </form>

    <form
      v-else
      class="flex w-full max-w-xs flex-col gap-3"
      @submit.prevent="submitPin"
    >
      <label class="text-sm font-medium" for="pin">Kode fra SMS</label>
      <input
        id="pin"
        v-model="pin"
        type="text"
        inputmode="numeric"
        autocomplete="one-time-code"
        class="rounded border border-slate-300 px-3 py-2 text-center tracking-widest"
        placeholder="••••••"
      />
      <button
        type="submit"
        :disabled="busy || !pin"
        class="rounded bg-slate-900 px-3 py-2 font-medium text-white disabled:opacity-50"
      >
        Log ind
      </button>
      <p class="text-center text-xs text-slate-500">
        If we know you, we have sent you an SMS. If you don't receive an SMS and
        you feel we should know you, please reach out.
      </p>
    </form>

    <p v-if="error" class="text-sm text-red-600">{{ error }}</p>
  </main>
</template>
