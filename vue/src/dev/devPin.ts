import { ref } from 'vue'

import { fetchWrapper, HttpError } from '@/helpers'
import { setDevPhoneObserver } from '@/stores/session.store'

// The dev panel's PIN readout (PRD 014, task 216).
//
// `sms.LogSender` already puts the PIN in the API log, so nothing new is disclosed here — this
// only saves a terminal round-trip, paid on every login, switch-profile and shared-number
// `/auth/choose` test.
//
// # Privacy boundary
//
// The phone number below is the one the developer just typed into the login form. That is the
// **only** personal datum this layer may show: no name, no role, and never a guardian number
// (`.rules`). The endpoint itself returns nothing else — see `go/cmd/api/dev.go`.
//
// # It does not skip the input
//
// The PIN is displayed and can be copied; it is not posted around the OTP field. The field's own
// behaviour — paste, autofill, validation, the resend timer — is under test too, and a shortcut
// past it would leave the part of login that most often breaks unexercised.

const phone = ref('')
const pin = ref('')
const status = ref('')

export const devPinPhone = phone
export const devPinValue = pin
export const devPinStatus = status

/** Asks the dev-only BFF endpoint for the PIN currently issued to `number`. */
export async function fetchDevPin(number: string = phone.value) {
  const trimmed = number.trim()
  if (!trimmed) {
    status.value = 'no number'
    return
  }
  phone.value = trimmed
  pin.value = ''
  status.value = 'fetching…'
  try {
    const body = await fetchWrapper.get<{ pin: string }>(
      `/api/dev/pin?phone=${encodeURIComponent(trimmed)}`,
    )
    pin.value = body.pin
    status.value = ''
  } catch (err) {
    // A 404 is the ordinary "nothing outstanding" answer, not a failure: the endpoint returns it
    // for an unknown number and a known-but-PIN-less one alike, deliberately, so it cannot be
    // used to enumerate numbers. Reporting it as an error would make a normal state look broken.
    if (err instanceof HttpError && err.status === 404) {
      status.value = 'no pin issued yet'
      return
    }
    // A 404 from a *missing route* looks identical from here — which is worth saying out loud,
    // because that is what a developer sees if the API is not running with ENV=development.
    status.value = err instanceof Error ? err.message : 'failed'
  }
}

export async function copyDevPin() {
  if (!pin.value) return
  try {
    await navigator.clipboard.writeText(pin.value)
    status.value = 'copied'
  } catch {
    // Clipboard access needs a secure context and a user gesture; both hold here, but a refusal
    // is not worth more than a note — the PIN is on screen to be read.
    status.value = 'copy refused — read it above'
  }
}

/**
 * Watches for login attempts so the PIN appears without anyone typing the number twice.
 *
 * Called once from `@/dev/bootstrap`.
 */
export function initDevPin() {
  setDevPhoneObserver((requested) => {
    void fetchDevPin(requested)
  })
}
