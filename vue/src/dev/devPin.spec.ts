import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { devPinPhone, devPinStatus, devPinValue, fetchDevPin, initDevPin } from '@/dev/devPin'
import { HttpError, fetchWrapper } from '@/helpers'
import { useSessionStore } from '@/stores/session.store'

describe('dev pin readout', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    devPinPhone.value = ''
    devPinValue.value = ''
    devPinStatus.value = ''
    vi.restoreAllMocks()
  })

  it('fetches the issued pin for a number', async () => {
    const get = vi.spyOn(fetchWrapper, 'get').mockResolvedValue({ pin: '830254' })

    await fetchDevPin('42453977')

    expect(get).toHaveBeenCalledWith('/api/dev/pin?phone=42453977')
    expect(devPinValue.value).toBe('830254')
    expect(devPinStatus.value).toBe('')
  })

  // A 404 is the endpoint's ordinary "nothing outstanding" answer — and it is deliberately
  // indistinguishable from an unknown number, so it cannot be used to enumerate numbers.
  // Reporting it as an error would make a normal state look broken.
  it('reads a 404 as "none issued yet", not as a failure', async () => {
    vi.spyOn(fetchWrapper, 'get').mockRejectedValue(new HttpError(404, 'not found'))

    await fetchDevPin('42453977')

    expect(devPinValue.value).toBe('')
    expect(devPinStatus.value).toBe('no pin issued yet')
  })

  it('refuses an empty number rather than asking for one', async () => {
    const get = vi.spyOn(fetchWrapper, 'get')
    await fetchDevPin('   ')
    expect(get).not.toHaveBeenCalled()
    expect(devPinStatus.value).toBe('no number')
  })

  // So the number does not have to be typed twice — once into the login form and once here.
  it('picks up the number the app just requested a pin for', async () => {
    const get = vi.spyOn(fetchWrapper, 'get').mockResolvedValue({ pin: '112233' })
    vi.spyOn(fetchWrapper, 'post').mockResolvedValue({ message: 'sent' })
    initDevPin()

    await useSessionStore().requestPin('42453977')
    // The observer fires after the POST resolves — there is no PIN to read until the BFF has
    // issued one.
    await vi.waitFor(() => expect(devPinValue.value).toBe('112233'))
    expect(get).toHaveBeenCalledWith('/api/dev/pin?phone=42453977')
  })
})
