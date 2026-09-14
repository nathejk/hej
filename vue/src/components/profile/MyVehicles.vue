<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Car, Plus, Trash2 } from '@lucide/vue'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { seatsLabel, vehicleSummary } from '@/helpers/vehicleLabels'
import { useSessionStore } from '@/stores/session.store'
import { useVehiclesStore } from '@/stores/vehicles.store'
import { type Vehicle, type VehicleInput } from '@/stores/vehicles.store'

// "Mine køretøjer" on the profile page (PRD 003 §17, owned by PRD 010).
//
// This is the surface that makes registration trustworthy rather than a one-shot question
// during onboarding: somebody who skipped registers here, and somebody whose plans changed
// corrects here.
//
// **No nag.** No badge, no warning, no "incomplete profile" for a member with no vehicle. Not
// bringing a car is an entirely ordinary state, and the app cannot tell "did not bother" from
// "came by train" — so implying fault would be both wrong and unfixable by the person reading
// it.
//
// Role-gated identically to the onboarding step: absent for a spejder rather than empty, since
// an empty section invites them to look for an action the BFF would refuse.

const vehicles = useVehiclesStore()
const session = useSessionStore()

// The same exclusion the step machine and the BFF use. Stated here only to decide whether to
// draw the section; the endpoints are the authority.
const mayRegister = computed(() => session.role !== null && session.role !== 'spejder')

onMounted(() => {
  if (mayRegister.value) void vehicles.ensureLoaded()
})

// —— The add/edit form ——
//
// One dialog for both, because the fields are identical and two would drift. `editing` holds
// the vehicle being changed, or null when adding.

const formOpen = ref(false)
const editing = ref<Vehicle | null>(null)

const licensePlate = ref('')
const brand = ref('')
const model = ref('')
const color = ref('')
// A string for the same reason as in the onboarding step: bound to a number, an empty
// <input type="number"> reads as 0, and 0 seats is a real answer ("only for my own transport")
// that has to come from the member rather than from an empty field.
const seatCount = ref('')

const canSubmit = computed(() => licensePlate.value.trim().length > 0 && !vehicles.saving)

function openAdd() {
  vehicles.clearWriteError()
  editing.value = null
  licensePlate.value = ''
  brand.value = ''
  model.value = ''
  color.value = ''
  seatCount.value = ''
  formOpen.value = true
}

function openEdit(vehicle: Vehicle) {
  vehicles.clearWriteError()
  editing.value = vehicle
  licensePlate.value = vehicle.licensePlate
  brand.value = vehicle.brand
  model.value = vehicle.model
  color.value = vehicle.color
  seatCount.value = String(vehicle.seatCount)
  formOpen.value = true
}

function formValues(): VehicleInput {
  const parsed = Number.parseInt(seatCount.value, 10)
  return {
    licensePlate: licensePlate.value,
    brand: brand.value,
    model: model.value,
    color: color.value,
    seatCount: Number.isNaN(parsed) ? 0 : parsed,
  }
}

async function submit() {
  if (!canSubmit.value) return

  const target = editing.value
  const ok = target
    ? await vehicles.update(target.id, formValues())
    : (await vehicles.register(formValues())) !== null

  // Left open on failure, with the fields as typed: the member has just read a plate off a car,
  // and closing the dialog would throw that away to show an error.
  if (ok) formOpen.value = false
}

// —— Removing ——
//
// Confirmed, because a plate is quick to delete and slow to retype, and the consequence is a
// car the coordinator can no longer send to collect somebody.

const removing = ref<Vehicle | null>(null)

function askRemove(vehicle: Vehicle) {
  vehicles.clearWriteError()
  removing.value = vehicle
}

async function confirmRemove() {
  const target = removing.value
  if (!target) return
  if (await vehicles.remove(target.id)) removing.value = null
}

/** Brand, model and colour as one line, skipping whatever is missing. */
function summaryOf(vehicle: Vehicle): string {
  return vehicleSummary(vehicle)
}

function seatsOf(vehicle: Vehicle): string {
  return seatsLabel(vehicle.seatCount)
}
</script>

<template>
  <section v-if="mayRegister">
    <div class="flex items-baseline justify-between gap-4">
      <h2 class="font-nathejk text-lg text-slate-900">Mine køretøjer</h2>
      <!-- Default size, not `sm`: the button index notes xs/sm are opt-in for dense secondary
           affordances, and this is a primary action on a one-handed phone surface where 44px is
           a hard requirement (task 010). -->
      <Button variant="ghost" @click="openAdd">
        <Plus class="h-4 w-4" aria-hidden="true" />
        Tilføj
      </Button>
    </div>

    <p v-if="vehicles.loading && !vehicles.loaded" class="mt-2 text-sm text-slate-500">Henter …</p>

    <!-- A failed read says so instead of showing an empty list: "we could not reach the server"
         must not read as "you have nothing registered", or the member registers a second row for
         a car that is already there. -->
    <p v-else-if="vehicles.error" class="mt-2 rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-900">
      {{ vehicles.error }}
    </p>

    <ul
      v-else-if="vehicles.hasAny"
      class="mt-2 divide-y divide-slate-100 rounded-xl border border-slate-200 bg-white shadow-xs"
    >
      <li
        v-for="vehicle in vehicles.vehicles"
        :key="vehicle.id"
        class="flex items-center gap-3 px-4 py-3"
      >
        <Car class="h-5 w-5 shrink-0 text-slate-400" aria-hidden="true" />
        <div class="min-w-0 flex-1">
          <p class="truncate text-sm font-medium text-slate-900">{{ vehicle.licensePlate }}</p>
          <p v-if="summaryOf(vehicle)" class="truncate text-xs text-slate-500">
            {{ summaryOf(vehicle) }}
          </p>
          <p class="text-xs text-slate-400">{{ seatsOf(vehicle) }}</p>
        </div>
        <Button variant="ghost" @click="openEdit(vehicle)">Ret</Button>
        <Button
          variant="ghost"
          size="icon"
          :aria-label="`Fjern ${vehicle.licensePlate}`"
          @click="askRemove(vehicle)"
        >
          <Trash2 class="h-4 w-4" aria-hidden="true" />
        </Button>
      </li>
    </ul>

    <!-- The empty state. Reads as ordinary on purpose — see the component comment. -->
    <p v-else class="mt-2 text-sm text-slate-500">
      Du har ingen køretøjer registreret. Tager du bil med til Nathejk, må du gerne skrive den op —
      så ved vi, hvad der står i området, og hvilke biler vi kan sende ud efter en deltager.
    </p>

    <Dialog v-model:open="formOpen">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{{ editing ? 'Ret køretøj' : 'Tilføj køretøj' }}</DialogTitle>
          <DialogDescription>
            Nummerpladen er det vigtigste. Resten hjælper med at genkende bilen i mørke.
          </DialogDescription>
        </DialogHeader>

        <form class="flex flex-col gap-4" @submit.prevent="submit">
          <div class="flex flex-col gap-1.5">
            <Label for="my-vehicle-plate">Nummerplade</Label>
            <Input
              id="my-vehicle-plate"
              v-model="licensePlate"
              name="plate"
              placeholder="AB 12 345"
              autocapitalize="characters"
              autocomplete="off"
              required
            />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div class="flex flex-col gap-1.5">
              <Label for="my-vehicle-brand">Mærke</Label>
              <Input id="my-vehicle-brand" v-model="brand" name="brand" placeholder="VW" />
            </div>
            <div class="flex flex-col gap-1.5">
              <Label for="my-vehicle-model">Model</Label>
              <Input id="my-vehicle-model" v-model="model" name="model" placeholder="Transporter" />
            </div>
          </div>

          <div class="flex flex-col gap-1.5">
            <Label for="my-vehicle-color">Farve</Label>
            <Input id="my-vehicle-color" v-model="color" name="color" placeholder="Rød" />
          </div>

          <div class="flex flex-col gap-1.5">
            <!-- "ud over dig selv" belongs in the label, not a hint: a hint is the first thing
                 a phone keyboard covers, and an off-by-one here sends a car with one seat too
                 few to collect somebody at 02:00. -->
            <Label for="my-vehicle-seats">Pladser ud over dig selv</Label>
            <Input
              id="my-vehicle-seats"
              v-model="seatCount"
              name="seats"
              type="number"
              inputmode="numeric"
              min="0"
              max="8"
              placeholder="0"
            />
          </div>

          <p v-if="vehicles.writeError" class="text-sm text-red-600" role="alert">
            {{ vehicles.writeError.message }}
          </p>

          <DialogFooter>
            <Button type="submit" class="w-full" :disabled="!canSubmit">
              {{ vehicles.saving ? 'Gemmer …' : 'Gem' }}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>

    <!-- Confirmation before removal. A Dialog rather than a native confirm(): the latter is
         suppressible, unstyled, and on iOS names the site rather than the car. -->
    <Dialog :open="removing !== null" @update:open="(open) => !open && (removing = null)">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Fjern køretøj?</DialogTitle>
          <DialogDescription>
            {{ removing?.licensePlate }} bliver taget ud af oversigten, og vi kan ikke længere
            sende den ud efter en deltager. Du kan skrive den op igen senere.
          </DialogDescription>
        </DialogHeader>

        <p v-if="vehicles.writeError" class="text-sm text-red-600" role="alert">
          {{ vehicles.writeError.message }}
        </p>

        <DialogFooter class="gap-2">
          <Button variant="ghost" class="w-full" @click="removing = null">Behold</Button>
          <Button variant="destructive" class="w-full" :disabled="vehicles.saving" @click="confirmRemove">
            {{ vehicles.saving ? 'Fjerner …' : 'Fjern' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </section>
</template>
