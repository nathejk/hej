import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

// Mocked at the module boundary so the store is tested without a network.
vi.mock('@/helpers', async () => {
  const actual = await vi.importActual<typeof import('@/helpers')>('@/helpers')
  return {
    ...actual,
    fetchWrapper: {
      get: (url: string) => getMock(url),
      post: (url: string, body?: unknown) => postMock(url, body),
      patch: (url: string, body?: unknown) => patchMock(url, body),
      put: vi.fn(),
      delete: (url: string) => deleteMock(url),
    },
  }
})

import { HttpError, NetworkError } from '@/helpers'
import { useVehiclesStore } from '@/stores/vehicles.store'

let getMock: (url: string) => Promise<unknown>
let postMock: (url: string, body?: unknown) => Promise<unknown>
let patchMock: (url: string, body?: unknown) => Promise<unknown>
let deleteMock: (url: string) => Promise<unknown>

function payload(over: Partial<Record<string, unknown>> = {}) {
  return {
    id: 'v1',
    license_plate: 'DK+AB12345',
    brand: 'VW',
    model: 'Transporter',
    color: 'rød',
    seat_count: 4,
    description: '',
    ...over,
  }
}

beforeEach(() => {
  setActivePinia(createPinia())
  getMock = () => Promise.resolve({ vehicles: [] })
  postMock = () => Promise.resolve(payload())
  patchMock = () => Promise.resolve(null)
  deleteMock = () => Promise.resolve(null)
})

describe('load', () => {
  it('maps the BFF payload, including the seat count', async () => {
    getMock = () => Promise.resolve({ vehicles: [payload()] })
    const store = useVehiclesStore()
    await store.load()

    expect(store.vehicles).toHaveLength(1)
    expect(store.vehicles[0].licensePlate).toBe('DK+AB12345')
    expect(store.vehicles[0].seatCount).toBe(4)
    expect(store.loaded).toBe(true)
  })

  it('treats an empty list as a normal state, not an error', async () => {
    const store = useVehiclesStore()
    await store.load()

    expect(store.vehicles).toEqual([])
    expect(store.hasAny).toBe(false)
    expect(store.error).toBe('')
    expect(store.loaded).toBe(true)
  })

  // "We could not reach the server" is not "you have no vehicles". Emptying the list on
  // a failed read is what would invite a member to register a duplicate of a car that is
  // already in the inventory.
  it('leaves the previous list alone when the read fails', async () => {
    getMock = () => Promise.resolve({ vehicles: [payload()] })
    const store = useVehiclesStore()
    await store.load()

    getMock = () => Promise.reject(new NetworkError('/api/me/vehicles'))
    await store.load()

    expect(store.vehicles).toHaveLength(1)
    expect(store.error).not.toBe('')
  })

  it('does not throw when the read fails, so the surfaces still render', async () => {
    getMock = () => Promise.reject(new NetworkError('/api/me/vehicles'))
    const store = useVehiclesStore()
    await expect(store.load()).resolves.toBeUndefined()
  })
})

describe('register', () => {
  // The server's response enters the list, not the submitted form: it carries the id and
  // the normalised plate, so what the member sees next is what the inventory compares.
  it('adds the server\'s version of the vehicle, not the typed one', async () => {
    postMock = () => Promise.resolve(payload({ id: 'v-new', license_plate: 'DK+AB12345' }))
    const store = useVehiclesStore()

    const created = await store.register({ licensePlate: 'ab 12 345' })

    expect(created?.id).toBe('v-new')
    expect(created?.licensePlate).toBe('DK+AB12345')
    expect(store.vehicles.map((v) => v.licensePlate)).toEqual(['DK+AB12345'])
    expect(store.writeError).toBeNull()
  })

  // Nothing may enter the list that the server did not confirm. A member who believes
  // their car is registered when the publish failed is the car nobody dispatches.
  it('adds nothing when the write fails', async () => {
    postMock = () => Promise.reject(new NetworkError('/api/me/vehicles'))
    const store = useVehiclesStore()

    const created = await store.register({ licensePlate: 'ab12345' })

    expect(created).toBeNull()
    expect(store.vehicles).toEqual([])
  })

  it('reports offline distinguishably, so the message can be actionable', async () => {
    postMock = () => Promise.reject(new NetworkError('/api/me/vehicles'))
    const store = useVehiclesStore()
    await store.register({ licensePlate: 'ab12345' })

    expect(store.writeError?.kind).toBe('offline')
  })

  // A 409 is an answer, not a breakage: this car is already registered, quite possibly by
  // the driver whose passenger is now filling in the same form.
  it('reports a duplicate distinguishably from a generic failure', async () => {
    postMock = () => Promise.reject(new HttpError(409, 'køretøjet er allerede registreret'))
    const store = useVehiclesStore()
    await store.register({ licensePlate: 'ab12345' })

    expect(store.writeError?.kind).toBe('duplicate')

    postMock = () => Promise.reject(new HttpError(500, 'boom'))
    await store.register({ licensePlate: 'ab12345' })
    expect(store.writeError?.kind).toBe('failed')
  })

  it('sends the seat count and defaults the optional fields', async () => {
    let sent: unknown
    postMock = (_url, body) => {
      sent = body
      return Promise.resolve(payload())
    }
    const store = useVehiclesStore()
    await store.register({ licensePlate: 'ab12345', seatCount: 4 })

    expect(sent).toMatchObject({ license_plate: 'ab12345', seat_count: 4, brand: '' })
  })
})

describe('update', () => {
  // The delta is the whole point of PATCH here: an untouched field must not be
  // transmitted, or the server cannot tell "leave it" from "clear it".
  it('sends only the keys it was given', async () => {
    let sent: Record<string, unknown> | undefined
    patchMock = (_url, body) => {
      sent = body as Record<string, unknown>
      return Promise.resolve(null)
    }
    const store = useVehiclesStore()
    await store.update('v1', { color: 'blå' })

    expect(sent).toEqual({ color: 'blå' })
  })

  // The other half: an empty string is a value, and must survive as one.
  it('sends an empty string rather than omitting it', async () => {
    let sent: Record<string, unknown> | undefined
    patchMock = (_url, body) => {
      sent = body as Record<string, unknown>
      return Promise.resolve(null)
    }
    const store = useVehiclesStore()
    await store.update('v1', { description: '' })

    expect(sent).toEqual({ description: '' })
  })

  it('re-reads the list, so a server-normalised plate is what is shown', async () => {
    getMock = () => Promise.resolve({ vehicles: [payload({ license_plate: 'DK+XY98765' })] })
    const store = useVehiclesStore()

    expect(await store.update('v1', { licensePlate: 'xy 98 765' })).toBe(true)
    expect(store.vehicles[0].licensePlate).toBe('DK+XY98765')
  })

  it('surfaces a conflict on a plate change', async () => {
    patchMock = () => Promise.reject(new HttpError(409, 'allerede registreret'))
    const store = useVehiclesStore()

    expect(await store.update('v1', { licensePlate: 'xy98765' })).toBe(false)
    expect(store.writeError?.kind).toBe('duplicate')
  })
})

describe('remove', () => {
  it('drops the vehicle from the list', async () => {
    getMock = () => Promise.resolve({ vehicles: [payload({ id: 'v1' }), payload({ id: 'v2' })] })
    const store = useVehiclesStore()
    await store.load()

    expect(await store.remove('v1')).toBe(true)
    expect(store.vehicles.map((v) => v.id)).toEqual(['v2'])
  })

  // The BFF answers 404 for a car that is already gone, because it cannot distinguish
  // that from one that never existed without disclosing whose registrations exist.
  // Telling the member their deletion failed would be a lie.
  it('treats a 404 as success', async () => {
    getMock = () => Promise.resolve({ vehicles: [payload({ id: 'v1' })] })
    const store = useVehiclesStore()
    await store.load()

    deleteMock = () => Promise.reject(new HttpError(404, 'not found'))

    expect(await store.remove('v1')).toBe(true)
    expect(store.vehicles).toEqual([])
    expect(store.writeError).toBeNull()
  })

  it('keeps the vehicle when the delete genuinely fails', async () => {
    getMock = () => Promise.resolve({ vehicles: [payload({ id: 'v1' })] })
    const store = useVehiclesStore()
    await store.load()

    deleteMock = () => Promise.reject(new NetworkError('/api/me/vehicles/v1'))

    expect(await store.remove('v1')).toBe(false)
    expect(store.vehicles).toHaveLength(1)
    expect(store.writeError?.kind).toBe('offline')
  })
})

describe('clear', () => {
  // A shared handset is the normal case at Nathejk, and a plate belongs to a person.
  it('empties the store and re-arms the fetch for the next member', async () => {
    getMock = () => Promise.resolve({ vehicles: [payload()] })
    const store = useVehiclesStore()
    await store.load()
    expect(store.loaded).toBe(true)

    store.clear()

    expect(store.vehicles).toEqual([])
    // If `loaded` stayed true, the next member's surfaces would skip their own fetch.
    expect(store.loaded).toBe(false)
  })
})

describe('ensureLoaded', () => {
  it('fetches once', async () => {
    let calls = 0
    getMock = () => {
      calls += 1
      return Promise.resolve({ vehicles: [] })
    }
    const store = useVehiclesStore()
    await store.ensureLoaded()
    await store.ensureLoaded()

    expect(calls).toBe(1)
  })
})
