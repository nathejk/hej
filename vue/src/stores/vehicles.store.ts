import { defineStore } from 'pinia'

import { fetchWrapper, HttpError, NetworkError } from '@/helpers'

// vehicles.store holds the signed-in member's own vehicles (PRD 010): the cars they
// bring to the race, and from task 246 their trailers.
//
// One store behind both surfaces that show them — the onboarding step and the
// profile page's "Mine køretøjer" — so the two cannot disagree about what the member
// has registered.
//
// Three decisions worth keeping in one place rather than in each component:
//
// - **Nothing is added optimistically.** A registration is a write and needs
//   connectivity; the list only ever reflects what the server confirmed. PRD 008 §5 is
//   explicit that a write which could not be published has not happened, and a member
//   who believes their car is in the inventory when it is not is precisely the car
//   nobody dispatches at 02:00.
// - **Offline is a legible error, not a queue.** No background sync: a registration
//   that lands hours later, after the member has driven in and parked, is worse than
//   being told plainly to try again with signal — and the machinery would have to be
//   trusted for a write whose whole value is being correct before the event.
// - **Nothing is persisted.** The list is small and cheap to re-fetch, and it must be
//   visible only to its owner (PRD 010 §6). A cached copy would be a plate surviving on
//   a shared handset after its owner signed out; keeping it in memory only means a
//   profile switch, which reloads the page, cannot carry it over either.

export interface Vehicle {
  id: string
  /** The canonical prefixed form the BFF stores, e.g. "DK+AB12345". */
  licensePlate: string
  brand: string
  model: string
  color: string
  /** Excludes the driver. Every label the user sees has to say so (PRD 010 §7). */
  seatCount: number
  description: string
}

/** The registration form's fields. The custodian is the session's user, server-side. */
export interface VehicleInput {
  licensePlate: string
  brand?: string
  model?: string
  color?: string
  seatCount?: number
  description?: string
}

interface VehiclePayload {
  id: string
  license_plate: string
  brand: string
  model: string
  color: string
  seat_count: number
  description: string
}

/**
 * Why a write failed, as a shape the caller can branch on.
 *
 * `duplicate` exists because a 409 is not a failure to report as one: it is the
 * answer "this car is already registered", which the component turns into a sentence
 * and, for a passenger whose driver already registered the car, is arguably a
 * success. Collapsing every failure into one message string would make that
 * impossible to tell apart from a broken request.
 */
export type VehicleWriteError =
  | { kind: 'duplicate'; message: string }
  | { kind: 'offline'; message: string }
  | { kind: 'invalid'; message: string }
  | { kind: 'unavailable'; message: string }
  | { kind: 'failed'; message: string }

function toVehicle(p: VehiclePayload): Vehicle {
  return {
    id: p.id,
    licensePlate: p.license_plate,
    brand: p.brand,
    model: p.model,
    color: p.color,
    seatCount: p.seat_count,
    description: p.description,
  }
}

function toPayload(input: VehicleInput) {
  return {
    license_plate: input.licensePlate,
    brand: input.brand ?? '',
    model: input.model ?? '',
    color: input.color ?? '',
    seat_count: input.seatCount ?? 0,
    description: input.description ?? '',
  }
}

function classify(err: unknown): VehicleWriteError {
  if (err instanceof NetworkError) {
    return {
      kind: 'offline',
      message: 'Ingen forbindelse. Prøv igen når du har signal.',
    }
  }
  if (err instanceof HttpError) {
    if (err.status === 409) {
      return { kind: 'duplicate', message: 'Køretøjet er allerede registreret.' }
    }
    if (err.status === 400) {
      return { kind: 'invalid', message: err.message || 'Oplysningerne kunne ikke bruges.' }
    }
    // 503 is the BFF saying the event stream is not reachable, so the registration was
    // not recorded and nothing is wrong with what the member typed. Worth its own
    // sentence: "try again in a moment" is actionable, where the generic failure text
    // invites them to keep re-submitting a form that will keep failing.
    if (err.status === 503) {
      return {
        kind: 'unavailable',
        message: 'Kan ikke gemme lige nu. Prøv igen om et øjeblik.',
      }
    }
  }
  return { kind: 'failed', message: 'Kunne ikke gemme køretøjet. Prøv igen.' }
}

export const useVehiclesStore = defineStore('vehicles', {
  state: () => ({
    vehicles: [] as Vehicle[],
    loading: false,
    loaded: false,
    /** Why the last read failed. Separate from writeError: a failed list must not look like a failed save. */
    error: '',
    writeError: null as VehicleWriteError | null,
    saving: false,
  }),
  getters: {
    hasAny: (state) => state.vehicles.length > 0,
  },
  actions: {
    /**
     * Loads the caller's vehicles. Never throws — every surface that shows them has
     * to render while this is failing.
     *
     * A failed load leaves the previous list alone rather than emptying it: "we could
     * not reach the server" is not "you have no vehicles", and the difference decides
     * whether a member registers a duplicate of a car that is already there.
     */
    async load() {
      this.loading = true
      try {
        const data = await fetchWrapper.get<{ vehicles: VehiclePayload[] | null }>(
          '/api/me/vehicles',
        )
        this.vehicles = (data.vehicles ?? []).map(toVehicle)
        this.error = ''
        this.loaded = true
      } catch {
        this.error = 'Kunne ikke hente dine køretøjer.'
      } finally {
        this.loading = false
      }
    },

    /** Loads once. Both surfaces call this on mount, so the second is free. */
    async ensureLoaded() {
      if (this.loaded || this.loading) return
      await this.load()
    },

    /**
     * Registers a vehicle. Returns the created vehicle, or null with `writeError` set.
     *
     * The server's response is what enters the list, not the submitted form: it
     * carries the id and the *normalised* plate, so what the member sees afterwards is
     * the string the inventory will compare next time rather than what they typed.
     */
    async register(input: VehicleInput): Promise<Vehicle | null> {
      this.saving = true
      this.writeError = null
      try {
        const created = await fetchWrapper.post<VehiclePayload>('/api/me/vehicles', toPayload(input))
        const vehicle = toVehicle(created)
        this.vehicles = [...this.vehicles, vehicle]
        return vehicle
      } catch (err) {
        this.writeError = classify(err)
        return null
      } finally {
        this.saving = false
      }
    },

    /**
     * Edits a vehicle. `changes` is a delta and is sent as one: only the keys present
     * are transmitted, so an untouched field is left alone server-side and one sent
     * empty is cleared.
     *
     * Re-reads the list afterwards rather than patching the local copy, because the
     * server may have normalised the plate to something other than what was typed.
     */
    async update(id: string, changes: Partial<VehicleInput>): Promise<boolean> {
      this.saving = true
      this.writeError = null
      try {
        const body: Record<string, unknown> = {}
        if (changes.licensePlate !== undefined) body.license_plate = changes.licensePlate
        if (changes.brand !== undefined) body.brand = changes.brand
        if (changes.model !== undefined) body.model = changes.model
        if (changes.color !== undefined) body.color = changes.color
        if (changes.seatCount !== undefined) body.seat_count = changes.seatCount
        if (changes.description !== undefined) body.description = changes.description

        await fetchWrapper.patch(`/api/me/vehicles/${id}`, body)
        await this.load()
        return true
      } catch (err) {
        this.writeError = classify(err)
        return false
      } finally {
        this.saving = false
      }
    },

    /**
     * Withdraws a vehicle.
     *
     * A 404 counts as success: the server answers 404 for a vehicle that is already
     * gone (it cannot distinguish that from one that never existed without disclosing
     * whose registrations exist — see the BFF handler), and telling a member their
     * deletion failed when the car is demonstrably gone would be a lie.
     */
    async remove(id: string): Promise<boolean> {
      this.saving = true
      this.writeError = null
      try {
        await fetchWrapper.delete(`/api/me/vehicles/${id}`)
      } catch (err) {
        if (!(err instanceof HttpError && err.status === 404)) {
          this.writeError = classify(err)
          return false
        }
      } finally {
        this.saving = false
      }
      this.vehicles = this.vehicles.filter((v) => v.id !== id)
      return true
    },

    /** Drops the last write error, so re-opening a form does not show a stale one. */
    clearWriteError() {
      this.writeError = null
    },

    /**
     * Empties the store on sign-out.
     *
     * Not merely tidiness: these are plates belonging to a specific person, and a
     * shared handset is the normal case at Nathejk. Sign-out stays in the same
     * document, so without this the next member to sign in would see the previous
     * one's cars until their own fetch resolved. `loaded` has to go back to false
     * too, or their surfaces would skip that fetch entirely.
     *
     * A **profile switch** needs no call here: it does a full page load
     * (`UserMenu.switchTo`), precisely so no in-memory store can survive it, and
     * nothing here is persisted.
     */
    clear() {
      this.vehicles = []
      this.loaded = false
      this.loading = false
      this.error = ''
      this.writeError = null
      this.saving = false
    },
  },
})
