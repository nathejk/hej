<script setup lang="ts">
// Min profil (PRD 003): the details Nathejk holds about you, your portrait, and
// what this device is allowed to do.
//
// Reached from the user menu in the top bar (PRD 003 §7, decided 2026-08-28) — NOT
// from the bottom nav, so it is deliberately absent from config/navigation.ts.
//
// There is no sign-out button here on purpose: it lives in that same user menu, and
// PRD 005 requires exactly one sign-out action in the app.
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Bell, Camera, MapPin } from '@lucide/vue'
import { useProfileStore } from '@/stores/profile.store'
import { useNotificationsStore } from '@/stores/notifications.store'
import { useLocationStore } from '@/stores/location.store'
import { ROLE_LABELS } from '@/config/roles'
import { formatPhone } from '@/helpers'
import { blockedGuidance } from '@/config/permissions'
import PreferenceRow from '@/components/profile/PreferenceRow.vue'
import ProfilePhoto from '@/components/profile/ProfilePhoto.vue'

const profile = useProfileStore()
const notifications = useNotificationsStore()
const location = useLocationStore()

// Crew are seeded without an address and the app has no reason to show one, so an
// empty address is a normal state rather than missing data — but the row still says
// "Ikke registreret" rather than rendering a blank line.
const hasAddress = computed(() => {
  const d = profile.details
  return Boolean(d && (d.address || d.postalCode || d.city))
})

// —— På denne enhed ——
//
// The install row (PRD 005's `install.store`) is deliberately absent: that store does
// not exist yet, and stubbing it here would leave a row that always says the same
// thing. The list is a list, so appending it later costs one entry.

const busy = ref(false)

const pushRow = computed(() => {
  if (!notifications.available) {
    return {
      status: 'Ikke understøttet',
      detail:
        'Denne browser kan ikke sende notifikationer. På iPhone virker det kun, når appen er lagt på hjemmeskærmen.',
    }
  }
  if (notifications.permission === 'denied') {
    return { status: 'Blokeret', detail: blockedGuidance('notifications') }
  }
  // Granted but not subscribed is its own state, not a rounding error: the browser can
  // drop a subscription, and it is precisely when push silently stops working (task 100).
  if (notifications.permission === 'granted' && !notifications.subscribed) {
    return {
      status: 'Ikke tilmeldt',
      detail: 'Du har givet lov, men denne enhed er ikke tilmeldt notifikationer.',
      action: 'Tilmeld',
    }
  }
  if (notifications.subscribed) {
    return { status: 'Til', detail: 'Du får vigtige beskeder under løbet.' }
  }
  return {
    status: 'Fra',
    detail: 'Slå til, så du får de vigtige opdateringer under løbet.',
    action: 'Slå til',
  }
})

const locationRow = computed(() => {
  if (!location.available) {
    return { status: 'Ikke understøttet', detail: 'Denne enhed kan ikke oplyse din placering.' }
  }
  if (location.permission === 'denied') {
    return { status: 'Blokeret', detail: blockedGuidance('location') }
  }
  if (location.permission === 'granted') {
    // Deliberate wording (PRD 003 §11): nothing is shared live. Saying "til" alone
    // would imply someone is watching a screen with a dot on it.
    return {
      status: 'Til',
      detail: 'Kortet kan vise, hvor du er, og din rute gemmes og sendes til arrangørerne.',
    }
  }
  return {
    status: 'Fra',
    detail: 'Kortet kan ikke vise, hvor du er. Du bliver spurgt, når du åbner kortet.',
  }
})

// Camera has no store of its own yet — PRD 003's capture component (task 106) is
// blocked on the consent decision (task 102). Until then this row reports what can be
// known without opening the camera, which is the honest amount.
const cameraAvailable =
  typeof navigator !== 'undefined' && Boolean(navigator.mediaDevices?.getUserMedia)
const cameraPermission = ref<'unknown' | 'prompt' | 'granted' | 'denied'>('unknown')

async function syncCamera() {
  if (!cameraAvailable || typeof navigator === 'undefined' || !('permissions' in navigator)) return
  try {
    const status = await navigator.permissions.query({ name: 'camera' } as PermissionDescriptor)
    cameraPermission.value = status.state as 'prompt' | 'granted' | 'denied'
  } catch {
    // WebKit does not support querying the camera permission. Left as 'unknown',
    // which the row reports as "Du bliver spurgt …" rather than guessing "Fra" —
    // guessing here would tell an iPhone user their camera is off when it is not.
  }
}

const cameraRow = computed(() => {
  if (!cameraAvailable) {
    return { status: 'Ikke understøttet', detail: 'Denne browser giver ikke adgang til kameraet.' }
  }
  if (cameraPermission.value === 'denied') {
    return { status: 'Blokeret', detail: blockedGuidance('camera') }
  }
  if (cameraPermission.value === 'granted') {
    return { status: 'Til', detail: 'Du kan tage et billede af dig selv til din profil.' }
  }
  return { status: 'Klar', detail: 'Du bliver spurgt, første gang du tager et billede.' }
})

async function enablePush() {
  busy.value = true
  try {
    await notifications.enable()
  } finally {
    busy.value = false
  }
}

// A permission changed in browser or system settings does not notify the page, so the
// only reliable moment to re-read all of them is when the user comes back to it.
async function syncAll() {
  notifications.syncPermission()
  await Promise.all([notifications.syncSubscription(), location.syncPermission(), syncCamera()])
}

function onVisibility() {
  if (!document.hidden) void syncAll()
}

onMounted(() => {
  void profile.ensureLoaded()
  void syncAll()
  document.addEventListener('visibilitychange', onVisibility)
})
onUnmounted(() => {
  document.removeEventListener('visibilitychange', onVisibility)
})
</script>

<template>
  <div class="space-y-6 px-4 pt-4 pb-4">
    <header>
      <h1 class="font-nathejk text-2xl text-slate-900">Min profil</h1>
      <p v-if="profile.details" class="mt-1 text-sm text-slate-500">
        {{ profile.details.name }} · {{ ROLE_LABELS[profile.details.role] }}
        <template v-if="profile.details.team"> · {{ profile.details.team }}</template>
        <template v-else-if="profile.details.section"> · {{ profile.details.section }}</template>
      </p>
    </header>

    <!-- A failed fetch must still leave a usable page: the sections below are where the
         device-permission rows land (task 099), and those read local state that works
         perfectly well while the network does not. -->
    <p v-if="profile.error" class="rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-900">
      {{ profile.error }}
    </p>

    <ProfilePhoto />

    <section>
      <h2 class="font-nathejk text-lg text-slate-900">Mine oplysninger</h2>
      <p v-if="profile.loading" class="mt-2 text-sm text-slate-500">Henter …</p>

      <!-- A definition list, not a form: these fields are read-only (PRD 003 §6), and
           rendering them as disabled inputs would invite people to try to edit them. -->
      <dl
        v-if="profile.details"
        class="mt-2 divide-y divide-slate-100 rounded-xl border border-slate-200 bg-white px-4 shadow-xs"
      >
        <div class="flex items-baseline justify-between gap-4 py-3">
          <dt class="text-sm text-slate-500">Navn</dt>
          <dd class="text-right text-sm text-slate-900">{{ profile.details.name }}</dd>
        </div>

        <div class="flex items-baseline justify-between gap-4 py-3">
          <dt class="text-sm text-slate-500">Adresse</dt>
          <dd class="text-right text-sm text-slate-900">
            <template v-if="hasAddress">
              <span class="block">{{ profile.details.address }}</span>
              <span class="block">{{ profile.details.postalCode }} {{ profile.details.city }}</span>
            </template>
            <span v-else class="text-slate-400">Ikke registreret</span>
          </dd>
        </div>

        <div class="flex items-baseline justify-between gap-4 py-3">
          <dt class="text-sm text-slate-500">Telefon</dt>
          <dd class="text-right text-sm">
            <!-- href keeps the normalized number; only the label is grouped. -->
            <a
              v-if="profile.details.phone"
              :href="`tel:${profile.details.phone}`"
              class="text-slate-900 underline"
            >
              {{ formatPhone(profile.details.phone) }}
            </a>
            <span v-else class="text-slate-400">Ikke registreret</span>
          </dd>
        </div>

        <!-- Hidden entirely when phoneParent is null: that means this population has
             no guardian number at all (bandit, crew, gøgler), and showing them an
             empty row would imply they are missing something. An empty *string* is
             the different case — expected, not registered — and does show. -->
        <div
          v-if="profile.details.phoneParent !== null"
          class="flex items-baseline justify-between gap-4 py-3"
        >
          <dt class="text-sm text-slate-500">Forælders telefon</dt>
          <dd class="text-right text-sm">
            <a
              v-if="profile.details.phoneParent"
              :href="`tel:${profile.details.phoneParent}`"
              class="text-slate-900 underline"
            >
              {{ formatPhone(profile.details.phoneParent) }}
            </a>
            <span v-else class="text-slate-400">Ikke registreret</span>
          </dd>
        </div>
      </dl>

      <!-- Deliberately names no contact channel: PRD 003 §11 "Editability" is still
           open, and inventing a phone number or an email here would be worse than
           saying who to ask. -->
      <p v-if="profile.details" class="mt-2 text-xs text-slate-500">
        Oplysningerne kommer fra jeres tilmelding og kan ikke rettes i appen. Er noget
        forkert, sig det til din leder eller til Nathejk i startområdet.
      </p>
    </section>

    <section>
      <h2 class="font-nathejk text-lg text-slate-900">På denne enhed</h2>
      <div
        class="mt-2 divide-y divide-slate-100 rounded-xl border border-slate-200 bg-white px-4 shadow-xs"
      >
        <PreferenceRow
          :icon="Bell"
          label="Notifikationer"
          :status="pushRow.status"
          :detail="pushRow.detail"
          :action="pushRow.action"
          :busy="busy"
          @act="enablePush"
        />
        <PreferenceRow
          :icon="MapPin"
          label="Placering"
          :status="locationRow.status"
          :detail="locationRow.detail"
        />
        <PreferenceRow
          :icon="Camera"
          label="Kamera"
          :status="cameraRow.status"
          :detail="cameraRow.detail"
        />
      </div>
    </section>
  </div>
</template>
