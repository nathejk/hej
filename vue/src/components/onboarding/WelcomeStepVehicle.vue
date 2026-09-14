<script setup lang="ts">
import { computed, ref } from 'vue'
import { Car } from '@lucide/vue'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useVehiclesStore } from '@/stores/vehicles.store'

// The vehicle step of onboarding (PRD 010).
//
// Two screens in one step, and the order is the whole design: a yes/no gate first, so the
// majority who bring nothing answer in one tap and never see a form, and the form only for the
// minority who do. Asking six fields of everybody would be the fastest way to make this step
// something people learn to skip.
//
// **Skippable, like everything but login** (PRD 005 §6). Answering "no" is not a recorded fact
// — there is nothing to store and nothing to read back — so it is a session skip, and the
// profile page is where somebody who later borrows a car registers it.
//
// Runs for every role except spejder; the store decides that (`mayRegisterVehicle`), and the
// BFF enforces it. Nothing here re-derives the rule.

const emit = defineEmits<{ done: []; skip: [] }>()

const vehicles = useVehiclesStore()

// Which of the two screens is showing. `false` is the gate.
const filling = ref(false)

const licensePlate = ref('')
const brand = ref('')
const model = ref('')
const color = ref('')
// A string rather than a number, because an <input type="number"> bound to a number gives an
// empty field the value 0 — and 0 seats is a meaningful answer here ("brought only for my own
// transport, not available for pickups"), so it must be something the member chose rather than
// something an empty field produced.
const seatCount = ref('')

const canSubmit = computed(() => licensePlate.value.trim().length > 0 && !vehicles.saving)

// The BFF's message, not a second opinion. A 409 in particular is not a failure to apologise
// for: the car is in the inventory, which is all this step wanted.
const error = computed(() => vehicles.writeError)

function startFilling() {
  vehicles.clearWriteError()
  filling.value = true
}

async function submit() {
  if (!canSubmit.value) return

  const parsedSeats = Number.parseInt(seatCount.value, 10)
  const created = await vehicles.register({
    licensePlate: licensePlate.value,
    brand: brand.value,
    model: model.value,
    color: color.value,
    seatCount: Number.isNaN(parsedSeats) ? 0 : parsedSeats,
  })

  // On failure the fields are deliberately left as they are: the member has just typed a
  // registration plate off a car, and clearing it to show an error would be the most annoying
  // possible response to "no signal".
  if (created) emit('done')
}

// A duplicate means the car is already registered — by the driver, most likely, with the
// passenger now filling in the same form. Nothing more to do, so this step is finished rather
// than failed.
function acceptDuplicate() {
  emit('skip')
}
</script>

<template>
  <div class="flex h-full flex-col justify-center gap-6">
    <div class="flex flex-col items-center gap-4 text-center">
      <div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-slate-900 text-white">
        <Car class="h-7 w-7" aria-hidden="true" />
      </div>
      <h1 class="font-nathejk text-3xl tracking-wide">Har du en bil med?</h1>
      <!--
        The purpose in the participant's terms. Not "we are collecting data": the cars on site
        are what a member stranded on the route gets collected in, and the ones nobody knows
        about cannot be asked.
      -->
      <p v-if="!filling" class="text-sm leading-relaxed text-slate-600">
        Vi holder styr på alle biler i løbsområdet — både til parkering og fordi bilerne er dem,
        vi kan sende ud efter en deltager, der skal hentes.
      </p>
      <p v-else class="text-sm leading-relaxed text-slate-600">
        Nummerpladen er det vigtigste. Resten hjælper med at genkende bilen, når den skal findes
        på en mørk parkeringsplads.
      </p>
    </div>

    <!-- The gate. One tap for the majority. -->
    <div v-if="!filling" class="flex flex-col items-stretch gap-2">
      <Button class="w-full" @click="startFilling">Ja, jeg har en bil med</Button>
      <Button variant="ghost" class="w-full" @click="emit('skip')">
        Nej, jeg har ikke en bil med
      </Button>
      <p class="text-center text-xs leading-relaxed text-slate-400">
        Du kan altid tilføje en bil senere under Min profil.
      </p>
    </div>

    <!-- The form. -->
    <form v-else class="flex flex-col gap-4" @submit.prevent="submit">
      <div class="flex flex-col gap-1.5">
        <Label for="vehicle-plate">Nummerplade</Label>
        <Input
          id="vehicle-plate"
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
          <Label for="vehicle-brand">Mærke</Label>
          <Input id="vehicle-brand" v-model="brand" name="brand" placeholder="VW" />
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="vehicle-model">Model</Label>
          <Input id="vehicle-model" v-model="model" name="model" placeholder="Transporter" />
        </div>
      </div>

      <div class="flex flex-col gap-1.5">
        <Label for="vehicle-color">Farve</Label>
        <Input id="vehicle-color" v-model="color" name="color" placeholder="Rød" />
      </div>

      <div class="flex flex-col gap-1.5">
        <!--
          The label carries "ud over dig selv", rather than a hint under the field, because a
          hint is the first thing a phone keyboard covers. An off-by-one here has a coordinator
          sending a car with one seat too few to collect somebody at 02:00.
        -->
        <Label for="vehicle-seats">Pladser ud over dig selv</Label>
        <Input
          id="vehicle-seats"
          v-model="seatCount"
          name="seats"
          type="number"
          inputmode="numeric"
          min="0"
          max="8"
          placeholder="0"
        />
        <p class="text-xs text-slate-400">
          Hvor mange deltagere kan du have med, når du selv sidder bag rattet? Skriv 0, hvis
          bilen kun er til dig selv.
        </p>
      </div>

      <p v-if="error" class="text-sm text-red-600" role="alert">{{ error.message }}</p>

      <div class="flex flex-col items-stretch gap-2">
        <Button type="submit" class="w-full" :disabled="!canSubmit">
          {{ vehicles.saving ? 'Gemmer …' : 'Registrér bil' }}
        </Button>

        <!-- A duplicate is an answer, not an error: somebody already registered this car. -->
        <Button
          v-if="error?.kind === 'duplicate'"
          type="button"
          variant="secondary"
          class="w-full"
          @click="acceptDuplicate"
        >
          Bilen er allerede registreret — fortsæt
        </Button>

        <Button type="button" variant="ghost" class="w-full" @click="emit('skip')">
          Spring over
        </Button>
      </div>
    </form>
  </div>
</template>
